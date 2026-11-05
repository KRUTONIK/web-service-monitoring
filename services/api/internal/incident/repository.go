package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	contracts "github.com/KRUTONIK/web-service-monitoring/contracts/incidents"
	"github.com/KRUTONIK/web-service-monitoring/contracts/notifications"
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

type serviceRecipient struct {
	ID                   string
	Name                 string
	URL                  string
	Email                string
	Enabled              bool
	NotificationsEnabled bool
	DeletedAt            *time.Time
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
		var startedAt time.Time
		changed := false
		if event.Type == contracts.Started {
			result := transaction.Exec("INSERT INTO incidents (event_id, service_id, started_at) SELECT ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM incidents WHERE service_id = ? AND ended_at IS NULL)", event.ID, event.ServiceID, event.OccurredAt, event.ServiceID)
			if result.Error != nil {
				return fmt.Errorf("create incident: %w", result.Error)
			}
			changed = result.RowsAffected == 1
			startedAt = event.OccurredAt
		}
		if event.Type == contracts.Resolved {
			var open record
			if err := transaction.Where("service_id = ? AND ended_at IS NULL AND started_at <= ?", event.ServiceID, event.OccurredAt).First(&open).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return fmt.Errorf("find open incident: %w", err)
			}
			if err := transaction.Model(&record{}).Where("id = ?", open.ID).Update("ended_at", event.OccurredAt).Error; err != nil {
				return fmt.Errorf("close incident: %w", err)
			}
			changed = true
			startedAt = open.StartedAt
		}
		if changed {
			if err := repository.enqueueNotification(transaction, event, startedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository *Repository) enqueueNotification(transaction *gorm.DB, event contracts.Event, startedAt time.Time) error {
	var recipient serviceRecipient
	if err := transaction.Raw(`SELECT monitored_services.id, monitored_services.name, monitored_services.url, users.email,
 monitored_services.enabled, monitored_services.notifications_enabled, monitored_services.deleted_at
 FROM monitored_services JOIN users ON users.id = monitored_services.owner_id WHERE monitored_services.id = ?`, event.ServiceID).Scan(&recipient).Error; err != nil {
		return fmt.Errorf("find notification recipient: %w", err)
	}
	if recipient.ID == "" || recipient.Email == "" || !recipient.Enabled || !recipient.NotificationsEnabled || recipient.DeletedAt != nil {
		return nil
	}
	payload, err := json.Marshal(notifications.Event{ID: event.ID, Type: event.Type, ServiceID: event.ServiceID, ServiceName: recipient.Name, ServiceURL: recipient.URL, Recipient: recipient.Email, OccurredAt: event.OccurredAt, StartedAt: startedAt, StatusCode: event.StatusCode, ResponseTimeMS: event.ResponseTimeMS, Error: event.Error})
	if err != nil {
		return fmt.Errorf("encode notification: %w", err)
	}
	if err := transaction.Exec(`INSERT INTO notification_outbox (event_id, event_type, service_id, payload) VALUES (?, ?, ?, ?::jsonb) ON CONFLICT (event_id) DO NOTHING`, event.ID, event.Type, event.ServiceID, string(payload)).Error; err != nil {
		return fmt.Errorf("create notification outbox entry: %w", err)
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
