package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"openailogger/storage"
)

// Store implements an in-memory storage backend
type Store struct {
	mu      sync.RWMutex
	records map[string]*storage.Record
}

// New creates a new in-memory store
func New() *Store {
	return &Store{
		records: make(map[string]*storage.Record),
	}
}

// Save stores a record in memory
func (s *Store) Save(ctx context.Context, r *storage.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a copy to avoid external modifications
	record := *r
	s.records[r.ID] = &record
	return nil
}

// Get retrieves a record by ID
func (s *Store) Get(ctx context.Context, id string) (*storage.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[id]
	if !exists {
		return nil, fmt.Errorf("record not found: %s", id)
	}

	// Return a copy to avoid external modifications
	result := *record
	return &result, nil
}

// List retrieves records matching the query
func (s *Store) List(ctx context.Context, q storage.Query) ([]storage.Record, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []*storage.Record

	// Filter records
	for _, record := range s.records {
		if s.matchesQuery(record, q) {
			matches = append(matches, record)
		}
	}

	// Sort records
	s.sortRecords(matches, q.Sort)

	total := len(matches)

	// Apply pagination
	start := q.Offset
	if start > len(matches) {
		start = len(matches)
	}

	end := start + q.Limit
	if q.Limit <= 0 || end > len(matches) {
		end = len(matches)
	}

	result := make([]storage.Record, end-start)
	for i, record := range matches[start:end] {
		result[i] = *record // Copy to avoid external modifications
	}

	return result, total, nil
}

// Delete removes a record by ID
func (s *Store) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.records[id]; !exists {
		return fmt.Errorf("record not found: %s", id)
	}

	delete(s.records, id)
	return nil
}

// ExportNDJSON exports records as newline-delimited JSON
func (s *Store) ExportNDJSON(ctx context.Context, q storage.Query) (io.ReadCloser, error) {
	records, _, err := s.List(ctx, q)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return nil, fmt.Errorf("failed to encode record: %w", err)
		}
	}

	return io.NopCloser(&buf), nil
}

// Close closes the store (no-op for memory store)
func (s *Store) Close() error {
	return nil
}

// matchesQuery checks if a record matches the query filters
func (s *Store) matchesQuery(record *storage.Record, q storage.Query) bool {
	if q.Provider != nil && record.Provider != *q.Provider {
		return false
	}

	if q.StatusEq != nil && record.Status != *q.StatusEq {
		return false
	}

	if q.From != nil && record.Timestamp.Before(*q.From) {
		return false
	}

	if q.To != nil && record.Timestamp.After(*q.To) {
		return false
	}

	if q.ModelLike != nil && !strings.Contains(strings.ToLower(record.ModelHint), strings.ToLower(*q.ModelLike)) {
		return false
	}

	if q.URLLike != nil && !strings.Contains(strings.ToLower(record.URL), strings.ToLower(*q.URLLike)) {
		return false
	}

	if q.TextSearch != nil {
		searchTerm := strings.ToLower(*q.TextSearch)
		searchableText := strings.ToLower(record.RequestBody + " " + record.ResponseBody + " " + record.URL + " " + record.ModelHint)
		if !strings.Contains(searchableText, searchTerm) {
			return false
		}
	}

	return true
}

// sortRecords sorts records based on the sort parameter
func (s *Store) sortRecords(records []*storage.Record, sortBy string) {
	switch sortBy {
	case "-ts":
		sort.Slice(records, func(i, j int) bool {
			return records[i].Timestamp.After(records[j].Timestamp)
		})
	case "ts":
		fallthrough
	default:
		sort.Slice(records, func(i, j int) bool {
			return records[i].Timestamp.Before(records[j].Timestamp)
		})
	}
}
