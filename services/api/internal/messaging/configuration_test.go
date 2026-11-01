package messaging

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
	amqp "github.com/rabbitmq/amqp091-go"
)

type stubConfigurationRepository struct{}

func (stubConfigurationRepository) List(context.Context) ([]serviceconfig.Service, error) {
	return nil, nil
}

type servicesConfigurationRepository struct {
	services []serviceconfig.Service
}

func (repository servicesConfigurationRepository) List(context.Context) ([]serviceconfig.Service, error) {
	return repository.services, nil
}

type stubChannel struct {
	mu            sync.Mutex
	queueDeclares int
	published     []amqp.Publishing
}

func (channel *stubChannel) Close() error {
	return nil
}

func (channel *stubChannel) PublishWithContext(_ context.Context, _ string, _ string, _ bool, _ bool, message amqp.Publishing) error {
	channel.mu.Lock()
	defer channel.mu.Unlock()
	channel.published = append(channel.published, message)
	return nil
}

func (channel *stubChannel) QueueDeclare(_ string, _ bool, _ bool, _ bool, _ bool, _ amqp.Table) (amqp.Queue, error) {
	channel.mu.Lock()
	defer channel.mu.Unlock()
	channel.queueDeclares++
	return amqp.Queue{}, nil
}

type stubConnection struct {
	mu       sync.Mutex
	closed   bool
	channels []*stubChannel
}

func (connection *stubConnection) Channel() (rabbitChannel, error) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	if connection.closed {
		return nil, errors.New("connection is closed")
	}
	channel := &stubChannel{}
	connection.channels = append(connection.channels, channel)
	return channel, nil
}

func (connection *stubConnection) Close() error {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	connection.closed = true
	return nil
}

func (connection *stubConnection) IsClosed() bool {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	return connection.closed
}

func (connection *stubConnection) publishCount() int {
	connection.mu.Lock()
	channels := append([]*stubChannel(nil), connection.channels...)
	connection.mu.Unlock()

	count := 0
	for _, channel := range channels {
		channel.mu.Lock()
		count += len(channel.published)
		channel.mu.Unlock()
	}
	return count
}

func (connection *stubConnection) queueDeclareCount() int {
	connection.mu.Lock()
	channels := append([]*stubChannel(nil), connection.channels...)
	connection.mu.Unlock()

	count := 0
	for _, channel := range channels {
		channel.mu.Lock()
		count += channel.queueDeclares
		channel.mu.Unlock()
	}
	return count
}

type stubDialer struct {
	mu          sync.Mutex
	connections []rabbitConnection
	err         error
	calls       int
}

func (dialer *stubDialer) dial(_ string) (rabbitConnection, error) {
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	dialer.calls++
	if dialer.err != nil {
		return nil, dialer.err
	}
	if len(dialer.connections) == 0 {
		return nil, errors.New("no test connection")
	}
	connection := dialer.connections[0]
	dialer.connections = dialer.connections[1:]
	return connection, nil
}

func (dialer *stubDialer) callCount() int {
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	return dialer.calls
}

func newTestBroker(t *testing.T, dialer *stubDialer) *ConfigurationBroker {
	t.Helper()
	broker := &ConfigurationBroker{
		rabbitMQURL: "amqp://test",
		repository:  stubConfigurationRepository{},
		dial:        dialer.dial,
	}
	if err := broker.connect(); err != nil {
		t.Fatalf("connect broker: %v", err)
	}
	return broker
}

func testUpdate() serviceconfig.Update {
	return serviceconfig.Update{Event: "service.updated", Service: serviceconfig.Service{ID: "service-1"}}
}

func TestConfigurationBrokerPublishesUpdate(t *testing.T) {
	connection := &stubConnection{}
	broker := newTestBroker(t, &stubDialer{connections: []rabbitConnection{connection}})

	if err := broker.Publish(context.Background(), testUpdate()); err != nil {
		t.Fatalf("publish update: %v", err)
	}
	if connection.publishCount() != 1 {
		t.Fatalf("published updates = %d, want 1", connection.publishCount())
	}
}

