package messaging

import (
	"context"
	"fmt"
	"time"

	contractsmessaging "github.com/KRUTONIK/web-service-monitoring/contracts/messaging"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

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
	defer connection.Close()
	channel, err := connection.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	if _, err = channel.QueueDeclare(contractsmessaging.NotificationEventsQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if err = channel.Confirm(false); err != nil {
		return err
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []outboxRecord
		if err := tx.Raw("SELECT event_id, payload, attempts FROM notification_outbox WHERE published_at IS NULL ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 50").Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if err := channel.PublishWithContext(ctx, "", contractsmessaging.NotificationEventsQueue, true, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: row.Payload}); err != nil {
				return err
			}
			confirmation := <-confirms
			if !confirmation.Ack {
				return fmt.Errorf("notification event %s not confirmed", row.EventID)
			}
			if err := tx.Exec("UPDATE notification_outbox SET published_at = NOW(), attempts = attempts + 1, last_error = NULL WHERE event_id = ?", row.EventID).Error; err != nil {
				return err
			}
		}
		return nil
	})
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
