package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	contractsmessaging "github.com/KRUTONIK/web-service-monitoring/contracts/messaging"
	"github.com/KRUTONIK/web-service-monitoring/contracts/notifications"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func serviceName() string { return "Notification Service" }

type SMTPConfig struct {
	Address, From, Username, Password string
	TLS                               bool
}

// Send emits a UTF-8 plain-text notification. TLS validation is delegated to Go's
// standard library and is never disabled.
func Send(cfg SMTPConfig, event notifications.Event) error {
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
	defer connection.Close()
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return err
	}
	defer client.Quit()
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

type delivery struct {
	EventID   string `gorm:"primaryKey"`
	Status    string
	SentAt    *time.Time
	UpdatedAt time.Time
	LastError string
}

func (delivery) TableName() string { return "notification_deliveries" }

func main() {
	config := SMTPConfig{Address: value("SMTP_ADDRESS", "localhost:1025"), From: value("SMTP_FROM", "monitoring@example.test"), Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD"), TLS: os.Getenv("SMTP_TLS") == "true"}
	database, err := gorm.Open(postgres.Open(value("POSTGRES_DSN", "postgres://monitoring:monitoring@localhost:5432/monitoring?sslmode=disable")), &gorm.Config{})
	if err != nil {
		log.Fatalf("open PostgreSQL: %v", err)
	}
	for {
		connection, err := amqp.Dial(value("RABBITMQ_URL", "amqp://monitoring:monitoring@localhost:5672/"))
		if err == nil {
			err = consume(context.Background(), connection, database, config)
			_ = connection.Close()
		}
		log.Printf("notification consumer reconnect: %v", err)
		time.Sleep(time.Second)
	}
}
func value(name, fallback string) string {
	if result := os.Getenv(name); result != "" {
		return result
	}
	return fallback
}
func consume(ctx context.Context, connection *amqp.Connection, database *gorm.DB, config SMTPConfig) error {
	channel, err := connection.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	if _, err = channel.QueueDeclare(contractsmessaging.NotificationEventsQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if _, err = channel.QueueDeclare(contractsmessaging.NotificationRetryQueue, true, false, false, false, amqp.Table{"x-message-ttl": int32(5000), "x-dead-letter-exchange": "", "x-dead-letter-routing-key": contractsmessaging.NotificationEventsQueue}); err != nil {
		return err
	}
	if _, err = channel.QueueDeclare(contractsmessaging.NotificationDLQ, true, false, false, false, nil); err != nil {
		return err
	}
	messages, err := channel.Consume(contractsmessaging.NotificationEventsQueue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, ok := <-messages:
			if !ok {
				return fmt.Errorf("notification queue closed")
			}
			var event notifications.Event
			if err := json.Unmarshal(message.Body, &event); err != nil || event.ID == "" || event.Recipient == "" {
				if err := channel.Publish("", contractsmessaging.NotificationDLQ, false, false, amqp.Publishing{DeliveryMode: amqp.Persistent, Body: message.Body}); err != nil {
					return err
				}
				_ = message.Ack(false)
				continue
			}
			var sent delivery
			if database.Where("event_id = ? AND status = ?", event.ID, "sent").First(&sent).Error == nil {
				_ = message.Ack(false)
				continue
			}
			if err := Send(config, event); err != nil {
				attempts := int32(0)
				if v, ok := message.Headers["x-attempt"].(int32); ok {
					attempts = v
				}
				target := contractsmessaging.NotificationRetryQueue
				if attempts >= 4 {
					target = contractsmessaging.NotificationDLQ
				}
				if err := channel.Publish("", target, false, false, amqp.Publishing{DeliveryMode: amqp.Persistent, Headers: amqp.Table{"x-attempt": attempts + 1}, Body: message.Body}); err != nil {
					return err
				}
				_ = database.Save(&delivery{EventID: event.ID, Status: "failed", UpdatedAt: time.Now().UTC(), LastError: truncate(err.Error(), 500)}).Error
				log.Printf("notification event_id=%s service_id=%s type=%s attempt=%d error=%v", event.ID, event.ServiceID, event.Type, attempts+1, err)
				_ = message.Ack(false)
				continue
			}
			now := time.Now().UTC()
			if err := database.Save(&delivery{EventID: event.ID, Status: "sent", SentAt: &now, UpdatedAt: now}).Error; err != nil {
				return err
			}
			log.Printf("notification event_id=%s service_id=%s type=%s result=sent", event.ID, event.ServiceID, event.Type)
			if err := message.Ack(false); err != nil {
				return err
			}
		}
	}
}
