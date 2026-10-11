package messaging

import (
	"context"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestJobQueueArgumentsRejectOverflow(t *testing.T) {
	arguments := jobQueueArguments(15 * time.Second)
	if arguments["x-overflow"] != "reject-publish" {
		t.Fatalf("overflow behavior = %v, want reject-publish", arguments["x-overflow"])
	}
	if arguments["x-max-length"] != int32(maxQueuedJobs) {
		t.Fatalf("max length = %v, want %d", arguments["x-max-length"], maxQueuedJobs)
	}
	if arguments["x-message-ttl"] != int32((15 * time.Second).Milliseconds()) {
		t.Fatalf("TTL = %v, want 15000", arguments["x-message-ttl"])
	}
}

func TestAwaitJobPublishAcceptsBrokerConfirmation(t *testing.T) {
	confirms := make(chan amqp.Confirmation, 1)
	returns := make(chan amqp.Return, 1)
	confirms <- amqp.Confirmation{Ack: true}
	if err := awaitJobPublish(context.Background(), confirms, returns); err != nil {
		t.Fatalf("await confirmation: %v", err)
	}
}

func TestAwaitJobPublishReturnsBrokerRejection(t *testing.T) {
	confirms := make(chan amqp.Confirmation, 1)
	returns := make(chan amqp.Return, 1)
	confirms <- amqp.Confirmation{Ack: false}
	if err := awaitJobPublish(context.Background(), confirms, returns); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAwaitJobPublishReturnsUnroutableMessage(t *testing.T) {
	confirms := make(chan amqp.Confirmation, 1)
	returns := make(chan amqp.Return, 1)
	returns <- amqp.Return{ReplyCode: 312, ReplyText: "NO_ROUTE"}
	if err := awaitJobPublish(context.Background(), confirms, returns); err == nil || !strings.Contains(err.Error(), "NO_ROUTE") {
		t.Fatalf("unexpected error: %v", err)
	}
}
