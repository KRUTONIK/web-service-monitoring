package serviceconfig

import (
	"testing"
	"time"
)

func TestToServicePreservesManagedServiceFields(t *testing.T) {
	ownerID := "user-1"
	createdAt := time.Date(2026, time.November, 1, 9, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	service := toService(record{ID: "service-1", OwnerID: &ownerID, Name: "Основной сайт", URL: "https://example.test", Enabled: true, Version: 2, CreatedAt: createdAt, UpdatedAt: updatedAt})

	if service.ID != "service-1" || service.OwnerID != ownerID || service.Name != "Основной сайт" {
		t.Fatalf("unexpected service identity: %+v", service)
	}
	if service.URL != "https://example.test" || !service.Enabled || service.Version != 2 {
		t.Fatalf("unexpected service configuration: %+v", service)
	}
	if !service.CreatedAt.Equal(createdAt) || !service.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected timestamps: %+v", service)
	}
}

func TestToServiceSupportsPrototypeWithoutOwner(t *testing.T) {
	service := toService(record{ID: prototypeServiceID})
	if service.OwnerID != "" {
		t.Fatalf("unexpected prototype owner: %q", service.OwnerID)
	}
}

func TestToServicePreservesSoftDeletion(t *testing.T) {
	deletedAt := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)
	service := toService(record{ID: "service-1", Enabled: false, Version: 3, DeletedAt: &deletedAt})

	if service.Enabled || service.DeletedAt == nil || !service.DeletedAt.Equal(deletedAt) || service.Version != 3 {
		t.Fatalf("unexpected deleted service: %+v", service)
	}
}
