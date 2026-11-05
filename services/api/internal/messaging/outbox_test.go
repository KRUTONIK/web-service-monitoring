package messaging

import (
	"context"
	"strings"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestWaitForNotificationPublication(t *testing.T) {
	t.Run("acknowledgement", func(t *testing.T) {
		confirms := make(chan amqp.Confirmation, 1)
		confirms <- amqp.Confirmation{Ack: true}
		if err := waitForNotificationPublication(context.Background(), confirms, make(chan amqp.Return), make(chan *amqp.Error), "event-1"); err != nil {
			t.Fatalf("wait confirmation: %v", err)
		}
	})
	t.Run("unroutable message", func(t *testing.T) {
		returns := make(chan amqp.Return, 1)
		returns <- amqp.Return{ReplyText: "NO_ROUTE"}
		err := waitForNotificationPublication(context.Background(), make(chan amqp.Confirmation), returns, make(chan *amqp.Error), "event-2")
		if err == nil || !strings.Contains(err.Error(), "not routed") {
			t.Fatalf("expected routing error, got %v", err)
		}
	})
	t.Run("closed confirmation channel", func(t *testing.T) {
		confirms := make(chan amqp.Confirmation)
		close(confirms)
		err := waitForNotificationPublication(context.Background(), confirms, make(chan amqp.Return), make(chan *amqp.Error), "event-3")
		if err == nil || !strings.Contains(err.Error(), "channel closed") {
			t.Fatalf("expected closed-channel error, got %v", err)
		}
	})
}
