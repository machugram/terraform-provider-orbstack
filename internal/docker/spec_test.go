package docker

import (
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
)

func TestBuildSpec(t *testing.T) {
	cpus := 2.0
	memory := int64(64)
	spec, err := BuildSpec(SpecInput{
		Image: "nginx:latest",
		Ports: map[string]string{"8080": "80"},
		Env:   map[string]string{"NGINX_HOST": "localhost"},
		Volumes: map[string]string{
			"/tmp/site": "/usr/share/nginx/html",
		},
		CPUs:      &cpus,
		MemoryMiB: &memory,
		Restart:   "unless-stopped",
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Config.Image != "nginx:latest" {
		t.Fatalf("image = %s", spec.Config.Image)
	}
	if len(spec.Config.Env) != 1 || spec.Config.Env[0] != "NGINX_HOST=localhost" {
		t.Fatalf("env = %#v", spec.Config.Env)
	}
	if len(spec.HostConfig.Binds) != 1 || spec.HostConfig.Binds[0] != "/tmp/site:/usr/share/nginx/html" {
		t.Fatalf("binds = %#v", spec.HostConfig.Binds)
	}
	if spec.HostConfig.NanoCPUs != 2_000_000_000 {
		t.Fatalf("nanocpus = %d", spec.HostConfig.NanoCPUs)
	}
	if spec.HostConfig.Memory != 64*1024*1024 {
		t.Fatalf("memory = %d", spec.HostConfig.Memory)
	}
	if spec.HostConfig.RestartPolicy.Name != container.RestartPolicyUnlessStopped {
		t.Fatalf("restart = %s", spec.HostConfig.RestartPolicy.Name)
	}

	var found bool
	for port, bindings := range spec.HostConfig.PortBindings {
		if port.Num() == 80 && len(bindings) == 1 && bindings[0].HostPort == "8080" && bindings[0].HostIP.String() == "0.0.0.0" {
			found = true
		}
	}
	if !found {
		t.Fatalf("port bindings = %#v", spec.HostConfig.PortBindings)
	}
}

func TestBuildSpecRejectsPort(t *testing.T) {
	for _, port := range []string{"0", "65536", "abc"} {
		_, err := BuildSpec(SpecInput{
			Image: "nginx:latest",
			Ports: map[string]string{port: "80"},
			Env:   map[string]string{"SECRET": "hunter2"},
		})
		if err == nil {
			t.Fatalf("port %s: expected error", port)
		}
		if !strings.Contains(err.Error(), "1-65535") {
			t.Fatalf("port %s: %v", port, err)
		}
		if strings.Contains(err.Error(), "hunter2") {
			t.Fatalf("port error leaked env: %v", err)
		}
	}
}

func TestBuildSpecRejectsRelativeVolume(t *testing.T) {
	_, err := BuildSpec(SpecInput{
		Image:   "nginx:latest",
		Volumes: map[string]string{"site": "/usr/share/nginx/html"},
	})
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("err = %v", err)
	}
}
