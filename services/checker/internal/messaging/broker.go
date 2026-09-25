package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/serviceconfig"
	amqp "github.com/rabbitmq/amqp091-go"
)

const configUpdatesQueue = "checker.config.updates"

type Broker struct{ connection *amqp.Connection }

func Open(rabbitMQURL string) (*Broker, error) {
	connection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	broker := &Broker{connection: connection}
	if err := broker.declareQueue(); err != nil {
		connection.Close()
		return nil, err
	}
	return broker, nil
}

func (broker *Broker) Close() error { return broker.connection.Close() }

func (broker *Broker) ConsumeUpdates(ctx context.Context, handler func(serviceconfig.Update) error) error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open configuration update channel: %w", err)
	}
	defer channel.Close()
	deliveries, err := channel.Consume(configUpdatesQueue, "checker-config", false, false, false, false, nil)
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
				delivery.Nack(false, false)
				continue
			}
			if err := handler(update); err != nil {
				delivery.Nack(false, true)
				continue
			}
			delivery.Ack(false)
		}
	}
}

func (broker *Broker) declareQueue() error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open declaration channel: %w", err)
	}
	defer channel.Close()
	if _, err := channel.QueueDeclare(configUpdatesQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", configUpdatesQueue, err)
	}
	return nil
}
