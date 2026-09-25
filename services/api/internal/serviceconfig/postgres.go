package serviceconfig

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const prototypeServiceID = "prototype-service"

type Repository struct {
	database *sql.DB
}

func Open(ctx context.Context, dsn string) (*Repository, error) {
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	return &Repository{database: database}, nil
}

func (repository *Repository) Close() error {
	return repository.database.Close()
}

func (repository *Repository) Initialize(ctx context.Context, serviceURL string) error {
	_, err := repository.database.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS monitored_services (
            id TEXT PRIMARY KEY,
            url TEXT NOT NULL,
            enabled BOOLEAN NOT NULL,
            version BIGINT NOT NULL,
            updated_at TIMESTAMPTZ NOT NULL
        )
    `)
	if err != nil {
		return fmt.Errorf("create monitored services table: %w", err)
	}

	_, err = repository.database.ExecContext(ctx, `
        INSERT INTO monitored_services (id, url, enabled, version, updated_at)
        VALUES ($1, $2, TRUE, 1, NOW())
        ON CONFLICT (id) DO NOTHING
    `, prototypeServiceID, serviceURL)
	if err != nil {
		return fmt.Errorf("seed prototype service: %w", err)
	}

	return nil
}

func (repository *Repository) List(ctx context.Context) ([]Service, error) {
	rows, err := repository.database.QueryContext(ctx, `
        SELECT id, url, enabled, version, updated_at
        FROM monitored_services
        ORDER BY id
    `)
	if err != nil {
		return nil, fmt.Errorf("query monitored services: %w", err)
	}
	defer func() { _ = rows.Close() }()

	services := make([]Service, 0)
	for rows.Next() {
		var service Service
		if err := rows.Scan(
			&service.ID,
			&service.URL,
			&service.Enabled,
			&service.Version,
			&service.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan monitored service: %w", err)
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monitored services: %w", err)
	}

	return services, nil
}
