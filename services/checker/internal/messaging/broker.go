package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KRUTONIK/web-service-monitoring/contracts/jobs"
	amqp "github.com/rabbitmq/amqp091-go"
)

const checkJobsQueue = "checker.jobs"

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
	deliveries, err := channel.Consume(checkJobsQueue, "", false, false, false, false, nil)
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
			var job jobs.CheckJob
			if err := json.Unmarshal(delivery.Body, &job); err != nil || job.ServiceID == "" || job.ServiceURL == "" {
				if nackErr := delivery.Nack(false, false); nackErr != nil {
					return fmt.Errorf("reject invalid check job: %w", nackErr)
				}
				continue
			}
			if err := handler(job); err != nil {
				if nackErr := delivery.Nack(false, true); nackErr != nil {
					return fmt.Errorf("requeue check job: %w", nackErr)
				}
				continue
			}
			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("acknowledge check job: %w", err)
			}
		}
	}
}

func (broker *Broker) declareQueue() error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open declaration channel: %w", err)
	}
	defer func() { _ = channel.Close() }()
	if _, err := channel.QueueDeclarePassive(checkJobsQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("check jobs queue is unavailable: %w", err)
	}
	return nil
}
