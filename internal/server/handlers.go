package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"gts-comfy-helper/internal/storage"
)

const maxRequestBodyBytes = 1 << 20

type settingsPayload struct {
	PositiveTags    *string `json:"positive_tags"`
	NegativeTags    *string `json:"negative_tags"`
	LastAspectRatio *string `json:"last_aspect_ratio"`
}

type generatePayload struct {
	Prompt         string `json:"prompt"`
	GiantessCount  int    `json:"giantess_count"`
	TiniesMode     string `json:"tinies_mode"`
	TinyCount      int    `json:"tiny_count"`
	TinyGender     string `json:"tiny_gender"`
	TinyDescriptor string `json:"tiny_descriptor"`
	ArtStyle       string `json:"art_style"`
	BodyFraming    string `json:"body_framing"`
	CameraSelector string `json:"camera_selector"`
	AspectRatio    string `json:"aspect_ratio"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
}

func (a *App) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.db.Settings(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "load settings: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"positive_tags":     strings.TrimSpace(settings["positive_tags"]),
		"negative_tags":     strings.TrimSpace(settings["negative_tags"]),
		"last_aspect_ratio": strings.TrimSpace(settings["last_aspect_ratio"]),
	})
}

func (a *App) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var payload settingsPayload
	if !decodeJSONBody(w, r, &payload) {
		return
	}

	values := make(map[string]string)
	if payload.PositiveTags != nil {
		values["positive_tags"] = strings.TrimSpace(*payload.PositiveTags)
	}
	if payload.NegativeTags != nil {
		values["negative_tags"] = strings.TrimSpace(*payload.NegativeTags)
	}
	if payload.LastAspectRatio != nil {
		aspectRatio := strings.ToLower(strings.TrimSpace(*payload.LastAspectRatio))
		if aspectRatio == "" {
			aspectRatio = "square"
		}
		if _, _, ok := dimensionsForAspect(aspectRatio); !ok {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "last_aspect_ratio must be portrait, square, or landscape")
			return
		}
		values["last_aspect_ratio"] = aspectRatio
	}
	if len(values) == 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "at least one settings field is required")
		return
	}

	if err := a.db.UpsertSettings(r.Context(), values); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "save settings: "+err.Error())
		return
	}

	settings, err := a.db.Settings(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "load settings: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"positive_tags":     strings.TrimSpace(settings["positive_tags"]),
		"negative_tags":     strings.TrimSpace(settings["negative_tags"]),
		"last_aspect_ratio": strings.TrimSpace(settings["last_aspect_ratio"]),
	})
}

func (a *App) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var payload generatePayload
	if !decodeJSONBody(w, r, &payload) {
		return
	}
	payload.Prompt = strings.TrimSpace(payload.Prompt)
	aspectRatio := strings.ToLower(strings.TrimSpace(payload.AspectRatio))
	if aspectRatio == "" {
		aspectRatio = "square"
	}
	width, height, ok := dimensionsForAspect(aspectRatio)
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "aspect_ratio must be portrait, square, or landscape")
		return
	}
	if payload.Width > 0 && payload.Height > 0 && (payload.Width != width || payload.Height != height) {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "width/height do not match aspect_ratio")
		return
	}
	cameraSelector, ok := canonicalOption(payload.CameraSelector, cameraSelectorOptionsMap)
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "camera_selector must match one of supported camera selector values")
		return
	}
	artStyle, ok := canonicalOption(payload.ArtStyle, artStyleOptionsMap)
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "art_style must match one of supported art styles")
		return
	}
	bodyFraming, ok := canonicalOption(payload.BodyFraming, bodyFramingOptionsMap)
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "body_framing must match one of supported body framing values")
		return
	}
	if payload.GiantessCount != 1 && payload.GiantessCount != 2 {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "giantess_count must be 1 or 2")
		return
	}
	tiniesMode, ok := canonicalOption(payload.TiniesMode, tiniesModeOptionsMap)
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "tinies_mode must be count or group")
		return
	}
	tinyGender, ok := canonicalOption(payload.TinyGender, tinyGenderOptionsMap)
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "tiny_gender must be one of male, female, girl, or boy")
		return
	}
	if tiniesMode == "count" && payload.TinyCount <= 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "tiny_count must be a positive integer when tinies_mode is count")
		return
	}
	characterDefinition, err := buildCharacterDefinition(payload.GiantessCount, tiniesMode, payload.TinyCount, tinyGender, payload.TinyDescriptor)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	settings, err := a.db.Settings(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "load settings: "+err.Error())
		return
	}
	positive := strings.TrimSpace(settings["positive_tags"])
	negative := strings.TrimSpace(settings["negative_tags"])
	finalPrompt := buildFinalPrompt(positive, characterDefinition, payload.Prompt, artStyle, bodyFraming, cameraSelector)

	job, err := a.db.CreateJob(r.Context(), storage.Job{
		ID:             uuid.NewString(),
		Status:         storage.JobStatusQueued,
		PromptRaw:      payload.Prompt,
		PromptFinal:    finalPrompt,
		NegativePrompt: negative,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "create job: "+err.Error())
		return
	}

	go a.processGenerateJob(context.Background(), job, width, height)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":       job.ID,
		"status":       job.Status,
		"aspect_ratio": aspectRatio,
		"width":        width,
		"height":       height,
	})
}

func dimensionsForAspect(aspectRatio string) (int, int, bool) {
	switch strings.ToLower(strings.TrimSpace(aspectRatio)) {
	case "":
		return 1024, 1024, true
	case "portrait":
		return 896, 1152, true
	case "square":
		return 1024, 1024, true
	case "landscape":
		return 1152, 896, true
	default:
		return 0, 0, false
	}
}

func (a *App) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "job id is required")
		return
	}
	job, err := a.db.GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeAPIError(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "get job: "+err.Error())
		return
	}
	assetURL := ""
	if strings.TrimSpace(job.OutputFile) != "" {
		assetURL = "/assets/" + strings.TrimSpace(job.OutputFile)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"job_id":       job.ID,
		"status":       job.Status,
		"error":        job.Error,
		"asset_url":    assetURL,
		"prompt_final": job.PromptFinal,
	})
}

func (a *App) handleGetJobPreview(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "job id is required")
		return
	}
	job, err := a.db.GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeAPIError(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "get job: "+err.Error())
		return
	}

	sinceSeq := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("since_seq")); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed < 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "since_seq must be a non-negative integer")
			return
		}
		sinceSeq = parsed
	}

	snapshot := a.previewHub.get(id, sinceSeq)
	previewStatus := previewStatusWaiting
	warning := ""
	seq := int64(0)
	promptID := strings.TrimSpace(job.ComfyPromptID)
	updatedAt := ""
	framePayload := map[string]any(nil)

	if snapshot.Found {
		previewStatus = snapshot.Status
		warning = snapshot.Warning
		seq = snapshot.Seq
		if strings.TrimSpace(snapshot.PromptID) != "" {
			promptID = snapshot.PromptID
		}
		if !snapshot.UpdatedAt.IsZero() {
			updatedAt = snapshot.UpdatedAt.UTC().Format(time.RFC3339Nano)
		}
		if snapshot.NewerFrame && len(snapshot.Frame) > 0 {
			framePayload = map[string]any{
				"seq":         snapshot.Seq,
				"mime":        snapshot.MIME,
				"data_base64": base64.StdEncoding.EncodeToString(snapshot.Frame),
			}
		}
	} else {
		switch job.Status {
		case storage.JobStatusDone:
			previewStatus = previewStatusDone
		case storage.JobStatusFailed:
			previewStatus = previewStatusFailed
			warning = strings.TrimSpace(job.Error)
		default:
			previewStatus = previewStatusWaiting
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"job_id":         job.ID,
		"job_status":     job.Status,
		"preview_status": previewStatus,
		"prompt_id":      promptID,
		"seq":            seq,
		"updated_at":     updatedAt,
		"warning":        warning,
		"frame":          framePayload,
	})
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body is required")
			return false
		}
		writeAPIError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body must contain a single JSON object")
		return false
	}
	return true
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    strings.TrimSpace(code),
			"message": strings.TrimSpace(message),
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
