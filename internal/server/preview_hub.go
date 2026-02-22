package server

import (
	"sync"
	"time"
)

const (
	previewStatusWaiting         = "waiting"
	previewStatusStreaming       = "streaming"
	previewStatusFallbackHistory = "fallback_history"
	previewStatusDone            = "done"
	previewStatusFailed          = "failed"
	previewStatusNotApplicable   = "not_applicable"
)

type previewHub struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]*previewEntry
}

type previewEntry struct {
	Seq       int64
	Status    string
	PromptID  string
	Warning   string
	MIME      string
	Frame     []byte
	UpdatedAt time.Time
	Finished  bool
}

type previewSnapshot struct {
	Found      bool
	Seq        int64
	Status     string
	PromptID   string
	Warning    string
	MIME       string
	Frame      []byte
	UpdatedAt  time.Time
	NewerFrame bool
}

func newPreviewHub(ttl time.Duration) *previewHub {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &previewHub{ttl: ttl, entries: map[string]*previewEntry{}}
}

func (h *previewHub) update(jobID string, mutate func(*previewEntry)) {
	if h == nil || jobID == "" {
		return
	}
	now := time.Now().UTC()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cleanupLocked(now)
	entry, ok := h.entries[jobID]
	if !ok {
		entry = &previewEntry{Status: previewStatusWaiting, UpdatedAt: now}
		h.entries[jobID] = entry
	}
	entry.Seq++
	entry.UpdatedAt = now
	mutate(entry)
}

func (h *previewHub) setWaiting(jobID string) {
	h.update(jobID, func(entry *previewEntry) {
		entry.Status = previewStatusWaiting
		entry.Warning = ""
		entry.MIME = ""
		entry.Frame = nil
		entry.Finished = false
	})
}

func (h *previewHub) setFailed(jobID, warning string) {
	h.update(jobID, func(entry *previewEntry) {
		entry.Status = previewStatusFailed
		entry.Warning = warning
		entry.Finished = true
	})
}

func (h *previewHub) setDone(jobID string) {
	h.update(jobID, func(entry *previewEntry) {
		entry.Status = previewStatusDone
		entry.Finished = true
	})
}

func (h *previewHub) setNotApplicable(jobID, warning string) {
	h.update(jobID, func(entry *previewEntry) {
		entry.Status = previewStatusNotApplicable
		entry.Warning = warning
		entry.Finished = true
	})
}

func (h *previewHub) setQueued(jobID, promptID string) {
	h.update(jobID, func(entry *previewEntry) {
		entry.Status = previewStatusWaiting
		entry.PromptID = promptID
		entry.Warning = ""
		entry.Finished = false
	})
}

func (h *previewHub) setFallback(jobID, promptID, warning string) {
	h.update(jobID, func(entry *previewEntry) {
		entry.Status = previewStatusFallbackHistory
		entry.PromptID = promptID
		entry.Warning = warning
		entry.Finished = false
	})
}

func (h *previewHub) setFrame(jobID, promptID, mime string, frame []byte) {
	if len(frame) == 0 {
		return
	}
	copyFrame := append([]byte(nil), frame...)
	h.update(jobID, func(entry *previewEntry) {
		entry.Status = previewStatusStreaming
		entry.PromptID = promptID
		entry.Warning = ""
		entry.MIME = mime
		entry.Frame = copyFrame
		entry.Finished = false
	})
}

func (h *previewHub) get(jobID string, sinceSeq int64) previewSnapshot {
	if h == nil || jobID == "" {
		return previewSnapshot{}
	}
	now := time.Now().UTC()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cleanupLocked(now)
	entry, ok := h.entries[jobID]
	if !ok {
		return previewSnapshot{}
	}
	snapshot := previewSnapshot{
		Found:      true,
		Seq:        entry.Seq,
		Status:     entry.Status,
		PromptID:   entry.PromptID,
		Warning:    entry.Warning,
		MIME:       entry.MIME,
		UpdatedAt:  entry.UpdatedAt,
		NewerFrame: entry.Seq > sinceSeq && len(entry.Frame) > 0,
	}
	if snapshot.NewerFrame {
		snapshot.Frame = append([]byte(nil), entry.Frame...)
	}
	return snapshot
}

func (h *previewHub) cleanupLocked(now time.Time) {
	for jobID, entry := range h.entries {
		if !entry.Finished {
			continue
		}
		if now.Sub(entry.UpdatedAt) <= h.ttl {
			continue
		}
		delete(h.entries, jobID)
	}
}
