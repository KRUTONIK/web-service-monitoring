package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
	amqp "github.com/rabbitmq/amqp091-go"
)

const configUpdatesQueue = "checker.config.updates"

type configurationRepository interface {
	List(context.Context) ([]serviceconfig.Service, error)
}

type ConfigurationBroker struct {
	connection *amqp.Connection
	repository configurationRepository
}

func OpenConfigurationBroker(rabbitMQURL string, repository configurationRepository) (*ConfigurationBroker, error) {
	connection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	broker := &ConfigurationBroker{connection: connection, repository: repository}
	if err := broker.declareQueue(); err != nil {
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
		body, err := json.Marshal(serviceconfig.Update{Event: "service.updated", Service: service})
		if err != nil {
			return fmt.Errorf("encode configuration update: %w", err)
		}
		if err := channel.PublishWithContext(ctx, "", configUpdatesQueue, false, false, amqp.Publishing{
			ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body,
		}); err != nil {
			return fmt.Errorf("publish configuration update: %w", err)
		}
	}
	return nil
}

func (broker *ConfigurationBroker) declareQueue() error {
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
