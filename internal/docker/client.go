package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	mobyclient "github.com/moby/moby/client"
)

// ErrNotFound is returned when the engine has no such container.
var ErrNotFound = errors.New("container not found")

// Container is the inspect data returned by the Docker engine.
type Container struct {
	ID        string
	Name      string
	ImageRef  string
	ImageID   string
	Running   bool
	Restart   string
	Ports     map[string]string
	Volumes   map[string]string
	CPUs      float64
	MemoryMiB int64
}

// Client talks to the OrbStack Docker engine.
type Client struct {
	host string
	api  *mobyclient.Client
}

// New dials lazily: constructing the client does not open the socket.
// The first RPC does. Connection errors include host.
func New(host string) (*Client, error) {
	api, err := mobyclient.New(
		mobyclient.WithHost(host),
		mobyclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}
	return &Client{host: host, api: api}, nil
}

// Pull always pulls ref.
func (c *Client) Pull(ctx context.Context, ref string) error {
	rc, err := c.api.ImagePull(ctx, ref, mobyclient.ImagePullOptions{})
	if err != nil {
		return c.wrap(err)
	}
	defer rc.Close()
	if err := rc.Wait(ctx); err != nil {
		return c.wrap(err)
	}
	return nil
}

// Create creates a container and returns its id.
func (c *Client) Create(ctx context.Context, name string, spec Spec) (string, error) {
	res, err := c.api.ContainerCreate(ctx, mobyclient.ContainerCreateOptions{
		Name:       name,
		Config:     spec.Config,
		HostConfig: spec.HostConfig,
	})
	if err != nil {
		return "", c.wrap(err)
	}
	return res.ID, nil
}

// Start starts a container.
func (c *Client) Start(ctx context.Context, id string) error {
	_, err := c.api.ContainerStart(ctx, id, mobyclient.ContainerStartOptions{})
	return c.wrap(err)
}

// Stop stops a container with a 10 second timeout.
func (c *Client) Stop(ctx context.Context, id string) error {
	timeout := 10
	_, err := c.api.ContainerStop(ctx, id, mobyclient.ContainerStopOptions{Timeout: &timeout})
	return c.wrap(err)
}

// Remove force-removes a container and leaves volumes in place.
// A missing container is success.
func (c *Client) Remove(ctx context.Context, id string) error {
	_, err := c.api.ContainerRemove(ctx, id, mobyclient.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: false,
	})
	if cerrdefs.IsNotFound(err) {
		return nil
	}
	return c.wrap(err)
}

// UpdateRestart changes the restart policy in place.
func (c *Client) UpdateRestart(ctx context.Context, id, policy string) error {
	mode := restartMode(policy)
	_, err := c.api.ContainerUpdate(ctx, id, mobyclient.ContainerUpdateOptions{
		RestartPolicy: &container.RestartPolicy{Name: mode},
	})
	return c.wrap(err)
}

// Inspect loads a container by id or name.
func (c *Client) Inspect(ctx context.Context, idOrName string) (Container, error) {
	res, err := c.api.ContainerInspect(ctx, idOrName, mobyclient.ContainerInspectOptions{})
	if cerrdefs.IsNotFound(err) {
		return Container{}, ErrNotFound
	}
	if err != nil {
		return Container{}, c.wrap(err)
	}
	return viewFromInspect(res.Container), nil
}

func viewFromInspect(in container.InspectResponse) Container {
	view := Container{
		ID:      in.ID,
		Name:    strings.TrimPrefix(in.Name, "/"),
		ImageID: in.Image,
	}
	if in.Config != nil {
		view.ImageRef = in.Config.Image
	}
	if in.State != nil {
		view.Running = in.State.Running
	}
	if in.HostConfig != nil {
		view.Restart = string(in.HostConfig.RestartPolicy.Name)
		if view.Restart == "" {
			view.Restart = "no"
		}
		if in.HostConfig.NanoCPUs > 0 {
			view.CPUs = float64(in.HostConfig.NanoCPUs) / 1e9
		}
		if in.HostConfig.Memory > 0 {
			view.MemoryMiB = in.HostConfig.Memory / (1024 * 1024)
		}
		if len(in.HostConfig.PortBindings) > 0 {
			view.Ports = map[string]string{}
			for port, bindings := range in.HostConfig.PortBindings {
				if len(bindings) == 0 || bindings[0].HostPort == "" {
					continue
				}
				view.Ports[bindings[0].HostPort] = strconvPort(port.Num())
			}
		}
		if len(in.HostConfig.Binds) > 0 {
			view.Volumes = map[string]string{}
			for _, bind := range in.HostConfig.Binds {
				hostPath, containerPath, ok := strings.Cut(bind, ":")
				if !ok || hostPath == "" || containerPath == "" {
					continue
				}
				view.Volumes[hostPath] = containerPath
			}
		}
	}
	return view
}

func strconvPort(n uint16) string {
	return fmt.Sprintf("%d", n)
}

func (c *Client) wrap(err error) error {
	if err == nil || cerrdefs.IsNotFound(err) {
		return err
	}
	msg := err.Error()
	if strings.Contains(msg, "Cannot connect to the Docker daemon") ||
		strings.Contains(msg, "connect:") ||
		strings.Contains(msg, "no such file or directory") ||
		strings.Contains(msg, "connection refused") {
		return fmt.Errorf("dial %s: %w", c.host, err)
	}
	return err
}
