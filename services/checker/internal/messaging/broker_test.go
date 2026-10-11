package messaging

import (
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestDeliveryAttemptStartsAtOne(t *testing.T) {
	if attempt := deliveryAttempt(nil); attempt != 1 {
		t.Fatalf("attempt = %d, want 1", attempt)
	}
}

func TestDeliveryAttemptUsesRetryCount(t *testing.T) {
	headers := amqp.Table{
		"x-retry-count": int32(2),
	}
	if attempt := deliveryAttempt(headers); attempt != 3 {
		t.Fatalf("attempt = %d, want 3", attempt)
	}
}

func TestPermanentErrorCanBeClassified(t *testing.T) {
	original := errors.New("invalid job")
	var permanent *PermanentError
	if !errors.As(Permanent(original), &permanent) {
		t.Fatal("permanent error was not classified")
	}
	if !errors.Is(Permanent(original), original) {
		t.Fatal("permanent error does not preserve its cause")
	}
}

func TestCloneHeadersDoesNotModifySource(t *testing.T) {
	source := amqp.Table{"key": "value"}
	copy := cloneHeaders(source)
	copy["key"] = "changed"
	if source["key"] != "value" {
		t.Fatalf("source header changed: %#v", source)
	}
}
