package main

import (
	"context"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func serviceName() string { return "Notification Service" }

func main() {
	config := loadSMTPConfig()
	database, err := gorm.Open(postgres.Open(value("POSTGRES_DSN", "postgres://monitoring:monitoring@localhost:5432/monitoring?sslmode=disable")), &gorm.Config{})
	if err != nil {
		log.Fatalf("open PostgreSQL: %v", err)
	}
	for {
		connection, err := amqp.Dial(value("RABBITMQ_URL", "amqp://monitoring:monitoring@localhost:5672/"))
		if err == nil {
			err = consume(context.Background(), connection, database, config)
			if closeErr := connection.Close(); closeErr != nil {
				log.Printf("close notification RabbitMQ connection: %v", closeErr)
			}
		}
		log.Printf("notification consumer reconnect: %v", err)
		time.Sleep(time.Second)
	}
}
