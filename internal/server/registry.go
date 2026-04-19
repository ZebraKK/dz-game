// Package server implements the HTTP JSON service layer for gamesvr.
// It routes incoming JSON requests to the appropriate Table actor via its inbox channel.
package server

import (
	"fmt"
	"sync"

	"github.com/qbgames/qbdz-game/internal/table"
)

// Registry manages the set of live Table actors, keyed by table ID.
// It is safe for concurrent use.
type Registry struct {
	mu     sync.RWMutex
	tables map[uint32]*table.Table
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		tables: make(map[uint32]*table.Table),
	}
}

// Add registers a table. Returns an error if the table ID is already in use.
func (r *Registry) Add(id uint32, t *table.Table) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tables[id]; exists {
		return fmt.Errorf("table %d already registered", id)
	}
	r.tables[id] = t
	return nil
}

// Remove unregisters and returns the table with the given ID.
// Returns nil if the table was not found.
func (r *Registry) Remove(id uint32) *table.Table {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.tables[id]
	delete(r.tables, id)
	return t
}

// Get looks up a table by ID. Returns (table, true) on success.
func (r *Registry) Get(id uint32) (*table.Table, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tables[id]
	return t, ok
}

// Len returns the number of registered tables.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tables)
}

// ForEach calls fn for every registered table while holding the read lock.
// fn must not call any Registry methods (would deadlock).
func (r *Registry) ForEach(fn func(id uint32, t *table.Table)) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for id, t := range r.tables {
		fn(id, t)
	}
}
