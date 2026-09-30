package orb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Machine is an OrbStack Linux machine as reported by orbctl.
type Machine struct {
	ID             string
	Name           string
	Image          string
	Distro         string
	Version        string
	Arch           string
	Username       string
	Isolated       bool
	IsolateNetwork bool
	State          string
	PowerState     string
	Default        bool
	CPUs           *int64
	MemoryMiB      *int64
	DiskGiB        *int64
	DiskSizeBytes  int64
}

// MachineRequest is the desired machine shape.
type MachineRequest struct {
	Name           string
	Image          string
	Arch           string
	Username       string
	CloudInit      string
	CloudInitFile  string
	Isolated       bool
	IsolateNetwork bool
	Mounts         []string
	CPUs           *int64
	MemoryMiB      *int64
	DiskGiB        *int64
	PowerState     string
	DefaultMachine bool
}

// ValidateMachine checks request rules that orbctl would otherwise reject later.
func ValidateMachine(req MachineRequest) error {
	if len(req.Mounts) > 0 && !req.Isolated {
		return errors.New("mounts require isolated")
	}
	return nil
}

// CreateMachine creates a machine, applies limits, power, and the default flag.
func (c *Client) CreateMachine(ctx context.Context, req MachineRequest) (Machine, error) {
	if err := ValidateMachine(req); err != nil {
		return Machine{}, err
	}
	cloudFile, cleanup, err := materializeCloudInit(req)
	if err != nil {
		return Machine{}, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	create := CreateMachine{
		Name:           req.Name,
		Image:          req.Image,
		Arch:           req.Arch,
		Username:       req.Username,
		CPUs:           req.CPUs,
		MemoryMiB:      req.MemoryMiB,
		DiskGiB:        req.DiskGiB,
		Isolated:       req.Isolated,
		IsolateNetwork: req.IsolateNetwork,
		Mounts:         req.Mounts,
		CloudInitFile:  cloudFile,
	}
	if err := c.Create(ctx, create); err != nil {
		return Machine{}, err
	}
	if err := c.SetLimits(ctx, req.Name, req.CPUs, req.MemoryMiB, req.DiskGiB); err != nil {
		return Machine{}, err
	}
	if req.PowerState == "stopped" {
		if err := c.Stop(ctx, req.Name); err != nil {
			return Machine{}, err
		}
	}
	if req.DefaultMachine {
		if err := c.SetDefault(ctx, req.Name); err != nil {
			return Machine{}, err
		}
	}
	return c.GetMachine(ctx, req.Name, "")
}

// GetMachine reads a machine by name, then by id if the name is missing.
func (c *Client) GetMachine(ctx context.Context, name, id string) (Machine, error) {
	key := name
	if key == "" {
		key = id
	}
	info, err := c.Info(ctx, key)
	if errors.Is(err, ErrNotFound) && id != "" && id != key {
		info, err = c.Info(ctx, id)
	}
	if err != nil {
		return Machine{}, err
	}
	return c.machineFromInfo(ctx, info)
}

// UpdateMachine renames, updates limits, power, and the default flag.
func (c *Client) UpdateMachine(ctx context.Context, currentName string, desired MachineRequest) (Machine, error) {
	name := currentName
	if desired.Name != "" && desired.Name != name {
		if err := c.Rename(ctx, name, desired.Name); err != nil {
			return Machine{}, err
		}
		name = desired.Name
	}

	current, err := c.GetMachine(ctx, name, "")
	if err != nil {
		return Machine{}, err
	}
	if !sameLimit(current.CPUs, desired.CPUs) || !sameLimit(current.MemoryMiB, desired.MemoryMiB) || !sameLimit(current.DiskGiB, desired.DiskGiB) {
		if err := c.SetLimits(ctx, name, desired.CPUs, desired.MemoryMiB, desired.DiskGiB); err != nil {
			return Machine{}, err
		}
		if current.State == "running" {
			if err := c.Restart(ctx, name); err != nil {
				return Machine{}, err
			}
		}
	}

	info, err := c.Info(ctx, name)
	if err != nil {
		return Machine{}, err
	}
	power := desired.PowerState
	if power == "" {
		power = "running"
	}
	if power == "stopped" && info.Record.State != "stopped" {
		if err := c.Stop(ctx, name); err != nil {
			return Machine{}, err
		}
	}
	if power == "running" && info.Record.State == "stopped" {
		if err := c.Start(ctx, name); err != nil {
			return Machine{}, err
		}
	}

	defaultName, err := c.DefaultName(ctx)
	if err != nil {
		return Machine{}, err
	}
	if desired.DefaultMachine && defaultName != name {
		if err := c.SetDefault(ctx, name); err != nil {
			return Machine{}, err
		}
	}
	if !desired.DefaultMachine && defaultName == name {
		if err := c.SetDefault(ctx, "none"); err != nil {
			return Machine{}, err
		}
	}
	return c.GetMachine(ctx, name, "")
}

// RemoveMachine deletes a machine. A missing machine is success.
func (c *Client) RemoveMachine(ctx context.Context, nameOrID string) error {
	info, err := c.Info(ctx, nameOrID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return c.Delete(ctx, info.Record.Name)
}

// ListMachines returns machines. runningOnly limits the list to running ones.
func (c *Client) ListMachines(ctx context.Context, runningOnly bool) ([]Machine, error) {
	records, err := c.List(ctx, runningOnly)
	if err != nil {
		return nil, err
	}
	out := make([]Machine, 0, len(records))
	for _, rec := range records {
		out = append(out, machineFromRecord(rec, 0))
	}
	return out, nil
}

func (c *Client) machineFromInfo(ctx context.Context, info MachineInfo) (Machine, error) {
	machine := machineFromRecord(info.Record, info.DiskSize)
	cpus, mem, disk, err := c.Limits(ctx, info.Record.Name)
	if err != nil {
		return Machine{}, err
	}
	machine.CPUs = cpus
	machine.MemoryMiB = mem
	machine.DiskGiB = disk
	current, err := c.DefaultName(ctx)
	if err != nil {
		return Machine{}, err
	}
	machine.Default = current == info.Record.Name
	return machine, nil
}

func machineFromRecord(rec MachineRecord, diskSize int64) Machine {
	power := "running"
	if rec.State == "stopped" {
		power = "stopped"
	}
	return Machine{
		ID:             rec.ID,
		Name:           rec.Name,
		Image:          ImageRef(rec.Image),
		Distro:         rec.Image.Distro,
		Version:        rec.Image.Version,
		Arch:           rec.Image.Arch,
		Username:       rec.Config.DefaultUsername,
		Isolated:       rec.Config.Isolated,
		IsolateNetwork: rec.Config.IsolateNetwork,
		State:          rec.State,
		PowerState:     power,
		DiskSizeBytes:  diskSize,
	}
}

func sameLimit(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func materializeCloudInit(req MachineRequest) (string, func(), error) {
	if req.CloudInitFile != "" {
		if _, err := os.Stat(req.CloudInitFile); err != nil {
			return "", nil, fmt.Errorf("cloud_init_file not found: %w", err)
		}
		abs, err := filepath.Abs(req.CloudInitFile)
		if err != nil {
			return "", nil, fmt.Errorf("cloud_init_file path: %w", err)
		}
		return abs, nil, nil
	}
	if req.CloudInit == "" {
		return "", nil, nil
	}
	f, err := os.CreateTemp("", "orbstack-cloud-init-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("cloud-init: %w", err)
	}
	if _, err := f.WriteString(req.CloudInit); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("cloud-init: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("cloud-init: %w", err)
	}
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}
