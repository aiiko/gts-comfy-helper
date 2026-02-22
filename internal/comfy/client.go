package comfy

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed workflow.json
var defaultWorkflowBytes []byte

var errHistoryNotReady = errors.New("comfy history not ready")

type Client struct {
	baseURL      string
	timeout      time.Duration
	pollInterval time.Duration
	httpClient   *http.Client
}

type GenerateInput struct {
	PositivePrompt string
	NegativePrompt string
	Width          int
	Height         int
	Steps          int
	CFG            float64
	Sampler        string
	Scheduler      string
	Seed           int64
	ClientID       string
}

type GenerateResult struct {
	PromptID string
	Bytes    []byte
	MIME     string
}

type ProgressEvent struct {
	Stage    string
	PromptID string
	Frame    []byte
	MIME     string
	Warning  string
}

type promptResponse struct {
	PromptID any `json:"prompt_id"`
}

type historyRecord struct {
	Outputs map[string]struct {
		Images []imageRef `json:"images"`
	} `json:"outputs"`
}

type imageRef struct {
	Filename  string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type      string `json:"type"`
}

func NewClient(baseURL string, timeout, pollInterval time.Duration, httpClient *http.Client) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	if pollInterval <= 0 {
		pollInterval = 1200 * time.Millisecond
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{baseURL: baseURL, timeout: timeout, pollInterval: pollInterval, httpClient: httpClient}
}

func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

func (c *Client) GenerateWithProgress(ctx context.Context, input GenerateInput, onProgress func(ProgressEvent)) (GenerateResult, error) {
	if c == nil || !c.Enabled() {
		return GenerateResult{}, fmt.Errorf("comfy client disabled")
	}
	input = withDefaults(input)

	workflow, err := buildWorkflow(input)
	if err != nil {
		return GenerateResult{}, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	wsConn, wsWarn := c.openPreviewConnection(requestCtx, input.ClientID)
	if wsConn != nil {
		defer wsConn.Close()
	}

	promptID, err := c.queuePrompt(requestCtx, workflow, input.ClientID)
	if err != nil {
		return GenerateResult{}, err
	}
	notifyProgress(onProgress, ProgressEvent{Stage: "queued", PromptID: promptID})

	if wsConn != nil {
		if err := c.consumePreviewStreamFromConn(requestCtx, wsConn, promptID, onProgress); err != nil {
			notifyProgress(onProgress, ProgressEvent{Stage: "fallback_history", PromptID: promptID, Warning: "live preview disconnected; using history polling"})
		}
	} else if wsWarn != "" {
		notifyProgress(onProgress, ProgressEvent{Stage: "fallback_history", PromptID: promptID, Warning: wsWarn})
	}

	imgRef, err := c.waitForImage(requestCtx, promptID)
	if err != nil {
		return GenerateResult{}, err
	}
	imgBytes, mime, err := c.downloadImage(requestCtx, imgRef)
	if err != nil {
		return GenerateResult{}, err
	}
	return GenerateResult{PromptID: promptID, Bytes: imgBytes, MIME: mime}, nil
}

func notifyProgress(onProgress func(ProgressEvent), event ProgressEvent) {
	if onProgress == nil {
		return
	}
	onProgress(event)
}

func (c *Client) openPreviewConnection(ctx context.Context, clientID string) (*websocket.Conn, string) {
	wsURL, err := comfyWSURL(c.baseURL, strings.TrimSpace(clientID))
	if err != nil {
		return nil, "live preview unavailable; invalid websocket URL"
	}
	dialer := websocket.Dialer{HandshakeTimeout: 6 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, "live preview unavailable; websocket connection failed"
	}
	return conn, ""
}

func comfyWSURL(baseURL, clientID string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", fmt.Errorf("parse comfy base URL: %w", err)
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported comfy base scheme %q", parsed.Scheme)
	}
	parsed.Path = "/ws"
	q := parsed.Query()
	if strings.TrimSpace(clientID) != "" {
		q.Set("clientId", clientID)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

func (c *Client) consumePreviewStreamFromConn(ctx context.Context, conn *websocket.Conn, promptID string, onProgress func(ProgressEvent)) error {
	ctxDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-ctxDone:
		}
	}()
	defer close(ctxDone)

	for {
		msgType, message, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return err
		}

		switch msgType {
		case websocket.TextMessage:
			if isPromptDoneMessage(message, promptID) {
				return nil
			}
		case websocket.BinaryMessage:
			frame, mime := decodePreviewFrame(message)
			if len(frame) == 0 {
				continue
			}
			notifyProgress(onProgress, ProgressEvent{Stage: "preview_frame", PromptID: promptID, Frame: frame, MIME: mime})
		}
	}
}

