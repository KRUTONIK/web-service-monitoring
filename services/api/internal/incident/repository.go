package incident

import (
	"context"
	"errors"
	"fmt"
	"time"

	contracts "github.com/KRUTONIK/web-service-monitoring/contracts/incidents"
	"gorm.io/gorm"
)

type Incident struct {
	ID        string     `json:"id"`
	ServiceID string     `json:"service_id"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

type record struct {
	ID        string `gorm:"default:gen_random_uuid()::text"`
	EventID   string
	ServiceID string
	StartedAt time.Time
	EndedAt   *time.Time
}

type eventRecord struct {
	EventID    string
	ServiceID  string
	EventType  string
	OccurredAt time.Time
}

func (eventRecord) TableName() string { return "incident_events" }

func (record) TableName() string { return "incidents" }

type Repository struct{ database *gorm.DB }

func New(database *gorm.DB) *Repository { return &Repository{database: database} }

func (repository *Repository) Process(ctx context.Context, event contracts.Event) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		// Serialize transitions for one service; a row exists even for soft-deleted services.
		if err := transaction.Exec("SELECT id FROM monitored_services WHERE id = ? FOR UPDATE", event.ServiceID).Error; err != nil {
			return fmt.Errorf("lock service: %w", err)
		}
		var existing eventRecord
		if err := transaction.Where("event_id = ?", event.ID).First(&existing).Error; err == nil {
			if existing.ServiceID != event.ServiceID {
				return fmt.Errorf("event %s belongs to another service", event.ID)
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find incident event: %w", err)
		}
		entry := eventRecord{EventID: event.ID, ServiceID: event.ServiceID, EventType: event.Type, OccurredAt: event.OccurredAt}
		if err := transaction.Create(&entry).Error; err != nil {
			return fmt.Errorf("register incident event: %w", err)
		}
		var newest eventRecord
		if err := transaction.Where("service_id = ? AND event_id <> ?", event.ServiceID, event.ID).Order("occurred_at DESC").First(&newest).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find newest incident event: %w", err)
		}
		if !newest.OccurredAt.IsZero() && newest.OccurredAt.After(event.OccurredAt) {
			return nil
		}
		if event.Type == contracts.Started {
			if err := transaction.Exec("INSERT INTO incidents (event_id, service_id, started_at) SELECT ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM incidents WHERE service_id = ? AND ended_at IS NULL)", event.ID, event.ServiceID, event.OccurredAt, event.ServiceID).Error; err != nil {
				return fmt.Errorf("create incident: %w", err)
			}
		}
		if event.Type == contracts.Resolved {
			if err := transaction.Model(&record{}).Where("service_id = ? AND ended_at IS NULL AND started_at <= ?", event.ServiceID, event.OccurredAt).Update("ended_at", event.OccurredAt).Error; err != nil {
				return fmt.Errorf("close incident: %w", err)
			}
		}
		return nil
	})
}

func (repository *Repository) ListByService(ctx context.Context, serviceID string) ([]Incident, error) {
	var records []record
	if err := repository.database.WithContext(ctx).Where("service_id = ?", serviceID).Order("started_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	incidents := make([]Incident, len(records))
	for index, value := range records {
		incidents[index] = Incident{ID: value.ID, ServiceID: value.ServiceID, StartedAt: value.StartedAt, EndedAt: value.EndedAt}
	}
	return incidents, nil
}

func (repository *Repository) ListPageByService(ctx context.Context, serviceID string, limit, offset int) ([]Incident, int, error) {
	database := repository.database.WithContext(ctx).Where("service_id = ?", serviceID)
	var total int64
	if err := database.Model(&record{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count incidents: %w", err)
	}
	var records []record
	if err := database.Order("started_at DESC, id").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list incidents page: %w", err)
	}
	incidents := make([]Incident, len(records))
	for index, value := range records {
		incidents[index] = Incident{ID: value.ID, ServiceID: value.ServiceID, StartedAt: value.StartedAt, EndedAt: value.EndedAt}
	}
	return incidents, int(total), nil
}
