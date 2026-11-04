package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/qbgames/qbdz-game/internal/tableapp"
)

func TestJSONLHistoryRecorder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hands.jsonl")
	recorder, err := NewJSONLHistoryRecorder(path)
	if err != nil {
		t.Fatal(err)
	}
	want := tableapp.HandRecord{TableID: "main", HandID: "hand-1"}
	if err := recorder.RecordHand(want); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got tableapp.HandRecord
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.HandID != want.HandID || got.TableID != want.TableID {
		t.Fatalf("unexpected record: %+v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected history permissions: %o", info.Mode().Perm())
	}
}
