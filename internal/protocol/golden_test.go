package protocol

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoldenFixturesAndHashes(t *testing.T) {
	dir := filepath.Join("testdata", "v1")
	manifest := readManifest(t, filepath.Join(dir, "SHA256SUMS"))

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}
	seen := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		seen++
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		wantHash, ok := manifest[entry.Name()]
		if !ok {
			t.Fatalf("%s missing from SHA256SUMS", entry.Name())
		}
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != wantHash {
			t.Fatalf("%s hash mismatch: got %s want %s", entry.Name(), got, wantHash)
		}
		if _, err := DecodeEnvelope(raw); err != nil {
			t.Fatalf("decode %s: %v", entry.Name(), err)
		}
	}
	if seen != 25 {
		t.Fatalf("expected 25 JSON fixtures, found %d", seen)
	}
}

func TestSnapshotFixturesDecodeIntoSnapshotPayload(t *testing.T) {
	for _, name := range []string{
		"06-table-snapshot-player.server.json",
		"07-table-snapshot-spectator.server.json",
	} {
		raw, err := os.ReadFile(filepath.Join("testdata", "v1", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		env, err := DecodeEnvelope(raw)
		if err != nil {
			t.Fatalf("decode envelope %s: %v", name, err)
		}
		var snapshot SnapshotPayload
		if err := env.DecodePayload(&snapshot); err != nil {
			t.Fatalf("decode snapshot %s: %v", name, err)
		}
		if len(snapshot.Seats) != 9 {
			t.Fatalf("%s: expected 9 seats, got %d", name, len(snapshot.Seats))
		}
		if snapshot.Config.SmallBlind != 10 || snapshot.Config.BigBlind != 20 {
			t.Fatalf("%s: unexpected blinds: %+v", name, snapshot.Config)
		}
		if snapshot.Config.StartingStack != 2000 {
			t.Fatalf("%s: unexpected starting stack: %+v", name, snapshot.Config)
		}
	}
}

func readManifest(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer f.Close()

	out := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			t.Fatalf("invalid manifest line: %q", line)
		}
		out[parts[1]] = parts[0]
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan manifest: %v", err)
	}
	return out
}
