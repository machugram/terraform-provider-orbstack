package docker

import (
	"fmt"
	"maps"
	"math"
	"net/netip"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
)

// SpecInput is the container configuration supplied by Terraform.
type SpecInput struct {
	Image       string
	BindAddress string
	Ports       map[string]string
	Env         map[string]string
	Volumes     map[string]string
	Command     []string
	Workdir     string
	CPUs        *float64
	MemoryMiB   *int64
	Restart     string
}

// Spec is the Engine API create body.
type Spec struct {
	Config     *container.Config
	HostConfig *container.HostConfig
}

// BuildSpec maps a container config onto a Docker Config and HostConfig.
// Ports and volume host paths are validated here.
func BuildSpec(in SpecInput) (Spec, error) {
	cfg := &container.Config{Image: in.Image}
	host := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: restartMode(in.Restart)},
	}

	if len(in.Command) > 0 {
		cfg.Cmd = append([]string(nil), in.Command...)
	}
	if in.Workdir != "" {
		cfg.WorkingDir = in.Workdir
	}
	if len(in.Env) > 0 {
		keys := slices.Sorted(maps.Keys(in.Env))
		cfg.Env = make([]string, 0, len(keys))
		for _, key := range keys {
			cfg.Env = append(cfg.Env, key+"="+in.Env[key])
		}
	}
	if in.CPUs != nil {
		if *in.CPUs <= 0 {
			return Spec{}, fmt.Errorf("cpus must be greater than 0")
		}
		host.NanoCPUs = int64(math.Round(*in.CPUs * 1e9))
	}
	if in.MemoryMiB != nil {
		host.Memory = *in.MemoryMiB * 1024 * 1024
	}

	bind, err := bindAddr(in.BindAddress)
	if err != nil {
		return Spec{}, err
	}
	if len(in.Ports) > 0 {
		host.PortBindings = network.PortMap{}
		cfg.ExposedPorts = network.PortSet{}
		for hostPort, containerPort := range in.Ports {
			if _, err := parsePort(hostPort); err != nil {
				return Spec{}, err
			}
			cport, err := parsePort(containerPort)
			if err != nil {
				return Spec{}, err
			}
			port, ok := network.PortFrom(uint16(cport), network.TCP)
			if !ok {
				return Spec{}, fmt.Errorf("port %q is outside 1-65535", containerPort)
			}
			cfg.ExposedPorts[port] = struct{}{}
			host.PortBindings[port] = []network.PortBinding{{
				HostIP:   bind,
				HostPort: hostPort,
			}}
		}
	}

	if len(in.Volumes) > 0 {
		hosts := slices.Sorted(maps.Keys(in.Volumes))
		host.Binds = make([]string, 0, len(hosts))
		for _, hostPath := range hosts {
			containerPath := in.Volumes[hostPath]
			if !path.IsAbs(hostPath) {
				return Spec{}, fmt.Errorf("volume host path %q must be absolute", hostPath)
			}
			if strings.Contains(hostPath, ":") {
				return Spec{}, fmt.Errorf("volume host path %q must not contain ':'", hostPath)
			}
			if containerPath == "" || !path.IsAbs(containerPath) {
				return Spec{}, fmt.Errorf("volume container path %q must be absolute", containerPath)
			}
			if strings.Contains(containerPath, ":") {
				return Spec{}, fmt.Errorf("volume container path %q must not contain ':'", containerPath)
			}
			host.Binds = append(host.Binds, hostPath+":"+containerPath)
		}
	}

	return Spec{Config: cfg, HostConfig: host}, nil
}

func bindAddr(raw string) (netip.Addr, error) {
	if raw == "" {
		raw = "127.0.0.1"
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("bind_address %q must be an IP address", raw)
	}
	return addr, nil
}

func parsePort(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("port %q is outside 1-65535", raw)
	}
	return n, nil
}

func restartMode(policy string) container.RestartPolicyMode {
	switch policy {
	case "always":
		return container.RestartPolicyAlways
	case "on-failure":
		return container.RestartPolicyOnFailure
	case "unless-stopped", "":
		return container.RestartPolicyUnlessStopped
	default:
		return container.RestartPolicyDisabled
	}
}
