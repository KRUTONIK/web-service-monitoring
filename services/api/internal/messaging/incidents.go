package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	contracts "github.com/KRUTONIK/web-service-monitoring/contracts/incidents"
	contractsmessaging "github.com/KRUTONIK/web-service-monitoring/contracts/messaging"
	amqp "github.com/rabbitmq/amqp091-go"
)

type incidentProcessor interface {
	Process(context.Context, contracts.Event) error
}

type IncidentConsumer struct {
	connection *amqp.Connection
	url        string
	processor  incidentProcessor
}

func OpenIncidentConsumer(url string, processor incidentProcessor) (*IncidentConsumer, error) {
	connection, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect incident consumer: %w", err)
	}
	consumer := &IncidentConsumer{connection: connection, url: url, processor: processor}
	if err := consumer.declare(); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("declare incidents queue: %w", err)
	}
	return consumer, nil
}

const incidentReconnectDelay = time.Second

func (consumer *IncidentConsumer) Close() error { return consumer.connection.Close() }

func (consumer *IncidentConsumer) Consume(ctx context.Context) error {
	for {
		err := consumer.consumeOnce(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			_ = consumer.Close()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(incidentReconnectDelay):
		}
		connection, dialErr := amqp.Dial(consumer.url)
		if dialErr != nil {
			continue
		}
		consumer.connection = connection
		if declareErr := consumer.declare(); declareErr != nil {
			_ = connection.Close()
			continue
		}
	}
}

func (consumer *IncidentConsumer) consumeOnce(ctx context.Context) error {
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
			if err := json.Unmarshal(delivery.Body, &event); err != nil || !validIncidentEvent(event) {
				if publishErr := consumer.publishInvalid(ctx, delivery.Body); publishErr != nil {
					return publishErr
				}
				if ackErr := delivery.Ack(false); ackErr != nil {
					return ackErr
				}
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

func (consumer *IncidentConsumer) declare() error {
	channel, err := consumer.connection.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = channel.Close() }()
	// Keep the declaration argument-free. Older deployments created this
	// durable queue without DLX arguments; changing them would fail with 406
	// and prevent startup. Invalid events are explicitly republished to DLQ.
	_, err = channel.QueueDeclare(contractsmessaging.IncidentEventsQueue, true, false, false, false, nil)
	if err != nil {
		return err
	}
	_, err = channel.QueueDeclare("incidents.events.dlq", true, false, false, false, nil)
	return err
}

func validIncidentEvent(event contracts.Event) bool {
	return event.ID != "" && event.ServiceID != "" && !event.OccurredAt.IsZero() && (event.Type == contracts.Started || event.Type == contracts.Resolved)
}

func (consumer *IncidentConsumer) publishInvalid(ctx context.Context, body []byte) error {
	channel, err := consumer.connection.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = channel.Close() }()
	if err := channel.Confirm(false); err != nil {
		return err
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	returns := channel.NotifyReturn(make(chan amqp.Return, 1))
	if err := channel.PublishWithContext(ctx, "", "incidents.events.dlq", true, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body}); err != nil {
		return err
	}
	select {
	case returned := <-returns:
		return fmt.Errorf("invalid incident event was not routed: %s", returned.ReplyText)
	default:
	}
	select {
	case returned := <-returns:
		return fmt.Errorf("invalid incident event was not routed: %s", returned.ReplyText)
	case confirmation, ok := <-confirms:
		if !ok || !confirmation.Ack {
			return fmt.Errorf("invalid incident event publication was not confirmed")
		}
		select {
		case returned := <-returns:
			return fmt.Errorf("invalid incident event was not routed: %s", returned.ReplyText)
		default:
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
