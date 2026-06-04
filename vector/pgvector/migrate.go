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

	m002 := goose.NewGoMigration(
		2,
		&goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				ALTER TABLE documents
				ADD COLUMN IF NOT EXISTS tsv tsvector
				GENERATED ALWAYS AS (to_tsvector('english', content)) STORED`)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS documents_tsv_idx
				ON documents USING GIN(tsv)`)
			return err
		}},
		&goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `DROP INDEX IF EXISTS documents_tsv_idx`); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `ALTER TABLE documents DROP COLUMN IF EXISTS tsv`)
			return err
		}},
	)

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		nil,
		goose.WithGoMigrations(m001, m002),
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
