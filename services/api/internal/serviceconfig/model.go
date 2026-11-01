package serviceconfig

import "time"

type Service struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"-"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Enabled   bool      `json:"enabled"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Snapshot struct {
	Services []Service `json:"services"`
}

type Update struct {
	Event   string  `json:"event"`
	Service Service `json:"service"`
}
