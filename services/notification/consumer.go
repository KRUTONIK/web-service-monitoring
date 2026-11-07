package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	contractsmessaging "github.com/KRUTONIK/web-service-monitoring/contracts/messaging"
	"github.com/KRUTONIK/web-service-monitoring/contracts/notifications"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

type delivery struct {
	EventID   string `gorm:"primaryKey"`
	Status    string
	SentAt    *time.Time
	UpdatedAt time.Time
	LastError string
}

func (delivery) TableName() string { return "notification_deliveries" }

func consume(ctx context.Context, connection *amqp.Connection, database *gorm.DB, config SMTPConfig) error {
	channel, err := connection.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = channel.Close() }()
	if _, err = channel.QueueDeclare(contractsmessaging.NotificationEventsQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if _, err = channel.QueueDeclare(contractsmessaging.NotificationRetryQueue, true, false, false, false, amqp.Table{"x-message-ttl": int32(5000), "x-dead-letter-exchange": "", "x-dead-letter-routing-key": contractsmessaging.NotificationEventsQueue}); err != nil {
		return err
	}
	if _, err = channel.QueueDeclare(contractsmessaging.NotificationDLQ, true, false, false, false, nil); err != nil {
		return err
	}
	if err = channel.Confirm(false); err != nil {
		return err
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	returns := channel.NotifyReturn(make(chan amqp.Return, 1))
	closed := channel.NotifyClose(make(chan *amqp.Error, 1))
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
				if err := publishConfirmed(ctx, channel, confirms, returns, closed, contractsmessaging.NotificationDLQ, message.Body, nil); err != nil {
					return err
				}
				if err := message.Ack(false); err != nil {
					return err
				}
				continue
			}
			alreadySent, err := deliveryAlreadySent(database, event.ID)
			if err != nil {
				return fmt.Errorf("find notification delivery: %w", err)
			}
			if alreadySent {
				if err := message.Ack(false); err != nil {
					return err
				}
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
				if err := publishConfirmed(ctx, channel, confirms, returns, closed, target, message.Body, amqp.Table{"x-attempt": attempts + 1}); err != nil {
					return err
				}
				if err := database.Save(&delivery{EventID: event.ID, Status: "failed", UpdatedAt: time.Now().UTC(), LastError: truncate(err.Error(), 500)}).Error; err != nil {
					return err
				}
				log.Printf("notification event_id=%s service_id=%s type=%s attempt=%d error=%v", event.ID, event.ServiceID, event.Type, attempts+1, err)
				if err := message.Ack(false); err != nil {
					return err
				}
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

// deliveryAlreadySent distinguishes an absent delivery from an unavailable
// database. Only an explicit not-found result permits an SMTP attempt.
func deliveryAlreadySent(database *gorm.DB, eventID string) (bool, error) {
	var sent delivery
	err := database.Where("event_id = ? AND status = ?", eventID, "sent").First(&sent).Error
	return deliveryLookupResult(err)
}

func deliveryLookupResult(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func publishConfirmed(parent context.Context, channel *amqp.Channel, confirms <-chan amqp.Confirmation, returns <-chan amqp.Return, closed <-chan *amqp.Error, target string, body []byte, headers amqp.Table) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if err := channel.PublishWithContext(ctx, "", target, true, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Headers: headers, Body: body}); err != nil {
		return err
	}
	return waitForNotificationPublication(ctx, confirms, returns, closed, target)
}

// waitForNotificationPublication waits for the broker's outcome after a
// message has been published. Keeping this separate makes the confirmation
// policy independently testable without a RabbitMQ server.
func waitForNotificationPublication(ctx context.Context, confirms <-chan amqp.Confirmation, returns <-chan amqp.Return, closed <-chan *amqp.Error, target string) error {
	for {
		select {
		case returned, ok := <-returns:
			if !ok {
				returns = nil
				continue
			}
			return fmt.Errorf("notification publication to %s was not routed: %s", target, returned.ReplyText)
		case confirmation, ok := <-confirms:
			if !ok || !confirmation.Ack {
				return fmt.Errorf("notification publication to %s was not confirmed", target)
			}
			select {
			case returned, ok := <-returns:
				if ok {
					return fmt.Errorf("notification publication to %s was not routed: %s", target, returned.ReplyText)
				}
			default:
			}
			return nil
		case amqpErr, ok := <-closed:
			if !ok || amqpErr == nil {
				return fmt.Errorf("notification channel closed")
			}
			return fmt.Errorf("notification channel closed: %w", amqpErr)
		case <-ctx.Done():
			return fmt.Errorf("wait for notification publication: %w", ctx.Err())
		}
	}
}
