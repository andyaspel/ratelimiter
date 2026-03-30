package ratelimiter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLiteStore_SaveListAndGetFile(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "files.db"))
	if err != nil {
		t.Fatalf("expected SQLite store to open: %v", err)
	}
	defer store.Close()

	saved, err := store.SaveFile(context.Background(), "hello.txt", "text/plain", []byte("hello world"))
	if err != nil {
		t.Fatalf("expected file to save: %v", err)
	}
	if saved.ID == 0 {
		t.Fatalf("expected inserted file to have an ID")
	}

	files, err := store.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("expected file list to succeed: %v", err)
	}
	if len(files) != 1 || files[0].Name != "hello.txt" {
		t.Fatalf("unexpected file metadata: %+v", files)
	}

	got, err := store.GetFile(context.Background(), saved.ID)
	if err != nil {
		t.Fatalf("expected stored file lookup to succeed: %v", err)
	}
	if string(got.Data) != "hello world" {
		t.Fatalf("unexpected stored file content: %q", string(got.Data))
	}
}

func TestSQLiteStore_SaveFileFromPath(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(inputPath, []byte("sample data"), 0o644); err != nil {
		t.Fatalf("expected test file to be created: %v", err)
	}

	store, err := OpenSQLiteStore(filepath.Join(dir, "files.db"))
	if err != nil {
		t.Fatalf("expected SQLite store to open: %v", err)
	}
	defer store.Close()

	saved, err := store.SaveFileFromPath(context.Background(), inputPath)
	if err != nil {
		t.Fatalf("expected file path save to succeed: %v", err)
	}
	if saved.Name != "sample.txt" {
		t.Fatalf("expected file name to be preserved, got %q", saved.Name)
	}
	if !strings.HasPrefix(saved.ContentType, "text/plain") {
		t.Fatalf("expected text content type to be detected, got %q", saved.ContentType)
	}
}

func TestSQLiteStore_ListFilesRejectsInvalidTimestamp(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "files.db"))
	if err != nil {
		t.Fatalf("expected SQLite store to open: %v", err)
	}
	defer store.Close()

	_, err = store.db.ExecContext(
		context.Background(),
		`INSERT INTO stored_files (name, content_type, size, data, created_at) VALUES (?, ?, ?, ?, ?)`,
		"broken.txt",
		"text/plain",
		1,
		[]byte("x"),
		"not-a-time",
	)
	if err != nil {
		t.Fatalf("expected invalid row insert to succeed for test setup: %v", err)
	}

	_, err = store.ListFiles(context.Background())
	if !errors.Is(err, ErrInvalidStoredTimestamp) {
		t.Fatalf("expected ErrInvalidStoredTimestamp, got %v", err)
	}
}
