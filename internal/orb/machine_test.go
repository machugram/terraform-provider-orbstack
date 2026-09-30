package orb

import (
	"testing"
)

func TestBuildCreateArgs(t *testing.T) {
	cpus := int64(2)
	memory := int64(2048)
	disk := int64(16)
	got := BuildCreateArgs(CreateMachine{
		Name:           "dev",
		Image:          "ubuntu:noble",
		Arch:           "arm64",
		Username:       "dev",
		CPUs:           &cpus,
		MemoryMiB:      &memory,
		DiskGiB:        &disk,
		Isolated:       true,
		IsolateNetwork: true,
		Mounts:         []string{"/tmp/src:/src"},
		CloudInitFile:  "/tmp/user-data.yaml",
	})
	want := []string{
		"create",
		"--cpus", "2",
		"--memory", "2048M",
		"--disk", "16G",
		"--arch", "arm64",
		"--user", "dev",
		"--isolated",
		"--isolate-network",
		"--mount", "/tmp/src:/src",
		"-c", "/tmp/user-data.yaml",
		"ubuntu:noble", "dev",
	}
	if len(got) != len(want) {
		t.Fatalf("len %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q", i, got[i], want[i])
		}
	}

	minimal := BuildCreateArgs(CreateMachine{Name: "box"})
	if got, want := join(minimal), "create ubuntu box"; got != want {
		t.Fatalf("minimal = %q, want %q", got, want)
	}
}

func TestParseInfo(t *testing.T) {
	info, err := ParseInfo([]byte(`{
	  "record": {
	    "id": "01KHVDCW7HTAGRPMB823KADP9B",
	    "name": "ubuntu",
	    "state": "stopped",
	    "image": {"distro": "ubuntu", "version": "questing", "arch": "arm64"},
	    "config": {"isolated": false, "isolate_network": false, "default_username": "rexfordmachu"}
	  },
	  "disk_size": 847974400
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Record.ID != "01KHVDCW7HTAGRPMB823KADP9B" || info.Record.Name != "ubuntu" {
		t.Fatalf("record = %#v", info.Record)
	}
	if info.Record.Image.Distro != "ubuntu" || info.Record.Image.Version != "questing" || info.Record.Image.Arch != "arm64" {
		t.Fatalf("image = %#v", info.Record.Image)
	}
	if info.DiskSize != 847974400 {
		t.Fatalf("disk_size = %d", info.DiskSize)
	}
	if ImageRef(info.Record.Image) != "ubuntu:questing" {
		t.Fatalf("image ref = %s", ImageRef(info.Record.Image))
	}
}

func TestLimitsFromConfigZeroIsNull(t *testing.T) {
	cpus, memory, disk, err := LimitsFromConfig("0", "0", "0")
	if err != nil {
		t.Fatal(err)
	}
	if cpus != nil || memory != nil || disk != nil {
		t.Fatalf("zero limits = %v %v %v, want nil", cpus, memory, disk)
	}

	cpus, memory, disk, err = LimitsFromConfig("2", "2048", "17179869184")
	if err != nil {
		t.Fatal(err)
	}
	if cpus == nil || *cpus != 2 || memory == nil || *memory != 2048 || disk == nil || *disk != 16 {
		t.Fatalf("limits = %v %v %v", cpus, memory, disk)
	}
	if DiskBytes(16) != 17179869184 {
		t.Fatalf("disk bytes = %d", DiskBytes(16))
	}
}

func TestValidateMachineMounts(t *testing.T) {
	if err := ValidateMachine(MachineRequest{Mounts: []string{"/tmp:/tmp"}}); err == nil {
		t.Fatal("expected mounts to require isolated")
	}
	if err := ValidateMachine(MachineRequest{Isolated: true, Mounts: []string{"/tmp:/tmp"}}); err != nil {
		t.Fatal(err)
	}
}

func join(args []string) string {
	out := ""
	for i, arg := range args {
		if i > 0 {
			out += " "
		}
		out += arg
	}
	return out
}
