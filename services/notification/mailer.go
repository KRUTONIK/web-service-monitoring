package main

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/contracts/notifications"
)

func Send(cfg SMTPConfig, event notifications.Event) (result error) {
	to, err := mail.ParseAddress(event.Recipient)
	if err != nil || strings.ContainsAny(event.Recipient, "\r\n") {
		return fmt.Errorf("invalid recipient")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil || strings.ContainsAny(cfg.From, "\r\n") {
		return fmt.Errorf("invalid sender")
	}
	host, _, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return err
	}
	subject, body := Compose(event, "")
	message := "From: " + from.String() + "\r\nTo: " + to.String() + "\r\nSubject: =?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?=\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body
	connection, err := net.DialTimeout("tcp", cfg.Address, 10*time.Second)
	if err != nil {
		return err
	}
	if err := connection.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		_ = connection.Close()
		return fmt.Errorf("set SMTP deadline: %w", err)
	}
	defer func() {
		// smtp.Client.Quit closes the underlying connection. Closing it again is
		// expected to report net.ErrClosed and must not turn a delivered email
		// into a retry.
		if err := connection.Close(); result == nil && err != nil && !errors.Is(err, net.ErrClosed) {
			result = fmt.Errorf("close SMTP connection: %w", err)
		}
	}()
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return err
	}
	defer func() {
		if err := client.Quit(); result == nil && err != nil {
			result = fmt.Errorf("finish SMTP session: %w", err)
		}
	}()
	if cfg.TLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if cfg.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, host)); err != nil {
			return err
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return err
	}
	if err := client.Rcpt(to.Address); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = writer.Write([]byte(message)); err != nil {
		return err
	}
	return writer.Close()
}

func Compose(event notifications.Event, webURL string) (string, string) {
	link := ""
	if webURL != "" {
		link = "\nСтраница сервиса: " + strings.TrimRight(webURL, "/") + "/" + event.ServiceID
	}
	if event.Type == "incident.resolved" {
		return "[Мониторинг] Восстановлен сервис «" + event.ServiceName + "»", fmt.Sprintf("Сервис: %s\nURL: %s\nВосстановлен: %s UTC\nНачало сбоя: %s UTC\nПродолжительность: %s\nТекущее состояние: доступен%s", event.ServiceName, event.ServiceURL, event.OccurredAt.UTC().Format(time.RFC3339), event.StartedAt.UTC().Format(time.RFC3339), event.OccurredAt.Sub(event.StartedAt).Round(time.Second), link)
	}
	details := ""
	if event.StatusCode != 0 {
		details += fmt.Sprintf("\nHTTP-код: %d", event.StatusCode)
	}
	if event.Error != "" {
		details += "\nОшибка: " + truncate(event.Error, 500)
	}
	return "[Мониторинг] Недоступен сервис «" + event.ServiceName + "»", fmt.Sprintf("Сервис: %s\nURL: %s\nНачало сбоя: %s UTC%s\nТекущее состояние: недоступен%s", event.ServiceName, event.ServiceURL, event.OccurredAt.UTC().Format(time.RFC3339), details, link)
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
