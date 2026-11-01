package serviceconfig

import "testing"

func TestStateAppliesOnlyNewerConfiguration(t *testing.T) {
	state := NewState()
	state.Replace(Snapshot{Services: []Service{{ID: "service-1", URL: "https://old.test", Enabled: true, Version: 2}}})
	if state.Apply(Service{ID: "service-1", URL: "https://stale.test", Enabled: true, Version: 2}) {
		t.Fatal("expected stale update to be ignored")
	}
	if !state.Apply(Service{ID: "service-1", URL: "https://new.test", Enabled: true, Version: 3}) {
		t.Fatal("expected newer update to be applied")
	}
	services := state.EnabledServices()
	if len(services) != 1 || services[0].URL != "https://new.test" {
		t.Fatalf("unexpected services: %+v", services)
	}
}

func TestStateDoesNotRestoreDeletedServiceFromSnapshot(t *testing.T) {
	state := NewState()
	state.Replace(Snapshot{Services: []Service{{ID: "service-1", URL: "https://service.test", Enabled: true, Version: 1}}})
	state.Apply(Service{ID: "service-1", URL: "https://service.test", Enabled: false, Version: 2})
	if services := state.EnabledServices(); len(services) != 0 {
		t.Fatalf("deleted service is still enabled: %+v", services)
	}

	state.Replace(Snapshot{Services: nil})
	if services := state.EnabledServices(); len(services) != 0 {
		t.Fatalf("deleted service was restored by snapshot: %+v", services)
	}
}

func TestStateReconcilesOnlyNewerSnapshotServices(t *testing.T) {
	state := NewState()
	state.Replace(Snapshot{Services: []Service{{ID: "existing", URL: "https://current.test", Enabled: true, Version: 3}}})
	updated := state.Reconcile(Snapshot{Services: []Service{
		{ID: "existing", URL: "https://stale.test", Enabled: true, Version: 2},
		{ID: "new", URL: "https://new.test", Enabled: true, Version: 1},
		{ID: "deleted", URL: "https://deleted.test", Enabled: false, Version: 4},
	}})
	if updated != 2 {
		t.Fatalf("unexpected number of reconciled services: %d", updated)
	}
	services := state.EnabledServices()
	if len(services) != 2 || services[0].ID != "existing" || services[0].URL != "https://current.test" || services[1].ID != "new" {
		t.Fatalf("unexpected enabled services after reconciliation: %+v", services)
	}
}
