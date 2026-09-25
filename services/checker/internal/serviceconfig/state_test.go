package serviceconfig

import "testing"

func TestApplyOnlyNewerVersion(t *testing.T) {
	state := NewState()
	state.Replace(Snapshot{Services: []Service{{
		ID:      "service-1",
		URL:     "https://current.test",
		Enabled: true,
		Version: 3,
	}}})

	if state.Apply(Service{ID: "service-1", URL: "https://old.test", Version: 2}) {
		t.Fatal("expected older configuration to be ignored")
	}
	if state.Apply(Service{ID: "service-1", URL: "https://duplicate.test", Version: 3}) {
		t.Fatal("expected duplicate configuration to be ignored")
	}
	if !state.Apply(Service{ID: "service-1", URL: "https://new.test", Version: 4}) {
		t.Fatal("expected newer configuration to be applied")
	}

	services := state.Services()
	if len(services) != 1 || services[0].URL != "https://new.test" || services[0].Version != 4 {
		t.Fatalf("unexpected state: %+v", services)
	}
}

func TestReplaceRemovesPreviousConfiguration(t *testing.T) {
	state := NewState()
	state.Apply(Service{ID: "old", Version: 1})
	state.Replace(Snapshot{Services: []Service{{ID: "current", Version: 2}}})

	services := state.Services()
	if len(services) != 1 || services[0].ID != "current" {
		t.Fatalf("unexpected state after snapshot: %+v", services)
	}
}
