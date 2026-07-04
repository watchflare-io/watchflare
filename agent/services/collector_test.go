package services

import (
	"runtime"
	"testing"
)

func names(svcs []*Service) []string {
	out := make([]string, len(svcs))
	for i, s := range svcs {
		out[i] = s.Name
	}
	return out
}

func TestMergeServices_Union(t *testing.T) {
	units := []rawUnit{
		{Name: "a.service", Description: "A", ActiveState: "active", SubState: "running"},
		{Name: "b.service", Description: "B", ActiveState: "inactive", SubState: "dead"},
		{Name: "c.service", Description: "C", ActiveState: "active", SubState: "exited"},
	}
	files := []rawUnitFile{
		{Name: "a.service", State: "enabled"},
		{Name: "b.service", State: "enabled"},
		{Name: "c.service", State: "disabled"},
		{Name: "d.service", State: "disabled"}, // disabled + not active -> excluded
	}

	got := mergeServices(units, files)
	want := []string{"a.service", "b.service", "c.service"} // a,b enabled; c active

	if len(got) != 3 {
		t.Fatalf("want 3, got %d (%v)", len(got), names(got))
	}
	for i, n := range want {
		if got[i].Name != n {
			t.Fatalf("sorted union mismatch at %d: got %v", i, names(got))
		}
	}
	// b is the enabled+inactive anomaly we want visible
	if got[1].Name != "b.service" || got[1].ActiveState != "inactive" {
		t.Fatalf("b mapping wrong: %+v", got[1])
	}
	// c is running but disabled
	if got[2].EnabledState != "disabled" || got[2].SubState != "exited" {
		t.Fatalf("c mapping wrong: %+v", got[2])
	}
}

func TestMergeServices_EnabledButNotLoaded(t *testing.T) {
	got := mergeServices(nil, []rawUnitFile{{Name: "x.service", State: "enabled"}})
	if len(got) != 1 || got[0].ActiveState != "inactive" || got[0].SubState != "dead" {
		t.Fatalf("expected inactive/dead default, got %+v", got)
	}
}

func TestMergeServices_ExcludesNonServiceUnitFiles(t *testing.T) {
	// ListUnitFiles returns all unit types; only .service units may enter the inventory.
	files := []rawUnitFile{
		{Name: "fstrim.timer", State: "enabled"},
		{Name: "ssh.socket", State: "enabled"},
		{Name: "multi-user.target", State: "enabled"},
		{Name: "nginx.service", State: "enabled"},
	}

	got := mergeServices(nil, files)
	if len(got) != 1 || got[0].Name != "nginx.service" {
		t.Fatalf("expected only nginx.service, got %v", names(got))
	}
}

func TestMergeServices_TemplateInstanceInheritsEnabled(t *testing.T) {
	units := []rawUnit{
		{Name: "systemd-fsck@dev-BOOT.service", Description: "File System Check", ActiveState: "active", SubState: "exited"},
	}
	files := []rawUnitFile{{Name: "systemd-fsck@.service", State: "static"}}

	got := mergeServices(units, files)
	if len(got) != 1 || got[0].EnabledState != "static" {
		t.Fatalf("instance should inherit template state 'static', got %v", got)
	}
}

func TestIsAvailable_NonLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		if New().IsAvailable() {
			t.Fatal("collector must be unavailable off Linux")
		}
	}
}
