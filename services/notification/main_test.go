package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/KRUTONIK/web-service-monitoring/contracts/notifications"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

func TestServiceName(t *testing.T) {
	if serviceName() != "Notification Service" {
		t.Fatalf("unexpected service name: %s", serviceName())
	}
}

func TestComposeFailureAndRecovery(t *testing.T) {
	started := time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	failure := notifications.Event{ID: "event-1", Type: "incident.started", ServiceID: "service-1", ServiceName: "Сайт", ServiceURL: "https://example.test", Recipient: "user@example.test", OccurredAt: started, StatusCode: 503, ResponseTimeMS: 125, Error: "временная ошибка"}
	subject, body := Compose(failure, "https://monitor.example")
	if subject != "[Мониторинг] Недоступен сервис «Сайт»" || !strings.Contains(body, "503") || !strings.Contains(body, "125 мс") || !strings.Contains(body, "временная ошибка") || !strings.Contains(body, "https://monitor.example/service-1") {
		t.Fatalf("failure message: %q %q", subject, body)
	}
	recovery := failure
	recovery.Type, recovery.OccurredAt, recovery.StartedAt = "incident.resolved", started.Add(95*time.Second), started
	subject, body = Compose(recovery, "https://monitor.example/")
	if subject != "[Мониторинг] Восстановлен сервис «Сайт»" || !strings.Contains(body, "1m35s") || !strings.Contains(body, "доступен") || !strings.Contains(body, "https://monitor.example/service-1") {
		t.Fatalf("recovery message: %q %q", subject, body)
	}
}

func TestDeliveryLookupResult(t *testing.T) {
	if sent, err := deliveryLookupResult(nil); !sent || err != nil {
		t.Fatalf("sent delivery: sent=%t err=%v", sent, err)
	}
	if sent, err := deliveryLookupResult(gorm.ErrRecordNotFound); sent || err != nil {
		t.Fatalf("missing delivery: sent=%t err=%v", sent, err)
	}
	databaseError := errors.New("database unavailable")
	if sent, err := deliveryLookupResult(databaseError); sent || !errors.Is(err, databaseError) {
		t.Fatalf("database error: sent=%t err=%v", sent, err)
	}
}

func TestSendRejectsInvalidAddressesBeforeConnecting(t *testing.T) {
	event := notifications.Event{Recipient: "owner@example.test\r\nBcc: other@example.test"}
	if err := Send(SMTPConfig{Address: "127.0.0.1:1", From: "monitoring@example.test"}, event); err == nil {
		t.Fatal("expected invalid recipient error")
	}
	if err := Send(SMTPConfig{Address: "127.0.0.1:1", From: "monitoring@example.test\r\nBcc: other@example.test"}, notifications.Event{Recipient: "owner@example.test"}); err == nil {
		t.Fatal("expected invalid sender error")
	}
}

func TestLoadSMTPConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		for _, name := range []string{"SMTP_ADDRESS", "SMTP_FROM", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_TLS", "WEB_PUBLIC_URL"} {
			t.Setenv(name, "")
		}
		got := loadSMTPConfig()
		want := SMTPConfig{Address: "localhost:1025", From: "monitoring@example.test"}
		if got != want {
			t.Fatalf("config = %+v, want %+v", got, want)
		}
	})
	t.Run("environment", func(t *testing.T) {
		t.Setenv("SMTP_ADDRESS", "smtp.example.test:587")
		t.Setenv("SMTP_FROM", "alerts@example.test")
		t.Setenv("SMTP_USERNAME", "monitor")
		t.Setenv("SMTP_PASSWORD", "secret")
		t.Setenv("SMTP_TLS", "true")
		t.Setenv("WEB_PUBLIC_URL", "https://monitor.example")
		got := loadSMTPConfig()
		want := SMTPConfig{Address: "smtp.example.test:587", From: "alerts@example.test", Username: "monitor", Password: "secret", TLS: true, WebPublicURL: "https://monitor.example"}
		if got != want {
			t.Fatalf("config = %+v, want %+v", got, want)
		}
	})
	t.Run("only literal true enables TLS", func(t *testing.T) {
		t.Setenv("SMTP_TLS", "TRUE")
		if loadSMTPConfig().TLS {
			t.Fatal("TLS enabled for a non-true value")
		}
	})
}

