//go:build darwin

package install

import "testing"

func TestGetServiceManager_Darwin_ReturnsDarwinService(t *testing.T) {
	svc, err := GetServiceManager()
	if err != nil {
		t.Fatalf("expected no error on macOS, got: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil ServiceManager on macOS")
	}
	if _, ok := svc.(*DarwinService); !ok {
		t.Errorf("expected *DarwinService, got %T", svc)
	}
	if svc.RequiresRoot() {
		t.Error("DarwinService should not require root")
	}
}
