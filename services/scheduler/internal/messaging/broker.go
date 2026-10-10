package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/contracts/jobs"
	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	configUpdatesQueue = "scheduler.config.updates"
	checkJobsQueue     = "checker.jobs"
	maxQueuedJobs      = 1000
)

type Broker struct {
	connection *amqp.Connection
	jobTTL     time.Duration
}

func Open(rabbitMQURL string, jobTTL time.Duration) (*Broker, error) {
	connection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	broker := &Broker{connection: connection, jobTTL: jobTTL}
	if err := broker.declareQueues(); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return broker, nil
}

func (broker *Broker) Close() error {
	return broker.connection.Close()
}

func (broker *Broker) PublishJobs(ctx context.Context, services []serviceconfig.Service) error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open job publication channel: %w", err)
	}
	defer func() { _ = channel.Close() }()

	scheduledAt := time.Now().UTC()
	for _, service := range services {
		job := jobs.CheckJob{ServiceID: service.ID, ServiceURL: service.URL, ScheduledAt: scheduledAt}
		body, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("encode check job: %w", err)
		}
		if err := channel.PublishWithContext(ctx, "", checkJobsQueue, false, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body}); err != nil {
			return fmt.Errorf("publish check job: %w", err)
		}
	}
	return nil
}

func (broker *Broker) ConsumeUpdates(ctx context.Context, handler func(serviceconfig.Update) error) error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open configuration update channel: %w", err)
	}
	defer func() { _ = channel.Close() }()
	deliveries, err := channel.Consume(configUpdatesQueue, "scheduler-config", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume configuration updates: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("configuration update channel closed")
			}
			var update serviceconfig.Update
			if err := json.Unmarshal(delivery.Body, &update); err != nil {
				if nackErr := delivery.Nack(false, false); nackErr != nil {
					return fmt.Errorf("reject invalid configuration update: %w", nackErr)
				}
				continue
			}
			if err := handler(update); err != nil {
				if nackErr := delivery.Nack(false, true); nackErr != nil {
					return fmt.Errorf("requeue configuration update: %w", nackErr)
				}
				continue
			}
			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("acknowledge configuration update: %w", err)
			}
		}
	}
}

func (broker *Broker) declareQueues() error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open declaration channel: %w", err)
	}
	defer func() { _ = channel.Close() }()
	if _, err := channel.QueueDeclare(configUpdatesQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", configUpdatesQueue, err)
	}
	arguments := amqp.Table{
		"x-max-length":  int32(maxQueuedJobs),
		"x-overflow":    "drop-head",
		"x-message-ttl": int32(broker.jobTTL.Milliseconds()),
	}
	if _, err := channel.QueueDeclare(checkJobsQueue, true, false, false, false, arguments); err != nil {
		return fmt.Errorf("declare queue %s: %w", checkJobsQueue, err)
	}
	return nil
}
