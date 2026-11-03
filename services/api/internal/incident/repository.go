package incident

import (
	"context"
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

func (record) TableName() string { return "incidents" }

type Repository struct{ database *gorm.DB }

func New(database *gorm.DB) *Repository { return &Repository{database: database} }

func (repository *Repository) Process(ctx context.Context, event contracts.Event) error {
	if event.Type == contracts.Started {
		record := record{EventID: event.ID, ServiceID: event.ServiceID, StartedAt: event.OccurredAt}
		if err := repository.database.WithContext(ctx).Create(&record).Error; err != nil {
			// The unique event and open-incident constraints make redelivery safe.
			if err == gorm.ErrDuplicatedKey {
				return nil
			}
			return fmt.Errorf("create incident: %w", err)
		}
		return nil
	}
	if event.Type == contracts.Resolved {
		if err := repository.database.WithContext(ctx).Model(&record{}).Where("service_id = ? AND ended_at IS NULL", event.ServiceID).Update("ended_at", event.OccurredAt).Error; err != nil {
			return fmt.Errorf("close incident: %w", err)
		}
	}
	return nil
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