func TestComposeOptionalFieldsAndURLs(t *testing.T) {
	when := time.Date(2026, time.November, 5, 12, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60))
	event := notifications.Event{Type: "incident.started", ServiceID: "id", ServiceName: "Русский сервис", ServiceURL: "https://service.example", OccurredAt: when}
	_, body := Compose(event, "")
	for _, unexpected := range []string{"HTTP-код:", "Ошибка:", "Время ответа:", "Страница сервиса:"} {
		if strings.Contains(body, unexpected) {
			t.Fatalf("body unexpectedly contains %q: %q", unexpected, body)
		}
	}
	if !strings.Contains(body, "2026-11-05T09:00:00Z UTC") {
		t.Fatalf("time was not normalized to UTC: %q", body)
	}
	for _, publicURL := range []string{"https://monitor.example", "https://monitor.example/"} {
		_, body = Compose(event, publicURL)
		if strings.Count(body, "https://monitor.example/id") != 1 {
			t.Fatalf("URL %q produced bad service link: %q", publicURL, body)
		}
	}
}

func TestTruncatePreservesShortValues(t *testing.T) {
	if got := truncate("ошибка", len("ошибка")); got != "ошибка" {
		t.Fatalf("truncate at exact byte limit = %q", got)
	}
	if got := truncate("abcdef", 3); got != "abc…" {
		t.Fatalf("truncate = %q", got)
	}
}

func TestTruncateDoesNotSplitUTF8Characters(t *testing.T) {
	tests := []struct {
		name, value string
		limit       int
		want        string
	}{
		{name: "ASCII", value: "abcdef", limit: 3, want: "abc…"},
		{name: "Cyrillic", value: "ошибка", limit: 4, want: "ош…"},
		{name: "inside multibyte character", value: "аб", limit: 3, want: "а…"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := truncate(test.value, test.limit)
			if got != test.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", test.value, test.limit, got, test.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("truncate(%q, %d) returned invalid UTF-8: %q", test.value, test.limit, got)
			}
		})
	}
}

func TestWaitForNotificationPublication(t *testing.T) {
	returned := func(text string) amqp.Return { return amqp.Return{ReplyText: text} }
	tests := []struct {
		name  string
		setup func() (<-chan amqp.Confirmation, <-chan amqp.Return, <-chan *amqp.Error, context.Context)
		want  string
	}{
		{
			name: "ack", setup: func() (<-chan amqp.Confirmation, <-chan amqp.Return, <-chan *amqp.Error, context.Context) {
				confirms := make(chan amqp.Confirmation, 1)
				confirms <- amqp.Confirmation{Ack: true}
				return confirms, make(chan amqp.Return), make(chan *amqp.Error), context.Background()
			},
		},
		{
			name: "nack", setup: func() (<-chan amqp.Confirmation, <-chan amqp.Return, <-chan *amqp.Error, context.Context) {
				confirms := make(chan amqp.Confirmation, 1)
				confirms <- amqp.Confirmation{Ack: false}
				return confirms, make(chan amqp.Return), make(chan *amqp.Error), context.Background()
			}, want: "not confirmed",
		},
		{
			name: "returned takes priority over ack", setup: func() (<-chan amqp.Confirmation, <-chan amqp.Return, <-chan *amqp.Error, context.Context) {
				confirms := make(chan amqp.Confirmation, 1)
				confirms <- amqp.Confirmation{Ack: true}
				returns := make(chan amqp.Return, 1)
				returns <- returned("no route")
				return confirms, returns, make(chan *amqp.Error), context.Background()
			}, want: "not routed: no route",
		},
		{
			name: "closed confirmation channel", setup: func() (<-chan amqp.Confirmation, <-chan amqp.Return, <-chan *amqp.Error, context.Context) {
				confirms := make(chan amqp.Confirmation)
				close(confirms)
				return confirms, make(chan amqp.Return), make(chan *amqp.Error), context.Background()
			}, want: "not confirmed",
		},
		{
			name: "channel error", setup: func() (<-chan amqp.Confirmation, <-chan amqp.Return, <-chan *amqp.Error, context.Context) {
				closed := make(chan *amqp.Error, 1)
				closed <- &amqp.Error{Code: 320, Reason: "closed"}
				return make(chan amqp.Confirmation), make(chan amqp.Return), closed, context.Background()
			}, want: "channel closed: Exception (320) Reason: \"closed\"",
		},
		{
			name: "cancelled context", setup: func() (<-chan amqp.Confirmation, <-chan amqp.Return, <-chan *amqp.Error, context.Context) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return make(chan amqp.Confirmation), make(chan amqp.Return), make(chan *amqp.Error), ctx
			}, want: "context canceled",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			confirms, returns, closed, ctx := test.setup()
			err := waitForNotificationPublication(ctx, confirms, returns, closed, "target")
			if test.want == "" && err != nil {
				t.Fatalf("wait = %v", err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("wait = %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestDeliveryLookupResultWrapsOnlyDatabaseFailures(t *testing.T) {
	databaseError := fmt.Errorf("query: %w", errors.New("offline"))
	_, err := deliveryLookupResult(databaseError)
	if !errors.Is(err, databaseError) {
		t.Fatalf("error = %v, want original database error", err)
	}
}