func isPromptDoneMessage(message []byte, promptID string) bool {
	var payload struct {
		Type string `json:"type"`
		Data struct {
			Node     any `json:"node"`
			PromptID any `json:"prompt_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(message, &payload); err != nil {
		return false
	}
	if strings.TrimSpace(payload.Type) != "executing" {
		return false
	}
	if payload.Data.Node != nil {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(payload.Data.PromptID)) == strings.TrimSpace(promptID)
}

func decodePreviewFrame(message []byte) ([]byte, string) {
	if len(message) < 8 {
		return nil, ""
	}
	frame := message[8:]
	if len(frame) == 0 {
		return nil, ""
	}
	mime := http.DetectContentType(frame)
	if mime != "image/png" && mime != "image/jpeg" && mime != "image/webp" {
		return nil, ""
	}
	return frame, mime
}

func withDefaults(input GenerateInput) GenerateInput {
	input.PositivePrompt = strings.TrimSpace(input.PositivePrompt)
	input.NegativePrompt = strings.TrimSpace(input.NegativePrompt)
	if input.Width <= 0 {
		input.Width = 1152
	}
	if input.Height <= 0 {
		input.Height = 768
	}
	if input.Steps <= 0 {
		input.Steps = 28
	}
	if input.CFG <= 0 {
		input.CFG = 4
	}
	if strings.TrimSpace(input.Sampler) == "" {
		input.Sampler = "dpmpp_2m_sde_gpu"
	}
	if strings.TrimSpace(input.Scheduler) == "" {
		input.Scheduler = "simple"
	}
	if strings.TrimSpace(input.ClientID) == "" {
		input.ClientID = "gts-comfy-helper"
	}
	return input
}

func buildWorkflow(input GenerateInput) (map[string]any, error) {
	workflow := make(map[string]any)
	if err := json.Unmarshal(defaultWorkflowBytes, &workflow); err != nil {
		return nil, fmt.Errorf("decode workflow: %w", err)
	}
	if err := setWorkflowNodeInput(workflow, "11", "text", input.PositivePrompt); err != nil {
		return nil, err
	}
	if err := setWorkflowNodeInput(workflow, "12", "text", input.NegativePrompt); err != nil {
		return nil, err
	}
	if err := setWorkflowNodeInput(workflow, "19", "seed", input.Seed); err != nil {
		return nil, err
	}
	if err := setWorkflowNodeInput(workflow, "19", "steps", input.Steps); err != nil {
		return nil, err
	}
	if err := setWorkflowNodeInput(workflow, "19", "cfg", input.CFG); err != nil {
		return nil, err
	}
	if err := setWorkflowNodeInput(workflow, "19", "sampler_name", input.Sampler); err != nil {
		return nil, err
	}
	if err := setWorkflowNodeInput(workflow, "19", "scheduler", input.Scheduler); err != nil {
		return nil, err
	}
	if err := setWorkflowNodeInput(workflow, "28", "width", input.Width); err != nil {
		return nil, err
	}
	if err := setWorkflowNodeInput(workflow, "28", "height", input.Height); err != nil {
		return nil, err
	}
	return workflow, nil
}

func setWorkflowNodeInput(workflow map[string]any, nodeID, key string, value any) error {
	node, ok := workflow[nodeID].(map[string]any)
	if !ok {
		return fmt.Errorf("workflow node %s not found", nodeID)
	}
	inputs, ok := node["inputs"].(map[string]any)
	if !ok {
		return fmt.Errorf("workflow node %s missing inputs", nodeID)
	}
	inputs[key] = value
	return nil
}

func (c *Client) queuePrompt(ctx context.Context, workflow map[string]any, clientID string) (string, error) {
	payload := map[string]any{"prompt": workflow, "client_id": strings.TrimSpace(clientID)}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal prompt payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/prompt", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build prompt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("prompt request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read prompt response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("prompt http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	parsed := promptResponse{}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("decode prompt response: %w", err)
	}
	promptID := strings.TrimSpace(fmt.Sprint(parsed.PromptID))
	if promptID == "" || promptID == "<nil>" {
		return "", fmt.Errorf("prompt response missing prompt_id")
	}
	return promptID, nil
}

func (c *Client) waitForImage(ctx context.Context, promptID string) (imageRef, error) {
	promptID = strings.TrimSpace(promptID)
	if promptID == "" {
		return imageRef{}, fmt.Errorf("prompt_id is required")
	}
	for {
		ref, err := c.fetchFirstImageFromHistory(ctx, promptID)
		if err == nil {
			return ref, nil
		}
		if !errors.Is(err, errHistoryNotReady) {
			return imageRef{}, err
		}
		select {
		case <-ctx.Done():
			return imageRef{}, fmt.Errorf("wait history timed out: %w", ctx.Err())
		case <-time.After(c.pollInterval):
		}
	}
}

func (c *Client) fetchFirstImageFromHistory(ctx context.Context, promptID string) (imageRef, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/history/"+url.PathEscape(promptID), nil)
	if err != nil {
		return imageRef{}, fmt.Errorf("build history request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return imageRef{}, fmt.Errorf("history request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return imageRef{}, fmt.Errorf("read history response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return imageRef{}, fmt.Errorf("history http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	history := make(map[string]historyRecord)
	if err := json.Unmarshal(respBytes, &history); err != nil {
		return imageRef{}, fmt.Errorf("decode history response: %w", err)
	}
	if len(history) == 0 {
		return imageRef{}, errHistoryNotReady
	}

	record, ok := history[promptID]
	if !ok {
		for _, item := range history {
			record = item
			ok = true
			break
		}
	}
	if !ok {
		return imageRef{}, errHistoryNotReady
	}

	keys := make([]string, 0, len(record.Outputs))
	for key := range record.Outputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, img := range record.Outputs[key].Images {
			if strings.TrimSpace(img.Filename) == "" {
				continue
			}
			return img, nil
		}
	}
	return imageRef{}, errHistoryNotReady
}

func (c *Client) downloadImage(ctx context.Context, img imageRef) ([]byte, string, error) {
	values := url.Values{}
	values.Set("filename", strings.TrimSpace(img.Filename))
	if strings.TrimSpace(img.Subfolder) != "" {
		values.Set("subfolder", strings.TrimSpace(img.Subfolder))
	}
	if strings.TrimSpace(img.Type) != "" {
		values.Set("type", strings.TrimSpace(img.Type))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/view?"+values.Encode(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("build view request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("view request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, "", fmt.Errorf("read view response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("view http %d", resp.StatusCode)
	}
	if len(body) == 0 {
		return nil, "", fmt.Errorf("view image is empty")
	}
	mime := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if mime == "" {
		mime = http.DetectContentType(body)
	}
	return body, mime, nil
}
