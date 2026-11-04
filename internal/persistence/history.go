package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/qbgames/qbdz-game/internal/tableapp"
)

type JSONLHistoryRecorder struct {
	mu   sync.Mutex
	path string
}

func NewJSONLHistoryRecorder(path string) (*JSONLHistoryRecorder, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create history directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open history file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close history file: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	return &JSONLHistoryRecorder{path: path}, nil
}

func (r *JSONLHistoryRecorder) RecordHand(record tableapp.HandRecord) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode hand record: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	file, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return fmt.Errorf("write hand record: %w", err)
	}
	return nil
}
