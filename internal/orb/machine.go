package orb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const gib = int64(1024 * 1024 * 1024)

// MachineRecord is one machine from orbctl JSON.
type MachineRecord struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Image  MachineImage  `json:"image"`
	Config MachineConfig `json:"config"`
	State  string        `json:"state"`
}

// MachineImage is the distro recorded for a machine.
type MachineImage struct {
	Distro  string `json:"distro"`
	Version string `json:"version"`
	Arch    string `json:"arch"`
}

// MachineConfig is the config object on a machine record.
type MachineConfig struct {
	Isolated        bool   `json:"isolated"`
	IsolateNetwork  bool   `json:"isolate_network"`
	DefaultUsername string `json:"default_username"`
}

// MachineInfo is the orbctl info -f json document.
type MachineInfo struct {
	Record   MachineRecord `json:"record"`
	DiskSize int64         `json:"disk_size"`
}

// CreateMachine is the input for orbctl create.
type CreateMachine struct {
	Name           string
	Image          string
	Arch           string
	Username       string
	CPUs           *int64
	MemoryMiB      *int64
	DiskGiB        *int64
	Isolated       bool
	IsolateNetwork bool
	Mounts         []string
	CloudInitFile  string
}

// BuildCreateArgs returns the orbctl arguments for create, omitting unset flags.
func BuildCreateArgs(m CreateMachine) []string {
	args := []string{"create"}
	if m.CPUs != nil {
		args = append(args, "--cpus", strconv.FormatInt(*m.CPUs, 10))
	}
	if m.MemoryMiB != nil {
		args = append(args, "--memory", fmt.Sprintf("%dM", *m.MemoryMiB))
	}
	if m.DiskGiB != nil {
		args = append(args, "--disk", fmt.Sprintf("%dG", *m.DiskGiB))
	}
	if m.Arch != "" {
		args = append(args, "--arch", m.Arch)
	}
	if m.Username != "" {
		args = append(args, "--user", m.Username)
	}
	if m.Isolated {
		args = append(args, "--isolated")
	}
	if m.IsolateNetwork {
		args = append(args, "--isolate-network")
	}
	for _, mount := range m.Mounts {
		args = append(args, "--mount", mount)
	}
	if m.CloudInitFile != "" {
		args = append(args, "-c", m.CloudInitFile)
	}
	image := m.Image
	if image == "" {
		image = "ubuntu"
	}
	args = append(args, image, m.Name)
	return args
}

// OptionalLimit returns nil when v is 0. OrbStack uses 0 for an unlimited limit.
func OptionalLimit(v int64) *int64 {
	if v == 0 {
		return nil
	}
	n := v
	return &n
}

// DiskGiBFromBytes converts a disk_bytes config value to GiB.
// Zero means unlimited and returns ok == false.
func DiskGiBFromBytes(bytes int64) (int64, bool) {
	if bytes < gib {
		return 0, false
	}
	return bytes / gib, true
}

// DiskBytes converts GiB to the disk_bytes config value.
func DiskBytes(gibs int64) int64 {
	return gibs * gib
}

// LimitsFromConfig parses orbctl config get values.
// A 0 cpu, memory, or disk value is stored as nil.
func LimitsFromConfig(cpuRaw, memRaw, diskRaw string) (cpus, memoryMiB, diskGiB *int64, err error) {
	cpu, err := ParseConfigInt(cpuRaw)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cpu: %w", err)
	}
	mem, err := ParseConfigInt(memRaw)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("memory_mib: %w", err)
	}
	diskBytes, err := ParseConfigInt(diskRaw)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("disk_bytes: %w", err)
	}
	cpus = OptionalLimit(cpu)
	memoryMiB = OptionalLimit(mem)
	if gibs, ok := DiskGiBFromBytes(diskBytes); ok {
		diskGiB = &gibs
	}
	return cpus, memoryMiB, diskGiB, nil
}

// ParseConfigInt parses orbctl config get output. Empty output is 0.
func ParseConfigInt(out string) (int64, error) {
	s := strings.TrimSpace(out)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

// ParseInfo decodes orbctl info -f json.
func ParseInfo(data []byte) (MachineInfo, error) {
	var info MachineInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return MachineInfo{}, err
	}
	if info.Record.ID == "" && info.Record.Name == "" {
		var rec MachineRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			return MachineInfo{}, err
		}
		info.Record = rec
	}
	return info, nil
}

