package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(ctx context.Context, dsn string) (*gorm.DB, error) {
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}

	sqlDatabase, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("get PostgreSQL connection: %w", err)
	}
	if err := sqlDatabase.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	if err := applyMigrations(ctx, sqlDatabase); err != nil {
		return nil, err
	}

	return database, nil
}

func Close(database *gorm.DB) error {
	sqlDatabase, err := database.DB()
	if err != nil {
		return err
	}
	return sqlDatabase.Close()
}

func applyMigrations(ctx context.Context, database *sql.DB) error {
	goose.SetBaseFS(migrationFiles)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set PostgreSQL migration dialect: %w", err)
	}
	if err := goose.UpContext(ctx, database, "migrations"); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}
	return nil
}
