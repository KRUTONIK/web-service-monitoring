package messaging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/serviceconfig"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/storage"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	configUpdatesQueue    = "checker.config.updates"
	snapshotRequestsQueue = "api.config.snapshot.requests"
	resultRequestsQueue   = "checker.results.requests"
)

type Broker struct {
	connection *amqp.Connection
}

type resultReader interface {
	Latest(context.Context) (check.Result, error)
}

type resultResponse struct {
	Result *check.Result `json:"result,omitempty"`
	Error  string        `json:"error,omitempty"`
}

func Open(rabbitMQURL string) (*Broker, error) {
	connection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	broker := &Broker{connection: connection}
	if err := broker.declareQueues(); err != nil {
		connection.Close()
		return nil, err
	}
	return broker, nil
}

func (broker *Broker) Close() error {
	return broker.connection.Close()
}

func (broker *Broker) RequestSnapshot(ctx context.Context) (serviceconfig.Snapshot, error) {
	channel, err := broker.connection.Channel()
	if err != nil {
		return serviceconfig.Snapshot{}, fmt.Errorf("open snapshot channel: %w", err)
	}
	defer channel.Close()

	replyQueue, err := channel.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return serviceconfig.Snapshot{}, fmt.Errorf("declare snapshot reply queue: %w", err)
	}
	replies, err := channel.Consume(replyQueue.Name, "", true, true, false, false, nil)
	if err != nil {
		return serviceconfig.Snapshot{}, fmt.Errorf("consume snapshot reply: %w", err)
	}

	correlationID, err := newCorrelationID()
	if err != nil {
		return serviceconfig.Snapshot{}, err
	}
	if err := channel.PublishWithContext(ctx, "", snapshotRequestsQueue, false, false, amqp.Publishing{
		CorrelationId: correlationID,
		ReplyTo:       replyQueue.Name,
	}); err != nil {
		return serviceconfig.Snapshot{}, fmt.Errorf("publish snapshot request: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return serviceconfig.Snapshot{}, fmt.Errorf("wait for configuration snapshot: %w", ctx.Err())
		case reply, ok := <-replies:
			if !ok {
				return serviceconfig.Snapshot{}, fmt.Errorf("configuration snapshot reply channel closed")
			}
			if reply.CorrelationId != correlationID {
				continue
			}

			var snapshot serviceconfig.Snapshot
			if err := json.Unmarshal(reply.Body, &snapshot); err != nil {
				return serviceconfig.Snapshot{}, fmt.Errorf("decode configuration snapshot: %w", err)
			}
			return snapshot, nil
		}
	}
}

func (broker *Broker) ConsumeUpdates(
	ctx context.Context,
	handler func(serviceconfig.Update) error,
) error {
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

func (broker *Broker) ServeResultRequests(ctx context.Context, reader resultReader) error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open result request channel: %w", err)
	}
	defer channel.Close()

	deliveries, err := channel.Consume(resultRequestsQueue, "checker-results", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume result requests: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("result request channel closed")
			}

			response := resultResponse{}
			result, readErr := reader.Latest(ctx)
			switch {
			case readErr == nil:
				response.Result = &result
			case errors.Is(readErr, storage.ErrNotFound):
				response.Error = "not_found"
			default:
				response.Error = "storage_error"
			}

			body, marshalErr := json.Marshal(response)
			if marshalErr != nil {
				delivery.Nack(false, true)
				continue
			}
			if delivery.ReplyTo == "" {
				delivery.Ack(false)
				continue
			}
			if err := channel.PublishWithContext(ctx, "", delivery.ReplyTo, false, false, amqp.Publishing{
				ContentType:   "application/json",
				CorrelationId: delivery.CorrelationId,
				Body:          body,
			}); err != nil {
				delivery.Nack(false, true)
				continue
			}
			delivery.Ack(false)
		}
	}
}

func (broker *Broker) declareQueues() error {
	channel, err := broker.connection.Channel()
	if err != nil {
		return fmt.Errorf("open declaration channel: %w", err)
	}
	defer channel.Close()

	for _, name := range []string{configUpdatesQueue, snapshotRequestsQueue, resultRequestsQueue} {
		if _, err := channel.QueueDeclare(name, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare queue %s: %w", name, err)
		}
	}
	return nil
}

func newCorrelationID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("create correlation ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}
