package serviceconfig

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("monitored service not found")

const prototypeServiceID = "prototype-service"

type Repository struct {
	database *gorm.DB
}

type record struct {
	ID        string `gorm:"default:gen_random_uuid()::text"`
	OwnerID   *string
	Name      string
	URL       string
	Enabled   bool
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (record) TableName() string { return "monitored_services" }

func New(database *gorm.DB) *Repository {
	return &Repository{database: database}
}

func (repository *Repository) Initialize(ctx context.Context, serviceURL string) error {
	now := time.Now().UTC()
	prototype := record{ID: prototypeServiceID, Name: "Демонстрационный сервис", URL: serviceURL, Enabled: true, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := repository.database.WithContext(ctx).Where("id = ?", prototypeServiceID).FirstOrCreate(&prototype).Error; err != nil {
		return fmt.Errorf("seed prototype service: %w", err)
	}

	return nil
}

func (repository *Repository) List(ctx context.Context) ([]Service, error) {
	var records []record
	if err := repository.database.WithContext(ctx).Where("deleted_at IS NULL").Order("id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query monitored services: %w", err)
	}

	return toServices(records), nil
}

func (repository *Repository) ListByOwner(ctx context.Context, ownerID string) ([]Service, error) {
	var records []record
	if err := repository.database.WithContext(ctx).Where("owner_id = ? AND deleted_at IS NULL", ownerID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query user monitored services: %w", err)
	}

	return toServices(records), nil
}

func (repository *Repository) ByIDAndOwner(ctx context.Context, ownerID, id string) (Service, error) {
	var record record
	database := repository.database.WithContext(ctx).Where("id = ? AND owner_id = ? AND deleted_at IS NULL", id, ownerID).First(&record)
	if errors.Is(database.Error, gorm.ErrRecordNotFound) {
		return Service{}, ErrNotFound
	}
	if database.Error != nil {
		return Service{}, fmt.Errorf("find monitored service: %w", database.Error)
	}

	return toService(record), nil
}

func (repository *Repository) Create(ctx context.Context, ownerID, name, url string, enabled bool) (Service, error) {
	now := time.Now().UTC()
	record := record{OwnerID: &ownerID, Name: name, URL: url, Enabled: enabled, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := repository.database.WithContext(ctx).Create(&record).Error; err != nil {
		return Service{}, fmt.Errorf("create monitored service: %w", err)
	}

	return toService(record), nil
}

func (repository *Repository) Update(ctx context.Context, ownerID, id, name, url string, enabled bool) (Service, error) {
	var record record
	database := repository.database.WithContext(ctx).Where("id = ? AND owner_id = ? AND deleted_at IS NULL", id, ownerID).First(&record)
	if errors.Is(database.Error, gorm.ErrRecordNotFound) {
		return Service{}, ErrNotFound
	}
	if database.Error != nil {
		return Service{}, fmt.Errorf("find monitored service: %w", database.Error)
	}

	record.Name = name
	record.URL = url
	record.Enabled = enabled
	record.Version++
	record.UpdatedAt = time.Now().UTC()
	if err := repository.database.WithContext(ctx).Save(&record).Error; err != nil {
		return Service{}, fmt.Errorf("update monitored service: %w", err)
	}

	return toService(record), nil
}

func (repository *Repository) Delete(ctx context.Context, ownerID, id string) (Service, error) {
	var record record
	database := repository.database.WithContext(ctx).Where("id = ? AND owner_id = ? AND deleted_at IS NULL", id, ownerID).First(&record)
	if errors.Is(database.Error, gorm.ErrRecordNotFound) {
		return Service{}, ErrNotFound
	}
	if database.Error != nil {
		return Service{}, fmt.Errorf("find monitored service: %w", database.Error)
	}

	now := time.Now().UTC()
	record.Enabled = false
	record.Version++
	record.UpdatedAt = now
	record.DeletedAt = &now
	if err := repository.database.WithContext(ctx).Save(&record).Error; err != nil {
		return Service{}, fmt.Errorf("soft delete monitored service: %w", err)
	}

	return toService(record), nil
}

func toServices(records []record) []Service {
	services := make([]Service, 0, len(records))
	for _, record := range records {
		services = append(services, toService(record))
	}

	return services
}

func toService(record record) Service {
	ownerID := ""
	if record.OwnerID != nil {
		ownerID = *record.OwnerID
	}

	return Service{ID: record.ID, OwnerID: ownerID, Name: record.Name, URL: record.URL, Enabled: record.Enabled, Version: record.Version, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, DeletedAt: record.DeletedAt}
}
