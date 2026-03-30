package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	ratelimiter "github.com/andyaspel/ratelimiter"
)

const maxUploadBytes = 10 << 20 // 10 MiB

var homeTemplate = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>ratelimiter demo</title>
  <style>
    body { font-family: sans-serif; max-width: 900px; margin: 2rem auto; padding: 0 1rem; }
    table { border-collapse: collapse; width: 100%; }
    th, td { border: 1px solid #ddd; padding: 0.5rem; text-align: left; }
    th { background: #f5f5f5; }
  </style>
</head>
<body>
  <h1>ratelimiter demo</h1>
  <p>Version: <strong>{{.Version}}</strong></p>
  <p>SQLite database: <code>{{.DBPath}}</code></p>

  <h2>Upload a file</h2>
  <form action="/upload" method="post" enctype="multipart/form-data">
    <input type="file" name="file" required>
    <button type="submit">Save to SQLite</button>
  </form>

  <h2>Saved files</h2>
  <table>
    <thead>
      <tr><th>ID</th><th>Name</th><th>Size</th><th>Saved</th><th>Download</th></tr>
    </thead>
    <tbody>
      {{range .Files}}
      <tr>
        <td>{{.ID}}</td>
        <td>{{.Name}}</td>
        <td>{{.Size}}</td>
        <td>{{.CreatedAt}}</td>
        <td><a href="/download?id={{.ID}}">download</a></td>
      </tr>
      {{else}}
      <tr><td colspan="5">No files saved yet.</td></tr>
      {{end}}
    </tbody>
  </table>
</body>
</html>`))

type demoApp struct {
	logger *slog.Logger
	store  *ratelimiter.SQLiteStore
	dbPath string
}

type homeData struct {
	Version string
	DBPath  string
	Files   []ratelimiter.StoredFile
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("command failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string, logger *slog.Logger) error {
	if len(args) == 0 {
		return runServe(logger, nil)
	}

	switch args[0] {
	case "serve":
		return runServe(logger, args[1:])
	case "save":
		return runSave(logger, args[1:])
	case "list":
		return runList(logger, args[1:])
	default:
		return fmt.Errorf("unknown command %q (use serve, save, or list)", args[0])
	}
}

func runServe(logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", ":8080", "HTTP listen address")
	dbPath := fs.String("db", "ratelimiter.db", "SQLite database path")
	capacity := fs.Int("capacity", 10, "token bucket capacity")
	refillRate := fs.Int("refill", 5, "tokens added per second")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	store, err := ratelimiter.OpenSQLiteStore(*dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	rl, err := ratelimiter.NewTokenBucketRateLimiter(*capacity, *refillRate)
	if err != nil {
		return err
	}

	app := &demoApp{logger: logger, store: store, dbPath: *dbPath}
	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleHome)
	mux.HandleFunc("/upload", app.handleUpload)
	mux.HandleFunc("/download", app.handleDownload)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	handler := ratelimiter.RequestLoggerMiddleware(logger)(ratelimiter.HTTPMiddleware(rl, func(w http.ResponseWriter, r *http.Request) {
		logger.Warn("request rate limited", "path", r.URL.Path, "remote_addr", r.RemoteAddr, "retry_after", rl.NextAvailable().String())
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
	})(mux))

	srv := &http.Server{
		Addr:         *addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.Info("starting demo server", "addr", *addr, "db", *dbPath)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func runSave(logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("save", flag.ContinueOnError)
	dbPath := fs.String("db", "ratelimiter.db", "SQLite database path")
	filePath := fs.String("file", "", "Path to a file to save")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *filePath == "" {
		return fmt.Errorf("save requires -file")
	}

	store, err := ratelimiter.OpenSQLiteStore(*dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	saved, err := store.SaveFileFromPath(context.Background(), *filePath)
	if err != nil {
		return err
	}

	logger.Info("file saved to SQLite", "id", saved.ID, "name", saved.Name, "size", saved.Size, "db", *dbPath)
	fmt.Printf("saved %s as id %d (%d bytes)\n", saved.Name, saved.ID, saved.Size)
	return nil
}

func runList(logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	dbPath := fs.String("db", "ratelimiter.db", "SQLite database path")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	store, err := ratelimiter.OpenSQLiteStore(*dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	files, err := store.ListFiles(context.Background())
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Println("no files saved yet")
		return nil
	}

	for _, file := range files {
		fmt.Printf("%d\t%s\t%d bytes\t%s\n", file.ID, file.Name, file.Size, file.CreatedAt.Format(time.RFC3339))
	}
	logger.Info("listed SQLite files", "count", len(files), "db", *dbPath)
	return nil
}

func (a *demoApp) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	files, err := a.store.ListFiles(r.Context())
	if err != nil {
		a.logger.Error("failed to list files", "error", err)
		http.Error(w, "failed to load files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := homeTemplate.Execute(w, homeData{Version: ratelimiter.Version, DBPath: a.dbPath, Files: files}); err != nil {
		a.logger.Error("failed to render home page", "error", err)
	}
}

func (a *demoApp) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "failed to parse upload", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}
	if len(data) > maxUploadBytes {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	saved, err := a.store.SaveFile(r.Context(), header.Filename, contentType, data)
	if err != nil {
		a.logger.Error("failed to save upload", "error", err, "name", header.Filename)
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	a.logger.Info("file uploaded", "id", saved.ID, "name", saved.Name, "size", saved.Size)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *demoApp) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	file, err := a.store.GetFile(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("failed to load file", "error", err, "id", id)
		http.Error(w, "failed to load file", http.StatusInternalServerError)
		return
	}

	if file.ContentType == "" {
		file.ContentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.Name))
	http.ServeContent(w, r, file.Name, file.CreatedAt, bytes.NewReader(file.Data))
}
