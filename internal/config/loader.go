package config

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"gopkg.in/yaml.v3"
)

// Loader loads AllConfig from a YAML file and supports hot-reload on SIGHUP.
// Get() always returns a pointer to an immutable snapshot — callers must not
// mutate the returned value. On reload, the pointer is swapped atomically,
// so in-progress hands keep their snapshot while the next hand picks up the new config.
type Loader struct {
	path    string
	ptr     atomic.Pointer[AllConfig]
	mu      sync.Mutex // protects concurrent Reload() calls
}

// NewLoader creates a Loader that reads from path and immediately loads the config.
func NewLoader(path string) (*Loader, error) {
	l := &Loader{path: path}
	if err := l.Reload(); err != nil {
		return nil, fmt.Errorf("config: initial load from %q failed: %w", path, err)
	}
	return l, nil
}

// NewLoaderFromDefault creates a Loader pre-populated with DefaultSceneConfig.
// Useful in tests where no YAML file is needed.
func NewLoaderFromDefault() *Loader {
	l := &Loader{}
	cfg := &AllConfig{Scene: DefaultSceneConfig()}
	l.ptr.Store(cfg)
	return l
}

// Get returns the current config snapshot. The pointer is stable until the next
// Reload; do not store it beyond the current hand.
func (l *Loader) Get() *AllConfig {
	return l.ptr.Load()
}

// Reload re-reads the YAML file and atomically replaces the current config.
// Returns an error if the file cannot be read or parsed; the old config remains active.
func (l *Loader) Reload() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.path)
	if err != nil {
		return fmt.Errorf("config: read %q: %w", l.path, err)
	}

	var cfg AllConfig
	// Apply defaults before unmarshalling so missing YAML fields get defaults.
	cfg.Scene = DefaultSceneConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("config: parse %q: %w", l.path, err)
	}

	l.ptr.Store(&cfg)
	return nil
}

// WatchSIGHUP starts a goroutine that calls Reload() whenever SIGHUP is received.
// Equivalent to C++ ISoGame::ReloadCfg triggered by the host process.
// The goroutine exits when the process dies.
func (l *Loader) WatchSIGHUP() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
			if err := l.Reload(); err != nil {
				// Log and continue — don't crash on a bad config file.
				fmt.Fprintf(os.Stderr, "config: SIGHUP reload error: %v\n", err)
			}
		}
	}()
}

// unsafePointerSize checks that atomic.Pointer works as expected (compile-time sanity).
var _ = unsafe.Sizeof(atomic.Pointer[AllConfig]{})
