package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	contracts "github.com/KRUTONIK/web-service-monitoring/contracts/incidents"
	"github.com/KRUTONIK/web-service-monitoring/contracts/jobs"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
)

type stubWriter struct {
	result check.Result
	previous check.Result
	err error
}

func (stub *stubWriter) Latest(_ context.Context, _ string) (check.Result, error) { return stub.previous, stub.err }
type stubPublisher struct { events []contracts.Event }
func (stub *stubPublisher) PublishIncident(_ context.Context, event contracts.Event) error { stub.events = append(stub.events, event); return nil }

func (stub *stubWriter) Write(_ context.Context, result check.Result) error {
	stub.result = result
	return nil
}

func TestRunJobWritesCheckResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	writer := &stubWriter{}
	scheduledAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	job := jobs.NewCheckJob("service-1", server.URL, 5, scheduledAt)
	if err := runJob(context.Background(), check.New(server.Client()), writer, &stubPublisher{}, job); err != nil {
		t.Fatalf("run check job: %v", err)
	}
	if writer.result.JobID != job.JobID || !writer.result.CheckedAt.Equal(scheduledAt) || writer.result.ServiceID != "service-1" || !writer.result.Available || writer.result.StatusCode != http.StatusOK {
		t.Fatalf("unexpected result: %+v", writer.result)
	}
}

func TestRunJobPublishesOnlyAvailabilityTransitions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	writer := &stubWriter{previous: check.Result{Available: true}}
	publisher := &stubPublisher{}
	job := jobs.NewCheckJob("service-1", server.URL, 5, time.Now().UTC())
	if err := runJob(context.Background(), check.New(server.Client()), writer, publisher, job); err != nil { t.Fatalf("run check job: %v", err) }
	if len(publisher.events) != 1 || publisher.events[0].Type != contracts.Started { t.Fatalf("unexpected events: %+v", publisher.events) }
	writer.previous = writer.result
	if err := runJob(context.Background(), check.New(server.Client()), writer, publisher, job); err != nil { t.Fatalf("redelivery: %v", err) }
	if len(publisher.events) != 1 { t.Fatalf("redelivery created duplicate event: %+v", publisher.events) }
}