// ParseList decodes orbctl list -f json.
func ParseList(data []byte) ([]MachineRecord, error) {
	var records []MachineRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// ImageRef joins distro and version the way create accepts them.
func ImageRef(img MachineImage) string {
	if img.Version == "" {
		return img.Distro
	}
	if img.Distro == "" {
		return img.Version
	}
	return img.Distro + ":" + img.Version
}

// Info runs orbctl info -f json.
func (c *Client) Info(ctx context.Context, nameOrID string) (MachineInfo, error) {
	stdout, stderr, err := c.Run(ctx, "info", "-f", "json", nameOrID)
	if err != nil {
		if IsNotFound(stdout, stderr) {
			return MachineInfo{}, ErrNotFound
		}
		return MachineInfo{}, commandErr(err, stdout, stderr)
	}
	info, parseErr := ParseInfo([]byte(stdout))
	if parseErr != nil {
		return MachineInfo{}, parseErr
	}
	return info, nil
}

// List runs orbctl list -f json. runningOnly adds -r.
func (c *Client) List(ctx context.Context, runningOnly bool) ([]MachineRecord, error) {
	args := []string{"list", "-f", "json"}
	if runningOnly {
		args = append(args, "-r")
	}
	stdout, stderr, err := c.Run(ctx, args...)
	if err != nil {
		return nil, commandErr(err, stdout, stderr)
	}
	return ParseList([]byte(stdout))
}

// Create runs orbctl create.
func (c *Client) Create(ctx context.Context, m CreateMachine) error {
	stdout, stderr, err := c.Run(ctx, BuildCreateArgs(m)...)
	return commandErr(err, stdout, stderr)
}

// Rename runs orbctl rename.
func (c *Client) Rename(ctx context.Context, oldName, newName string) error {
	stdout, stderr, err := c.Run(ctx, "rename", oldName, newName)
	return commandErr(err, stdout, stderr)
}

// Start runs orbctl start.
func (c *Client) Start(ctx context.Context, name string) error {
	stdout, stderr, err := c.Run(ctx, "start", name)
	return commandErr(err, stdout, stderr)
}

// Stop runs orbctl stop.
func (c *Client) Stop(ctx context.Context, name string) error {
	stdout, stderr, err := c.Run(ctx, "stop", name)
	return commandErr(err, stdout, stderr)
}

// Restart runs orbctl restart.
func (c *Client) Restart(ctx context.Context, name string) error {
	stdout, stderr, err := c.Run(ctx, "restart", name)
	return commandErr(err, stdout, stderr)
}

// DefaultName returns the current default machine name.
func (c *Client) DefaultName(ctx context.Context) (string, error) {
	stdout, stderr, err := c.Run(ctx, "default")
	if err != nil {
		return "", commandErr(err, stdout, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

// SetDefault runs orbctl default. Pass "none" to clear it.
func (c *Client) SetDefault(ctx context.Context, name string) error {
	stdout, stderr, err := c.Run(ctx, "default", name)
	return commandErr(err, stdout, stderr)
}

// Delete runs orbctl delete. A missing machine is success.
func (c *Client) Delete(ctx context.Context, name string) error {
	stdout, stderr, err := c.Run(ctx, "delete", name)
	if err != nil && IsNotFound(stdout, stderr) {
		return nil
	}
	return commandErr(err, stdout, stderr)
}

// ConfigGet runs orbctl config get.
func (c *Client) ConfigGet(ctx context.Context, key string) (string, error) {
	stdout, stderr, err := c.Run(ctx, "config", "get", key)
	if err != nil {
		return "", commandErr(err, stdout, stderr)
	}
	return stdout, nil
}

// ConfigSet runs orbctl config set.
func (c *Client) ConfigSet(ctx context.Context, key, value string) error {
	stdout, stderr, err := c.Run(ctx, "config", "set", key, value)
	return commandErr(err, stdout, stderr)
}

// MachineKey is machine.<name>.<leaf>.
func MachineKey(name, leaf string) string {
	return "machine." + name + "." + leaf
}

// Limits reads cpu, memory, and disk config for a machine.
func (c *Client) Limits(ctx context.Context, name string) (cpus, memoryMiB, diskGiB *int64, err error) {
	cpuRaw, err := c.ConfigGet(ctx, MachineKey(name, "cpu"))
	if err != nil {
		return nil, nil, nil, err
	}
	memRaw, err := c.ConfigGet(ctx, MachineKey(name, "memory_mib"))
	if err != nil {
		return nil, nil, nil, err
	}
	diskRaw, err := c.ConfigGet(ctx, MachineKey(name, "disk_bytes"))
	if err != nil {
		return nil, nil, nil, err
	}
	return LimitsFromConfig(cpuRaw, memRaw, diskRaw)
}

// SetLimits writes machine limit config keys. Nil means unlimited (0).
func (c *Client) SetLimits(ctx context.Context, name string, cpus, memoryMiB, diskGiB *int64) error {
	cpu := "0"
	if cpus != nil {
		cpu = strconv.FormatInt(*cpus, 10)
	}
	if err := c.ConfigSet(ctx, MachineKey(name, "cpu"), cpu); err != nil {
		return err
	}
	mem := "0"
	if memoryMiB != nil {
		mem = strconv.FormatInt(*memoryMiB, 10)
	}
	if err := c.ConfigSet(ctx, MachineKey(name, "memory_mib"), mem); err != nil {
		return err
	}
	disk := "0"
	if diskGiB != nil {
		disk = strconv.FormatInt(DiskBytes(*diskGiB), 10)
	}
	return c.ConfigSet(ctx, MachineKey(name, "disk_bytes"), disk)
}
