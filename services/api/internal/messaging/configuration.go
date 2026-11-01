package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	configUpdatesQueue  = "scheduler.config.updates"
	rabbitMQDialTimeout = 3 * time.Second
)

type configurationRepository interface {
	List(context.Context) ([]serviceconfig.Service, error)
}

type rabbitChannel interface {
	Close() error
	PublishWithContext(context.Context, string, string, bool, bool, amqp.Publishing) error
	QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error)
}

type rabbitConnection interface {
	Channel() (rabbitChannel, error)
	Close() error
	IsClosed() bool
}

type amqpConnection struct {
	connection *amqp.Connection
}

func (connection amqpConnection) Channel() (rabbitChannel, error) {
	return connection.connection.Channel()
}

func (connection amqpConnection) Close() error {
	return connection.connection.Close()
}

func (connection amqpConnection) IsClosed() bool {
	return connection.connection.IsClosed()
}

type connectionDialer func(string) (rabbitConnection, error)

type ConfigurationBroker struct {
	mu          sync.Mutex
	rabbitMQURL string
	connection  rabbitConnection
	dial        connectionDialer
	closed      bool
	repository  configurationRepository
}

func OpenConfigurationBroker(rabbitMQURL string, repository configurationRepository) (*ConfigurationBroker, error) {
	broker := &ConfigurationBroker{rabbitMQURL: rabbitMQURL, repository: repository, dial: dialRabbitMQ}
	if err := broker.connect(); err != nil {
		return nil, err
	}
	return broker, nil
}

func dialRabbitMQ(rabbitMQURL string) (rabbitConnection, error) {
	dialer := net.Dialer{Timeout: rabbitMQDialTimeout}
	connection, err := amqp.DialConfig(rabbitMQURL, amqp.Config{
		Dial: dialer.Dial,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	return amqpConnection{connection: connection}, nil
}

func (broker *ConfigurationBroker) Close() error {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.closed {
		return nil
	}
	broker.closed = true
	if broker.connection == nil {
		return nil
	}
	return broker.connection.Close()
}

func (broker *ConfigurationBroker) PublishCurrentConfiguration(ctx context.Context) error {
	services, err := broker.repository.List(ctx)
	if err != nil {
		return err
	}
	channel, err := broker.openChannel()
	if err != nil {
		return fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	defer func() { _ = channel.Close() }()

	for _, service := range services {
		if err := publish(channel, ctx, serviceconfig.Update{Event: "service.updated", Service: service}); err != nil {
			return err
		}
	}
	return nil
}

func (broker *ConfigurationBroker) Publish(ctx context.Context, update serviceconfig.Update) error {
	channel, err := broker.openChannel()
	if err != nil {
		return fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	defer func() { _ = channel.Close() }()

	return publish(channel, ctx, update)
}

func publish(channel rabbitChannel, ctx context.Context, update serviceconfig.Update) error {
	body, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("encode configuration update: %w", err)
	}
	if err := channel.PublishWithContext(ctx, "", configUpdatesQueue, false, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body,
	}); err != nil {
		return fmt.Errorf("publish configuration update: %w", err)
	}

	return nil
}

func (broker *ConfigurationBroker) openChannel() (rabbitChannel, error) {
	broker.mu.Lock()
	defer broker.mu.Unlock()

	if broker.closed {
		return nil, errors.New("configuration broker is closed")
	}
	if broker.connection == nil || broker.connection.IsClosed() {
		if err := broker.connectLocked(); err != nil {
			return nil, err
		}
	}
	channel, err := broker.connection.Channel()
	if err == nil {
		return channel, nil
	}
	if !broker.connection.IsClosed() {
		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	if err := broker.connectLocked(); err != nil {
		return nil, err
	}
	channel, err = broker.connection.Channel()
	if err != nil {
		return nil, fmt.Errorf("open RabbitMQ channel after reconnect: %w", err)
	}
	return channel, nil
}

func (broker *ConfigurationBroker) connect() error {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	return broker.connectLocked()
}

func (broker *ConfigurationBroker) connectLocked() error {
	connection, err := broker.dial(broker.rabbitMQURL)
	if err != nil {
		return fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	if err := declareQueue(connection); err != nil {
		_ = connection.Close()
		return err
	}
	broker.connection = connection
	return nil
}

func declareQueue(connection rabbitConnection) error {
	channel, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("open declaration channel: %w", err)
	}
	defer func() { _ = channel.Close() }()
	if _, err := channel.QueueDeclare(configUpdatesQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", configUpdatesQueue, err)
	}
	return nil
}
