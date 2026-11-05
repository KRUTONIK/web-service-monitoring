package messaging

import (
	"context"
	"fmt"
	"time"

	contractsmessaging "github.com/KRUTONIK/web-service-monitoring/contracts/messaging"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

const outboxPublishTimeout = 10 * time.Second

type outboxRecord struct {
	EventID  string
	Payload  []byte
	Attempts int
}

func (outboxRecord) TableName() string { return "notification_outbox" }

// PublishNotificationOutbox claims rows with SKIP LOCKED and marks them sent
// only after a RabbitMQ publisher confirmation.
func PublishNotificationOutbox(ctx context.Context, database *gorm.DB, rabbitURL string) error {
	connection, err := amqp.Dial(rabbitURL)
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()
	channel, err := connection.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = channel.Close() }()
	if _, err = channel.QueueDeclare(contractsmessaging.NotificationEventsQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if err = channel.Confirm(false); err != nil {
		return err
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	returns := channel.NotifyReturn(make(chan amqp.Return, 1))
	closed := channel.NotifyClose(make(chan *amqp.Error, 1))
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []outboxRecord
		if err := tx.Raw("SELECT event_id, payload, attempts FROM notification_outbox WHERE published_at IS NULL ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 50").Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			publishContext, cancel := context.WithTimeout(ctx, outboxPublishTimeout)
			err := channel.PublishWithContext(publishContext, "", contractsmessaging.NotificationEventsQueue, true, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: row.Payload})
			if err == nil {
				err = waitForNotificationPublication(publishContext, confirms, returns, closed, row.EventID)
			}
			cancel()
			if err != nil {
				return err
			}
			if err := tx.Exec("UPDATE notification_outbox SET published_at = NOW(), attempts = attempts + 1, last_error = NULL WHERE event_id = ?", row.EventID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// waitForNotificationPublication observes every broker outcome that can make a
// mandatory publish unsafe. RabbitMQ orders basic.return before basic.ack for a
// message, so seeing a confirmation after checking returns means the message
// was both routable and durably accepted by the broker.
func waitForNotificationPublication(ctx context.Context, confirms <-chan amqp.Confirmation, returns <-chan amqp.Return, closed <-chan *amqp.Error, eventID string) error {
	for {
		select {
		case returned, ok := <-returns:
			if !ok {
				returns = nil
				continue
			}
			return fmt.Errorf("notification event %s was not routed: %s", eventID, returned.ReplyText)
		case confirmation, ok := <-confirms:
			if !ok {
				return fmt.Errorf("notification event %s confirmation channel closed", eventID)
			}
			if !confirmation.Ack {
				return fmt.Errorf("notification event %s not confirmed", eventID)
			}
			select {
			case returned, ok := <-returns:
				if ok {
					return fmt.Errorf("notification event %s was not routed: %s", eventID, returned.ReplyText)
				}
			default:
			}
			return nil
		case amqpErr, ok := <-closed:
			if !ok || amqpErr == nil {
				return fmt.Errorf("notification event %s channel closed", eventID)
			}
			return fmt.Errorf("notification event %s channel closed: %w", eventID, amqpErr)
		case <-ctx.Done():
			return fmt.Errorf("wait for notification event %s confirmation: %w", eventID, ctx.Err())
		}
	}
}

func RunNotificationOutbox(ctx context.Context, database *gorm.DB, rabbitURL string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := PublishNotificationOutbox(ctx, database, rabbitURL); err != nil {
			fmt.Printf("publish notification outbox: %v\n", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
