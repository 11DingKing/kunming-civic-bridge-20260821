package fieldops_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/fieldops"
	"github.com/stretchr/testify/require"
)

func TestFieldOperationsHappyPathAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fieldops.json")
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	store, err := fieldops.Open(path)
	require.NoError(t, err)
	service := fieldops.NewService(store, func() time.Time { return now })
	_, err = service.CreateCampaign(context.Background(), fieldops.Campaign{ID: "summer", Tenant: "kunming", Quota: 3, OpensAt: now.Add(-time.Hour), ClosesAt: now.Add(time.Hour)})
	require.NoError(t, err)
	item, err := service.Submit(context.Background(), "kunming", "summer", "KM-001", []string{"community"})
	require.NoError(t, err)
	item, err = service.Transition(context.Background(), "kunming", item.ID, "received", "reviewing", item.Version)
	require.NoError(t, err)
	job, err := service.QueueJob(context.Background(), "kunming", item.ID, "feedback", "citizen-1")
	require.NoError(t, err)
	claimed, err := service.ClaimJobs(context.Background(), "kunming", "worker-a", 1, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed[0].ID)
	_, err = service.CompleteJob(context.Background(), "kunming", job.ID, "worker-a", "sent", nil, time.Second)
	require.NoError(t, err)
	require.NoError(t, store.Close())

	reopened, err := fieldops.Open(path)
	require.NoError(t, err)
	defer reopened.Close()
	reloaded := fieldops.NewService(reopened, func() time.Time { return now })
	got, err := reloaded.GetSuggestion(context.Background(), "kunming", item.ID)
	require.NoError(t, err)
	require.Equal(t, "reviewing", got.Status)
}

func TestFieldOperationsWorkerFollowsParentCancellation(t *testing.T) {
	store, err := fieldops.Open(filepath.Join(t.TempDir(), "fieldops.json"))
	require.NoError(t, err)
	defer store.Close()
	service := fieldops.NewService(store, time.Now)
	worker := fieldops.NewWorker(service, "kunming", "worker", time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, worker.Start(ctx))
	cancel()
	require.Eventually(t, func() bool { return !worker.Started() }, time.Second, time.Millisecond)
}
