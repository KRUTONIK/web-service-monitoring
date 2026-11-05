package main

import (
	"strings"
	"testing"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/contracts/notifications"
)

func TestServiceName(t *testing.T) {
	if serviceName() != "Notification Service" {
		t.Fatalf("unexpected service name: %s", serviceName())
	}
}

func TestComposeFailureAndRecovery(t *testing.T) {
	started := time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	failure := notifications.Event{ID: "event-1", Type: "incident.started", ServiceID: "service-1", ServiceName: "Сайт", ServiceURL: "https://example.test", Recipient: "user@example.test", OccurredAt: started, StatusCode: 503, Error: "временная ошибка"}
	subject, body := Compose(failure, "https://monitor.example")
	if subject != "[Мониторинг] Недоступен сервис «Сайт»" || !strings.Contains(body, "503") || !strings.Contains(body, "временная ошибка") || !strings.Contains(body, "https://monitor.example/service-1") {
		t.Fatalf("failure message: %q %q", subject, body)
	}
	recovery := failure
	recovery.Type, recovery.OccurredAt, recovery.StartedAt = "incident.resolved", started.Add(95*time.Second), started
	subject, body = Compose(recovery, "")
	if subject != "[Мониторинг] Восстановлен сервис «Сайт»" || !strings.Contains(body, "1m35s") || !strings.Contains(body, "доступен") {
		t.Fatalf("recovery message: %q %q", subject, body)
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
