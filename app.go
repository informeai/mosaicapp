package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/informeai/mosaicapp/internal/dedup"
	"github.com/informeai/mosaicapp/internal/store"
)

// FileCard is the JSON shape sent to the frontend for each stored file.
type FileCard struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	ChunkCount   int    `json:"chunkCount"`
	SavedBytes   int64  `json:"savedBytes"`
	SavedPercent int    `json:"savedPercent"`
	CreatedAt    string `json:"createdAt"`
}

// App is the Wails-bound application backend.
type App struct {
	ctx context.Context
	svc *dedup.Service
}

// NewApp creates the App backend and opens its SQLite store. The database
// lives under the OS user config directory so the tool works fully offline
// and keeps its data between runs.
func NewApp() *App {
	dbPath, err := dataFilePath()
	if err != nil {
		panic(fmt.Errorf("resolve database path: %w", err))
	}
	st, err := store.Open(dbPath)
	if err != nil {
		panic(fmt.Errorf("open database at %s: %w", dbPath, err))
	}
	return &App{svc: dedup.New(st, 0, 0, 0)}
}

func dataFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "mosaicapp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "mosaic.db"), nil
}

// startup is called by Wails once the frontend window is ready.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called by Wails when the application is closing.
func (a *App) shutdown(ctx context.Context) {
	a.svc.Close()
}

// UploadFile opens a native file picker and, if the user selects a file,
// ingests it into the dedup store.
func (a *App) UploadFile() (*FileCard, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Selecionar arquivo para deduplicar",
	})
	if err != nil {
		return nil, fmt.Errorf("abrir seletor de arquivo: %w", err)
	}
	if path == "" {
		return nil, nil // user cancelled
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ler arquivo: %w", err)
	}

	info, err := a.svc.Ingest(filepath.Base(path), data)
	if err != nil {
		return nil, fmt.Errorf("processar arquivo: %w", err)
	}

	card := toFileCard(info)
	return &card, nil
}

// ListFiles returns every file stored so far, most recent first.
func (a *App) ListFiles() ([]FileCard, error) {
	list, err := a.svc.List()
	if err != nil {
		return nil, err
	}
	cards := make([]FileCard, len(list))
	for i, info := range list {
		cards[i] = toFileCard(info)
	}
	return cards, nil
}

// DownloadFile reconstructs a stored file from its chunks and, if the user
// confirms a destination in the native save dialog, writes it to disk.
// It returns the saved path, or "" if the user cancelled.
func (a *App) DownloadFile(id int64) (string, error) {
	name, data, err := a.svc.Reconstruct(id)
	if err != nil {
		return "", fmt.Errorf("reconstruir arquivo: %w", err)
	}

	dest, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Salvar arquivo reconstruído",
		DefaultFilename: name,
	})
	if err != nil {
		return "", fmt.Errorf("abrir seletor de destino: %w", err)
	}
	if dest == "" {
		return "", nil // user cancelled
	}

	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", fmt.Errorf("gravar arquivo: %w", err)
	}
	return dest, nil
}

func toFileCard(info dedup.FileInfo) FileCard {
	return FileCard{
		ID:           info.ID,
		Name:         info.Name,
		Size:         info.Size,
		SHA256:       info.SHA256,
		ChunkCount:   info.ChunkCount,
		SavedBytes:   info.SavedBytes,
		SavedPercent: int(info.SavedPercent + 0.5),
		CreatedAt:    info.CreatedAt.Format(time.RFC3339),
	}
}
