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

func TestDeliveryAttemptAcceptsSupportedHeaderTypes(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		want  int
	}{
		{"int", int(2), 3},
		{"int32", int32(2), 3},
		{"int64", int64(2), 3},
		{"uint32", uint32(2), 3},
		{"uint64", uint64(2), 3},
		{"string", "2", 3},
		{"invalid", "not-a-number", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := deliveryAttempt(amqp.Table{"x-retry-count": test.value}); got != test.want {
				t.Fatalf("attempt = %d, want %d", got, test.want)
			}
		})
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
