package configsync

import (
	"context"
	"errors"
	"testing"

	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
)

type stubClient struct {
	snapshot serviceconfig.Snapshot
	err      error
}

func (stub *stubClient) Snapshot(context.Context) (serviceconfig.Snapshot, error) {
	return stub.snapshot, stub.err
}

func TestSyncPreservesConfigurationAfterErrorAndReconcilesNextSnapshot(t *testing.T) {
	state := serviceconfig.NewState()
	state.Replace(serviceconfig.Snapshot{Services: []serviceconfig.Service{{ID: "service-1", URL: "https://old.test", Enabled: true, Version: 1}}})
	client := &stubClient{err: errors.New("API unavailable")}
	synchronizer := New(client, state)

	if _, err := synchronizer.Sync(context.Background()); err == nil {
		t.Fatal("expected synchronization error")
	}
	if services := state.EnabledServices(); len(services) != 1 || services[0].URL != "https://old.test" {
		t.Fatalf("configuration was lost after error: %+v", services)
	}

	client.err = nil
	client.snapshot = serviceconfig.Snapshot{Services: []serviceconfig.Service{{ID: "service-1", URL: "https://new.test", Enabled: true, Version: 2}}}
	updated, err := synchronizer.Sync(context.Background())
	if err != nil || updated != 1 {
		t.Fatalf("unexpected synchronization result: updated=%d err=%v", updated, err)
	}
	if services := state.EnabledServices(); len(services) != 1 || services[0].URL != "https://new.test" {
		t.Fatalf("configuration was not reconciled: %+v", services)
	}
}
