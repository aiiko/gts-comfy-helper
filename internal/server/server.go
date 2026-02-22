package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gts-comfy-helper/internal/comfy"
	"gts-comfy-helper/internal/config"
	"gts-comfy-helper/internal/storage"
)

type App struct {
	cfg        config.Config
	db         *storage.DB
	comfy      *comfy.Client
	previewHub *previewHub
	assetsDir  string
}

func New(cfg config.Config, db *storage.DB) http.Handler {
	app := &App{
		cfg:        cfg,
		db:         db,
		comfy:      comfy.NewClient(cfg.ComfyBaseURL, time.Duration(cfg.ComfyTimeoutMs)*time.Millisecond, time.Duration(cfg.ComfyPollMs)*time.Millisecond, nil),
		previewHub: newPreviewHub(2 * time.Minute),
		assetsDir:  filepath.Join(cfg.DataDir, "assets"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", app.handleHealth)
	mux.HandleFunc("GET /api/settings", app.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", app.handlePutSettings)
	mux.HandleFunc("POST /api/generate", app.handleGenerate)
	mux.HandleFunc("GET /api/jobs/{id}", app.handleGetJob)
	mux.HandleFunc("GET /api/jobs/{id}/preview", app.handleGetJobPreview)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(app.assetsDir))))
	mux.Handle("GET /", http.FileServer(http.Dir("web")))

	return requestLogger(mux)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "elapsed_ms", time.Since(started).Milliseconds())
	})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := a.db.Health(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "db_path": a.db.Path(), "comfy_base_url": a.cfg.ComfyBaseURL})
}

func (a *App) saveImage(jobID string, bytes []byte) (string, error) {
	if err := os.MkdirAll(a.assetsDir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s.png", jobID)
	path := filepath.Join(a.assetsDir, name)
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

func (a *App) processGenerateJob(parent context.Context, job storage.Job) {
	ctx := context.Background()
	if parent != nil {
		ctx = parent
	}
	a.previewHub.setWaiting(job.ID)
	if err := a.db.UpdateJobRunning(ctx, job.ID); err != nil {
		a.previewHub.setFailed(job.ID, "failed to mark running")
		_ = a.db.UpdateJobFailed(ctx, job.ID, err.Error())
		return
	}

	result, err := a.comfy.GenerateWithProgress(ctx, comfy.GenerateInput{
		PositivePrompt: job.PromptFinal,
		NegativePrompt: job.NegativePrompt,
		Width:          1152,
		Height:         768,
		Steps:          28,
		CFG:            4,
		Sampler:        "dpmpp_2m_sde_gpu",
		Scheduler:      "simple",
		Seed:           time.Now().UnixNano(),
		ClientID:       "gts-comfy-helper-" + job.ID,
	}, func(event comfy.ProgressEvent) {
		switch event.Stage {
		case "queued":
			_ = a.db.UpdateJobComfyPromptID(context.Background(), job.ID, event.PromptID)
			a.previewHub.setQueued(job.ID, event.PromptID)
		case "fallback_history":
			a.previewHub.setFallback(job.ID, event.PromptID, event.Warning)
		case "preview_frame":
			a.previewHub.setFrame(job.ID, event.PromptID, event.MIME, event.Frame)
		}
	})
	if err != nil {
		a.previewHub.setFailed(job.ID, "comfy generation failed")
		_ = a.db.UpdateJobFailed(context.Background(), job.ID, err.Error())
		return
	}

	fileName, err := a.saveImage(job.ID, result.Bytes)
	if err != nil {
		a.previewHub.setFailed(job.ID, "failed writing output image")
		_ = a.db.UpdateJobFailed(context.Background(), job.ID, err.Error())
		return
	}

	if err := a.db.UpdateJobDone(context.Background(), job.ID, fileName); err != nil {
		a.previewHub.setFailed(job.ID, "failed updating job done")
		_ = a.db.UpdateJobFailed(context.Background(), job.ID, err.Error())
		return
	}
	a.previewHub.setDone(job.ID)
}
