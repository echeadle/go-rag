package pgvector

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

// runMigrations applies all pending migrations to db.
// Migration 001 creates the documents table using embeddingDim so the
// vector column matches whatever model the operator configures.
func runMigrations(ctx context.Context, db *sql.DB, embeddingDim int) error {
	m001 := goose.NewGoMigration(
		1,
		&goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS documents (
					id          TEXT        PRIMARY KEY,
					content     TEXT        NOT NULL,
					metadata    JSONB       NOT NULL DEFAULT '{}'::jsonb,
					embedding   vector(%d)  NOT NULL,
					created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
				)`, embeddingDim))
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS documents_embedding_idx
					ON documents USING hnsw (embedding vector_cosine_ops)`)
			return err
		}},
		&goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS documents`)
			return err
		}},
	)

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		nil, // SQL migrations in db/migrations/ will be added in future items
		goose.WithGoMigrations(m001),
	)
	if err != nil {
		return fmt.Errorf("migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}

	for _, r := range results {
		if r.Error != nil {
			return fmt.Errorf("migration %d: %w", r.Source.Version, r.Error)
		}
	}
	return nil
}
