package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/contracts/jobs"
	contractsmessaging "github.com/KRUTONIK/web-service-monitoring/contracts/messaging"
	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	configUpdatesQueue = "scheduler.config.updates"
	maxQueuedJobs      = 1000
)

type Broker struct {
	connection *amqp.Connection
	jobTTL     time.Duration
	retryDelay time.Duration
}

func Open(rabbitMQURL string, jobTTL, retryDelay time.Duration) (*Broker, error) {
	connection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	broker := &Broker{connection: connection, jobTTL: jobTTL, retryDelay: retryDelay}
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
	if err := channel.Confirm(false); err != nil {
		return fmt.Errorf("enable job publisher confirms: %w", err)
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	returns := channel.NotifyReturn(make(chan amqp.Return, 1))

	scheduledAt := time.Now().UTC()
	for _, service := range services {
		job := jobs.NewCheckJob(service.ID, service.URL, scheduledAt)
		body, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("encode check job: %w", err)
		}
		if err := publishJob(ctx, channel, confirms, returns, body); err != nil {
			log.Printf("check job was not published service=%s job_id=%s reason=%v", service.ID, job.JobID, err)
			return fmt.Errorf("publish check job for service %s: %w", service.ID, err)
		}
	}
	return nil
}

func publishJob(ctx context.Context, channel *amqp.Channel, confirms <-chan amqp.Confirmation, returns <-chan amqp.Return, body []byte) error {
	if err := channel.PublishWithContext(ctx, "", contractsmessaging.CheckJobsQueue, true, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body}); err != nil {
		return err
	}
	return awaitJobPublish(ctx, confirms, returns)
}

func awaitJobPublish(ctx context.Context, confirms <-chan amqp.Confirmation, returns <-chan amqp.Return) error {
	select {
	case returned := <-returns:
		return fmt.Errorf("broker returned message: %s (%d)", returned.ReplyText, returned.ReplyCode)
	case confirmation, ok := <-confirms:
		if !ok {
			return fmt.Errorf("publisher confirm channel closed")
		}
		if !confirmation.Ack {
			return fmt.Errorf("broker rejected message")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
	if err := channel.ExchangeDeclare(contractsmessaging.CheckRetryExchange, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare retry exchange: %w", err)
	}
	arguments := jobQueueArguments(checkJobTTL(broker.jobTTL, broker.retryDelay))
	if _, err := channel.QueueDeclare(contractsmessaging.CheckJobsQueue, true, false, false, false, arguments); err != nil {
		return fmt.Errorf("declare queue %s: %w", contractsmessaging.CheckJobsQueue, err)
	}
	retryArguments := amqp.Table{
		"x-message-ttl":             int32(broker.retryDelay.Milliseconds()),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": contractsmessaging.CheckJobsQueue,
	}
	if _, err := channel.QueueDeclare(contractsmessaging.CheckRetryQueue, true, false, false, false, retryArguments); err != nil {
		return fmt.Errorf("declare retry queue %s: %w", contractsmessaging.CheckRetryQueue, err)
	}
	if err := channel.QueueBind(contractsmessaging.CheckRetryQueue, contractsmessaging.CheckRetryRoute, contractsmessaging.CheckRetryExchange, false, nil); err != nil {
		return fmt.Errorf("bind retry queue: %w", err)
	}
	if _, err := channel.QueueDeclare(contractsmessaging.CheckJobsDLQ, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dead-letter queue %s: %w", contractsmessaging.CheckJobsDLQ, err)
	}
	return nil
}

func jobQueueArguments(jobTTL time.Duration) amqp.Table {
	return amqp.Table{
		"x-max-length":  int32(maxQueuedJobs),
		"x-overflow":    "reject-publish",
		"x-message-ttl": int32(jobTTL.Milliseconds()),
	}
}

func checkJobTTL(configuredTTL, retryDelay time.Duration) time.Duration {
	retryWindow := retryDelay * time.Duration(contractsmessaging.MaxCheckJobAttempts+1)
	if retryWindow > configuredTTL {
		return retryWindow
	}
	return configuredTTL
}
