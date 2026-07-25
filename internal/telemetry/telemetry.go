package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	eventSessionStart = "keen_session_start"
	eventSessionEnd   = "keen_session_end"

	endpoint       = "https://www.google-analytics.com/mp/collect"
	requestTimeout = time.Second
)

type Config struct {
	MeasurementID string
	APISecret     string
	Version       string
	Mode          string
}

type Reporter struct {
	clientID  string
	config    Config
	startedAt time.Time
	sessionID string
}

type event struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params"`
}

type payload struct {
	ClientID string  `json:"client_id"`
	Events   []event `json:"events"`
}

func New(cfg Config) *Reporter {
	if !Enabled(cfg.MeasurementID, cfg.APISecret) {
		return nil
	}
	clientID, err := loadOrCreateClientID()
	if err != nil {
		slog.Debug("telemetry disabled", "reason", "client ID unavailable")
		return nil
	}

	now := time.Now()
	r := &Reporter{
		clientID:  clientID,
		config:    cfg,
		startedAt: now,
		sessionID: strconv.FormatInt(now.UnixMilli(), 10),
	}
	go r.emit(r.newEvent(eventSessionStart, map[string]any{"engagement_time_msec": 1}))
	return r
}

func (r *Reporter) Close() {
	if r == nil {
		return
	}
	duration := time.Since(r.startedAt)
	r.emit(r.newEvent(eventSessionEnd, map[string]any{
		"duration_msec":        duration.Milliseconds(),
		"engagement_time_msec": max(duration.Milliseconds(), int64(1)),
	}))
}

func Enabled(measurementID, apiSecret string) bool {
	if strings.TrimSpace(measurementID) == "" || strings.TrimSpace(apiSecret) == "" {
		return false
	}
	if envDisabled("DO_NOT_TRACK") || envDisabled("CI") {
		return false
	}
	if enabled, set := telemetryEnvironmentSetting(); set {
		return enabled
	}
	return true
}

func (r *Reporter) newEvent(name string, params map[string]any) event {
	params["session_id"] = r.sessionID
	params["keen_version"] = r.config.Version
	params["os"] = runtime.GOOS
	params["arch"] = runtime.GOARCH
	params["mode"] = r.config.Mode
	return event{Name: name, Params: params}
}

func (r *Reporter) emit(item event) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if err := r.send(ctx, item); err != nil {
		slog.Debug("telemetry delivery failed")
	}
}

func (r *Reporter) send(ctx context.Context, item event) error {
	body, err := json.Marshal(payload{ClientID: r.clientID, Events: []event{item}})
	if err != nil {
		return err
	}
	query := url.Values{
		"measurement_id": {r.config.MeasurementID},
		"api_secret":     {r.config.APISecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+query.Encode(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected telemetry status %d", resp.StatusCode)
	}
	return nil
}
