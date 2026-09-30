package docker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ContainerRequest is the desired container shape.
type ContainerRequest struct {
	Name        string
	Image       string
	Ports       map[string]string
	Env         map[string]string
	Volumes     map[string]string
	Command     []string
	Workdir     string
	CPUs        *float64
	MemoryMiB   *int64
	Restart     string
	Running     bool
	BindAddress string
}

// CreateContainer pulls the image, creates the container, and starts it when requested.
func (c *Client) CreateContainer(ctx context.Context, req ContainerRequest) (Container, error) {
	spec, err := BuildSpec(specInput(req))
	if err != nil {
		return Container{}, err
	}
	if err := c.Pull(ctx, req.Image); err != nil {
		return Container{}, redactEnv(err, req.Env)
	}
	id, err := c.Create(ctx, req.Name, spec)
	if err != nil {
		return Container{}, redactEnv(err, req.Env)
	}
	if req.Running {
		if err := c.Start(ctx, id); err != nil {
			return Container{}, redactEnv(err, req.Env)
		}
	}
	created, err := c.Inspect(ctx, id)
	if err != nil {
		return Container{}, redactEnv(err, req.Env)
	}
	return created, nil
}

// FindContainer inspects by id, then by name.
func (c *Client) FindContainer(ctx context.Context, id, name string) (Container, error) {
	if id != "" {
		found, err := c.Inspect(ctx, id)
		if err == nil {
			return found, nil
		}
		if !errors.Is(err, ErrNotFound) || name == "" || name == id {
			return Container{}, err
		}
	}
	if name == "" {
		return Container{}, ErrNotFound
	}
	return c.Inspect(ctx, name)
}

// UpdateContainer changes restart policy and running state in place.
func (c *Client) UpdateContainer(ctx context.Context, id, restart string, running bool) (Container, error) {
	current, err := c.Inspect(ctx, id)
	if err != nil {
		return Container{}, err
	}
	if restart != "" && current.Restart != restart {
		if err := c.UpdateRestart(ctx, id, restart); err != nil {
			return Container{}, err
		}
	}
	if current.Running != running {
		if running {
			if err := c.Start(ctx, id); err != nil {
				return Container{}, err
			}
		} else if err := c.Stop(ctx, id); err != nil {
			return Container{}, err
		}
	}
	return c.Inspect(ctx, id)
}

// RemoveContainer force-removes a container without deleting volumes.
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	return c.Remove(ctx, id)
}

// SpecFromRequest validates a container request and returns the Engine API spec.
func SpecFromRequest(req ContainerRequest) (Spec, error) {
	return BuildSpec(specInput(req))
}

func specInput(req ContainerRequest) SpecInput {
	restart := req.Restart
	if restart == "" {
		restart = "unless-stopped"
	}
	return SpecInput{
		Image:       req.Image,
		BindAddress: req.BindAddress,
		Ports:       req.Ports,
		Env:         req.Env,
		Volumes:     req.Volumes,
		Command:     req.Command,
		Workdir:     req.Workdir,
		CPUs:        req.CPUs,
		MemoryMiB:   req.MemoryMiB,
		Restart:     restart,
	}
}

// ResolveHost picks a Docker host without dialing.
// Order: configured value, the orbstack docker context, then ~/.orbstack/run/docker.sock.
func ResolveHost(ctx context.Context, configured string) string {
	if strings.TrimSpace(configured) != "" {
		return strings.TrimSpace(configured)
	}
	cmd := exec.CommandContext(ctx, "docker", "context", "inspect", "orbstack", "--format", "{{.Endpoints.docker.Host}}")
	out, err := cmd.Output()
	if err == nil {
		host := strings.TrimSpace(string(out))
		if host != "" && host != "<no value>" {
			return host
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "unix://" + filepath.Join(".orbstack", "run", "docker.sock")
	}
	return "unix://" + filepath.Join(home, ".orbstack", "run", "docker.sock")
}

func redactEnv(err error, env map[string]string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	redacted := msg
	for _, value := range env {
		if value == "" {
			continue
		}
		redacted = strings.ReplaceAll(redacted, value, "[redacted]")
	}
	if redacted == msg {
		return err
	}
	return errors.New(redacted)
}