func TestConfigurationBrokerReconnectsAfterConnectionClose(t *testing.T) {
	initial := &stubConnection{}
	recovered := &stubConnection{}
	dialer := &stubDialer{connections: []rabbitConnection{initial, recovered}}
	broker := newTestBroker(t, dialer)
	if err := initial.Close(); err != nil {
		t.Fatalf("close initial connection: %v", err)
	}

	if err := broker.Publish(context.Background(), testUpdate()); err != nil {
		t.Fatalf("publish after reconnect: %v", err)
	}
	if recovered.publishCount() != 1 {
		t.Fatalf("published updates on recovered connection = %d, want 1", recovered.publishCount())
	}
	if recovered.queueDeclareCount() != 1 {
		t.Fatalf("queue declarations on recovered connection = %d, want 1", recovered.queueDeclareCount())
	}
	if dialer.callCount() != 2 {
		t.Fatalf("dial attempts = %d, want 2", dialer.callCount())
	}
}

func TestConfigurationBrokerReconnectsBeforePublishingCurrentConfiguration(t *testing.T) {
	initial := &stubConnection{}
	recovered := &stubConnection{}
	dialer := &stubDialer{connections: []rabbitConnection{initial, recovered}}
	broker := newTestBroker(t, dialer)
	broker.repository = servicesConfigurationRepository{services: []serviceconfig.Service{{ID: "service-1"}}}
	if err := initial.Close(); err != nil {
		t.Fatalf("close initial connection: %v", err)
	}

	if err := broker.PublishCurrentConfiguration(context.Background()); err != nil {
		t.Fatalf("publish current configuration after reconnect: %v", err)
	}
	if recovered.publishCount() != 1 {
		t.Fatalf("published updates on recovered connection = %d, want 1", recovered.publishCount())
	}
	if dialer.callCount() != 2 {
		t.Fatalf("dial attempts = %d, want 2", dialer.callCount())
	}
}

func TestConfigurationBrokerReturnsErrorWhenRabbitMQIsUnavailable(t *testing.T) {
	connection := &stubConnection{}
	dialer := &stubDialer{connections: []rabbitConnection{connection}}
	broker := newTestBroker(t, dialer)
	if err := connection.Close(); err != nil {
		t.Fatalf("close connection: %v", err)
	}
	dialer.err = errors.New("RabbitMQ unavailable")

	if err := broker.Publish(context.Background(), testUpdate()); err == nil {
		t.Fatal("publish error = nil, want RabbitMQ error")
	}
	if connection.publishCount() != 0 {
		t.Fatalf("published updates = %d, want 0", connection.publishCount())
	}
}

func TestConfigurationBrokerReconnectsSafelyForConcurrentPublishes(t *testing.T) {
	initial := &stubConnection{}
	recovered := &stubConnection{}
	dialer := &stubDialer{connections: []rabbitConnection{initial, recovered}}
	broker := newTestBroker(t, dialer)
	if err := initial.Close(); err != nil {
		t.Fatalf("close initial connection: %v", err)
	}

	const publishers = 20
	errors := make(chan error, publishers)
	var group sync.WaitGroup
	for range publishers {
		group.Add(1)
		go func() {
			defer group.Done()
			errors <- broker.Publish(context.Background(), testUpdate())
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent publish: %v", err)
		}
	}
	if recovered.publishCount() != publishers {
		t.Fatalf("published updates = %d, want %d", recovered.publishCount(), publishers)
	}
	if dialer.callCount() != 2 {
		t.Fatalf("dial attempts = %d, want 2", dialer.callCount())
	}
}

func TestConfigurationBrokerCloseIsSafe(t *testing.T) {
	connection := &stubConnection{}
	broker := newTestBroker(t, &stubDialer{connections: []rabbitConnection{connection}})

	if err := broker.Close(); err != nil {
		t.Fatalf("close broker: %v", err)
	}
	if err := broker.Close(); err != nil {
		t.Fatalf("close broker twice: %v", err)
	}
	if err := broker.Publish(context.Background(), testUpdate()); err == nil {
		t.Fatal("publish error = nil after broker close")
	}
	if err := broker.PublishCurrentConfiguration(context.Background()); err == nil {
		t.Fatal("publish current configuration error = nil after broker close")
	}
}
