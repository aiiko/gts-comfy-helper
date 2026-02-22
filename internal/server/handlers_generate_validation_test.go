package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleGenerateRejectsInvalidCharacterFields(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantText   string
	}{
		{
			name:       "invalid giantess count",
			body:       `{"prompt":"scene","giantess_count":3,"tinies_mode":"count","tiny_count":1}`,
			wantStatus: http.StatusBadRequest,
			wantText:   "giantess_count must be 1 or 2",
		},
		{
			name:       "invalid tinies mode",
			body:       `{"prompt":"scene","giantess_count":1,"tinies_mode":"many","tiny_count":1}`,
			wantStatus: http.StatusBadRequest,
			wantText:   "tinies_mode must be count or group",
		},
		{
			name:       "invalid tiny count for count mode",
			body:       `{"prompt":"scene","giantess_count":1,"tinies_mode":"count","tiny_count":0}`,
			wantStatus: http.StatusBadRequest,
			wantText:   "tiny_count must be a positive integer when tinies_mode is count",
		},
		{
			name:       "invalid tiny gender",
			body:       `{"prompt":"scene","giantess_count":1,"tinies_mode":"count","tiny_count":1,"tiny_gender":"unknown"}`,
			wantStatus: http.StatusBadRequest,
			wantText:   "tiny_gender must be one of male, female, girl, or boy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			app := &App{}
			app.handleGenerate(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status mismatch: want %d, got %d", tt.wantStatus, w.Code)
			}

			resp := map[string]any{}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			errorObj, _ := resp["error"].(map[string]any)
			message, _ := errorObj["message"].(string)
			if !strings.Contains(message, tt.wantText) {
				t.Fatalf("error mismatch: want to contain %q, got %q", tt.wantText, message)
			}
		})
	}
}
