package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const telemetryEnvVar = "KEEN_TELEMETRY"

type state struct {
	ClientID string `json:"client_id"`
}

func loadOrCreateClientID() (string, error) {
	current, err := loadState()
	if err == nil && current.ClientID != "" {
		return current.ClientID, nil
	}
	clientID, err := randomID()
	if err != nil {
		return "", err
	}
	if err := saveState(state{ClientID: clientID}); err != nil {
		return "", err
	}
	return clientID, nil
}

func loadState() (state, error) {
	data, err := os.ReadFile(statePath())
	if err != nil {
		return state{}, err
	}
	var current state
	if err := json.Unmarshal(data, &current); err != nil {
		return state{}, err
	}
	return current, nil
}

func saveState(current state) error {
	if err := os.MkdirAll(filepath.Dir(statePath()), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(), append(data, '\n'), 0600)
}

func statePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".keen", "telemetry.json")
}

func randomID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate telemetry ID: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func telemetryEnvironmentSetting() (bool, bool) {
	value, ok := os.LookupEnv(telemetryEnvVar)
	if !ok {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "on", "yes":
		return true, true
	case "0", "false", "off", "no":
		return false, true
	default:
		return false, false
	}
}

func envDisabled(name string) bool {
	value, ok := os.LookupEnv(name)
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "off", "no":
		return false
	default:
		return true
	}
}
