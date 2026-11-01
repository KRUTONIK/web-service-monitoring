package serviceconfig

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const prototypeServiceID = "prototype-service"

type Repository struct {
	database *gorm.DB
}

func New(database *gorm.DB) *Repository {
	return &Repository{database: database}
}

type record struct {
	ID        string
	URL       string
	Enabled   bool
	Version   int64
	UpdatedAt time.Time
}

func (record) TableName() string { return "monitored_services" }

func (repository *Repository) Initialize(ctx context.Context, serviceURL string) error {
	prototype := record{ID: prototypeServiceID, URL: serviceURL, Enabled: true, Version: 1, UpdatedAt: time.Now().UTC()}
	if err := repository.database.WithContext(ctx).Where("id = ?", prototypeServiceID).FirstOrCreate(&prototype).Error; err != nil {
		return fmt.Errorf("seed prototype service: %w", err)
	}

	return nil
}

func (repository *Repository) List(ctx context.Context) ([]Service, error) {
	var records []record
	if err := repository.database.WithContext(ctx).Order("id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query monitored services: %w", err)
	}

	services := make([]Service, 0, len(records))
	for _, record := range records {
		services = append(services, Service(record))
	}
	return services, nil
}
