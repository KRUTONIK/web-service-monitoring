package serviceconfig

import "time"

type Service struct {
	ID                   string     `json:"id"`
	OwnerID              string     `json:"-"`
	Name                 string     `json:"name"`
	URL                  string     `json:"url"`
	Enabled              bool       `json:"enabled"`
	NotificationsEnabled bool       `json:"notifications_enabled"`
	IntervalSeconds      int        `json:"interval_seconds"`
	TimeoutSeconds       int        `json:"timeout_seconds"`
	Version              int64      `json:"version"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	DeletedAt            *time.Time `json:"-"`
}

type Snapshot struct {
	Services []Service `json:"services"`
}

type Update struct {
	Event   string  `json:"event"`
	Service Service `json:"service"`
}
