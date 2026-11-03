package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	contracts "github.com/KRUTONIK/web-service-monitoring/contracts/incidents"
	contractsmessaging "github.com/KRUTONIK/web-service-monitoring/contracts/messaging"
	amqp "github.com/rabbitmq/amqp091-go"
)

type incidentProcessor interface {
	Process(context.Context, contracts.Event) error
}

type IncidentConsumer struct {
	connection *amqp.Connection
	processor  incidentProcessor
}

func OpenIncidentConsumer(url string, processor incidentProcessor) (*IncidentConsumer, error) {
	connection, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect incident consumer: %w", err)
	}
	consumer := &IncidentConsumer{connection: connection, processor: processor}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	defer func() { _ = channel.Close() }()
	if _, err := channel.QueueDeclare(contractsmessaging.IncidentEventsQueue, true, false, false, false, nil); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("declare incidents queue: %w", err)
	}
	return consumer, nil
}

func (consumer *IncidentConsumer) Close() error { return consumer.connection.Close() }

func (consumer *IncidentConsumer) Consume(ctx context.Context) error {
	channel, err := consumer.connection.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = channel.Close() }()
	deliveries, err := channel.Consume(contractsmessaging.IncidentEventsQueue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("incidents channel closed")
			}
			var event contracts.Event
			if err := json.Unmarshal(delivery.Body, &event); err != nil || event.ID == "" || event.ServiceID == "" {
				_ = delivery.Nack(false, false)
				continue
			}
			if err := consumer.processor.Process(ctx, event); err != nil {
				return err
			}
			if err := delivery.Ack(false); err != nil {
				return err
			}
		}
	}
}
