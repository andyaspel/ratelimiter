package ratelimiter

import (
	"context"
	"database/sql"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const defaultSQLiteDBPath = "ratelimiter.db"

// StoredFile represents a file saved in the SQLite-backed demo store.
type StoredFile struct {
	ID          int64
	Name        string
	ContentType string
	Size        int64
	Data        []byte
	CreatedAt   time.Time
}

// SQLiteStore persists uploaded or CLI-supplied files in a local SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLiteStore opens or creates a SQLite database and ensures the schema exists.
func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultSQLiteDBPath
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db}
	if err := store.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

// Close closes the underlying database handle.
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// SaveFile stores an in-memory file payload in SQLite.
func (s *SQLiteStore) SaveFile(ctx context.Context, name string, contentType string, data []byte) (StoredFile, error) {
	if s == nil || s.db == nil {
		return StoredFile{}, ErrNilSQLiteStore
	}
	if strings.TrimSpace(name) == "" {
		return StoredFile{}, ErrEmptyFileName
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO stored_files (name, content_type, size, data, created_at) VALUES (?, ?, ?, ?, ?)`,
		strings.TrimSpace(name),
		strings.TrimSpace(contentType),
		len(data),
		data,
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return StoredFile{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return StoredFile{}, err
	}

	return StoredFile{
		ID:          id,
		Name:        strings.TrimSpace(name),
		ContentType: strings.TrimSpace(contentType),
		Size:        int64(len(data)),
		Data:        append([]byte(nil), data...),
		CreatedAt:   now,
	}, nil
}

// SaveFileFromPath reads a file from disk and stores it in SQLite.
func (s *SQLiteStore) SaveFileFromPath(ctx context.Context, path string) (StoredFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StoredFile{}, err
	}

	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	return s.SaveFile(ctx, filepath.Base(path), contentType, data)
}

// ListFiles returns saved file metadata without loading full BLOB contents.
func (s *SQLiteStore) ListFiles(ctx context.Context) ([]StoredFile, error) {
	if s == nil || s.db == nil {
		return nil, ErrNilSQLiteStore
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, name, content_type, size, created_at FROM stored_files ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]StoredFile, 0)
	for rows.Next() {
		var file StoredFile
		var createdAt string
		if err := rows.Scan(&file.ID, &file.Name, &file.ContentType, &file.Size, &createdAt); err != nil {
			return nil, err
		}
		file.CreatedAt, err = parseStoredFileTime(createdAt)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

// GetFile returns a single stored file including its BLOB content.
func (s *SQLiteStore) GetFile(ctx context.Context, id int64) (StoredFile, error) {
	if s == nil || s.db == nil {
		return StoredFile{}, ErrNilSQLiteStore
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var file StoredFile
	var createdAt string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, content_type, size, data, created_at FROM stored_files WHERE id = ?`,
		id,
	).Scan(&file.ID, &file.Name, &file.ContentType, &file.Size, &file.Data, &createdAt)
	if err != nil {
		return StoredFile{}, err
	}
	file.CreatedAt, err = parseStoredFileTime(createdAt)
	if err != nil {
		return StoredFile{}, err
	}
	return file, nil
}

func parseStoredFileTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %q", ErrInvalidStoredTimestamp, value)
	}
	return parsed, nil
}

func (s *SQLiteStore) ensureSchema(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS stored_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			content_type TEXT NOT NULL,
			size INTEGER NOT NULL,
			data BLOB NOT NULL,
			created_at TEXT NOT NULL
		)
	`)
	return err
}
