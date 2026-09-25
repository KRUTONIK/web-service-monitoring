package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	configUpdatesQueue    = "checker.config.updates"
	snapshotRequestsQueue = "api.config.snapshot.requests"
)

type configurationRepository interface {
	List(context.Context) ([]serviceconfig.Service, error)
}

type ConfigurationBroker struct {
	connection *amqp.Connection
	repository configurationRepository
}

func OpenConfigurationBroker(
	rabbitMQURL string,
	repository configurationRepository,
) (*ConfigurationBroker, error) {
	connection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	broker := &ConfigurationBroker{
		connection: connection,
		repository: repository,
	}
	if err := broker.declareQueues(); err != nil {
		connection.Close()
		return nil, err
	}

	return broker, nil
}

func (broker *ConfigurationBroker) Close() error {
	return broker.connection.Close()
}

func (broker *ConfigurationBroker) PublishCurrentConfiguration(ctx context.Context) error {
	services, err := broker.repository.List(ctx)
	if err != nil {
		return err
	}

	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	defer channel.Close()

	for _, service := range services {
		body, err := json.Marshal(serviceconfig.Update{
			Event:   "service.updated",
			Service: service,
		})
		if err != nil {
			return fmt.Errorf("encode configuration update: %w", err)
		}

		if err := channel.PublishWithContext(ctx, "", configUpdatesQueue, false, false, amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		}); err != nil {
			return fmt.Errorf("publish configuration update: %w", err)
		}
	}

	return nil
}

func (broker *ConfigurationBroker) StartSnapshotResponder(ctx context.Context) error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open snapshot channel: %w", err)
	}

	deliveries, err := channel.Consume(
		snapshotRequestsQueue,
		"api-config-snapshot",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		channel.Close()
		return fmt.Errorf("consume snapshot requests: %w", err)
	}

	go func() {
		defer channel.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case delivery, ok := <-deliveries:
				if !ok {
					return
				}
				broker.respondWithSnapshot(ctx, channel, delivery)
			}
		}
	}()

	return nil
}

func (broker *ConfigurationBroker) respondWithSnapshot(
	ctx context.Context,
	channel *amqp.Channel,
	delivery amqp.Delivery,
) {
	services, err := broker.repository.List(ctx)
	if err != nil {
		delivery.Nack(false, true)
		return
	}

	body, err := json.Marshal(serviceconfig.Snapshot{Services: services})
	if err != nil {
		delivery.Nack(false, false)
		return
	}

	err = channel.PublishWithContext(ctx, "", delivery.ReplyTo, false, false, amqp.Publishing{
		ContentType:   "application/json",
		CorrelationId: delivery.CorrelationId,
		Body:          body,
	})
	if err != nil {
		delivery.Nack(false, true)
		return
	}

	delivery.Ack(false)
}

func (broker *ConfigurationBroker) declareQueues() error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open declaration channel: %w", err)
	}
	defer channel.Close()

	for _, name := range []string{configUpdatesQueue, snapshotRequestsQueue} {
		if _, err := channel.QueueDeclare(name, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare queue %s: %w", name, err)
		}
	}

	return nil
}
