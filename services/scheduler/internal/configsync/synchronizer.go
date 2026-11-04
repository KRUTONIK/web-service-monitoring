package configsync

import (
	"context"
	"fmt"

	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
)

type snapshotClient interface {
	Snapshot(context.Context) (serviceconfig.Snapshot, error)
}

type Synchronizer struct {
	client snapshotClient
	state  *serviceconfig.State
}

func New(client snapshotClient, state *serviceconfig.State) *Synchronizer {
	return &Synchronizer{client: client, state: state}
}

func (synchronizer *Synchronizer) Sync(ctx context.Context) (int, error) {
	snapshot, err := synchronizer.client.Snapshot(ctx)
	if err != nil {
		return 0, fmt.Errorf("request configuration snapshot: %w", err)
	}
	return synchronizer.state.Reconcile(snapshot), nil
}
