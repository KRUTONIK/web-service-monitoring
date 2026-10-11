package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/contracts/jobs"
	contractsmessaging "github.com/KRUTONIK/web-service-monitoring/contracts/messaging"
	amqp "github.com/rabbitmq/amqp091-go"
)

const maxJobAttempts = 3

// PermanentError marks a failure that must not be retried.
type PermanentError struct{ err error }

func (err *PermanentError) Error() string { return err.err.Error() }

func (err *PermanentError) Unwrap() error { return err.err }

func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{err: err}
}

type Broker struct{ connection *amqp.Connection }

func Open(rabbitMQURL string) (*Broker, error) {
	connection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	broker := &Broker{connection: connection}
	if err := broker.declareQueue(); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return broker, nil
}

func (broker *Broker) Close() error { return broker.connection.Close() }

func (broker *Broker) ConsumeJobs(ctx context.Context, handler func(jobs.CheckJob) error) error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open check jobs channel: %w", err)
	}
	defer func() { _ = channel.Close() }()
	if err := channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set check job prefetch: %w", err)
	}
	deliveries, err := channel.Consume(contractsmessaging.CheckJobsQueue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume check jobs: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("check jobs channel closed")
			}
			if err := broker.processDelivery(ctx, channel, delivery, handler); err != nil {
				return err
			}
		}
	}
}

func (broker *Broker) processDelivery(ctx context.Context, channel *amqp.Channel, delivery amqp.Delivery, handler func(jobs.CheckJob) error) error {
	attempt := deliveryAttempt(delivery.Headers)
	var job jobs.CheckJob
	if err := json.Unmarshal(delivery.Body, &job); err != nil || job.ServiceID == "" || job.ServiceURL == "" {
		reason := "invalid check job"
		if err != nil {
			reason = fmt.Sprintf("decode check job: %v", err)
		}
		return broker.moveToDLQ(ctx, channel, delivery, attempt, reason)
	}

	if err := handler(job); err != nil {
		var permanent *PermanentError
		if errors.As(err, &permanent) || attempt >= maxJobAttempts {
			return broker.moveToDLQ(ctx, channel, delivery, attempt, err.Error())
		}
		log.Printf("retry check job service=%s attempt=%d/%d error=%v", job.ServiceID, attempt, maxJobAttempts, err)
		if err := broker.moveToRetryQueue(ctx, channel, delivery, attempt, err.Error()); err != nil {
			return err
		}
		return nil
	}

	if err := delivery.Ack(false); err != nil {
		return fmt.Errorf("acknowledge check job: %w", err)
	}
	return nil
}

func (broker *Broker) moveToRetryQueue(ctx context.Context, channel *amqp.Channel, delivery amqp.Delivery, attempt int, reason string) error {
	headers := cloneHeaders(delivery.Headers)
	headers["x-retry-count"] = int32(attempt)
	headers["x-last-error"] = reason
	if err := channel.PublishWithContext(ctx, contractsmessaging.CheckRetryExchange, contractsmessaging.CheckRetryRoute, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Headers:      headers,
		Body:         delivery.Body,
	}); err != nil {
		return fmt.Errorf("publish check job to retry queue: %w", err)
	}
	if err := delivery.Ack(false); err != nil {
		return fmt.Errorf("acknowledge retried check job: %w", err)
	}
	return nil
}

func (broker *Broker) moveToDLQ(ctx context.Context, channel *amqp.Channel, delivery amqp.Delivery, attempt int, reason string) error {
	headers := cloneHeaders(delivery.Headers)
	headers["x-final-error"] = reason
	headers["x-attempts"] = int32(attempt)
	headers["x-failed-at"] = time.Now().UTC().Format(time.RFC3339Nano)
	if err := channel.PublishWithContext(ctx, "", contractsmessaging.CheckJobsDLQ, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Headers:      headers,
		Body:         delivery.Body,
	}); err != nil {
		return fmt.Errorf("publish check job to DLQ: %w", err)
	}
	log.Printf("dead-letter check job attempt=%d reason=%s", attempt, reason)
	if err := delivery.Ack(false); err != nil {
		return fmt.Errorf("acknowledge dead-lettered check job: %w", err)
	}
	return nil
}

func (broker *Broker) declareQueue() error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open declaration channel: %w", err)
	}
	defer func() { _ = channel.Close() }()
	if _, err := channel.QueueDeclarePassive(contractsmessaging.CheckJobsQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("check jobs queue is unavailable: %w", err)
	}
	if _, err := channel.QueueDeclarePassive(contractsmessaging.CheckJobsDLQ, true, false, false, false, nil); err != nil {
		return fmt.Errorf("check jobs dead-letter queue is unavailable: %w", err)
	}
	return nil
}

func deliveryAttempt(headers amqp.Table) int {
	return intValue(headers["x-retry-count"]) + 1
}

func intValue(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case int32:
		return int(number)
	case int64:
		return int(number)
	case uint32:
		return int(number)
	case uint64:
		return int(number)
	case string:
		parsed, _ := strconv.Atoi(number)
		return parsed
	default:
		return 0
	}
}

func cloneHeaders(source amqp.Table) amqp.Table {
	copy := make(amqp.Table, len(source)+3)
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
