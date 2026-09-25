package serviceconfig

import (
	"sync"
	"time"
)

type Service struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Enabled   bool      `json:"enabled"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Snapshot struct {
	Services []Service `json:"services"`
}

type Update struct {
	Event   string  `json:"event"`
	Service Service `json:"service"`
}

type State struct {
	mutex    sync.RWMutex
	services map[string]Service
}

func NewState() *State {
	return &State{services: make(map[string]Service)}
}

func (state *State) Replace(snapshot Snapshot) {
	state.mutex.Lock()
	defer state.mutex.Unlock()

	state.services = make(map[string]Service, len(snapshot.Services))
	for _, service := range snapshot.Services {
		state.services[service.ID] = service
	}
}

func (state *State) Apply(service Service) bool {
	state.mutex.Lock()
	defer state.mutex.Unlock()

	current, exists := state.services[service.ID]
	if exists && service.Version <= current.Version {
		return false
	}

	state.services[service.ID] = service
	return true
}

func (state *State) Services() []Service {
	state.mutex.RLock()
	defer state.mutex.RUnlock()

	services := make([]Service, 0, len(state.services))
	for _, service := range state.services {
		services = append(services, service)
	}
	return services
}
