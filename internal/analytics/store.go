package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/internal/fileutil"
)

// MaxEventFileSize is the maximum allowed size for analytics event files (1MB).
const MaxEventFileSize = 1 << 20

// Store defines the interface for persisting and querying analytics events.
type Store interface {
	// Append persists a new analytics event.
	Append(ctx context.Context, event Event) error
	// Query returns events matching the given filter, ordered by timestamp ascending.
	Query(ctx context.Context, filter QueryFilter) ([]Event, error)
}

// QueryFilter specifies criteria for querying analytics events.
type QueryFilter struct {
	From      *time.Time
	To        *time.Time
	EventType *EventType
	ReleaseID string
}

// FileStore implements Store using a file-per-day storage strategy
// under a base directory (typically .relicta/analytics/).
type FileStore struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStore creates a new file-based analytics store.
func NewFileStore(basePath string) (*FileStore, error) {
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create analytics directory: %w", err)
	}
	return &FileStore{basePath: basePath}, nil
}

// dayFileName returns the filename for a given date's events.
func dayFileName(t time.Time) string {
	return t.UTC().Format("2006-01-02") + ".jsonl"
}

// Append persists an analytics event into the day file for its timestamp.
func (s *FileStore) Append(_ context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	data = append(data, '\n')

	filePath := filepath.Join(s.basePath, dayFileName(event.Timestamp))
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to open analytics file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}

// Query scans day files within the date range and returns matching events.
func (s *FileStore) Query(_ context.Context, filter QueryFilter) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read analytics directory: %w", err)
	}

	var events []Event
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}

		// Parse date from filename to skip files outside range
		dateStr := entry.Name()[:len(entry.Name())-len(".jsonl")]
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue // skip non-date files
		}

		if filter.From != nil && fileDate.Before(filter.From.Truncate(24*time.Hour)) {
			continue
		}
		if filter.To != nil && fileDate.After(filter.To.Truncate(24*time.Hour)) {
			continue
		}

		fileEvents, err := s.readDayFile(entry.Name(), filter)
		if err != nil {
			continue // skip corrupt files
		}
		events = append(events, fileEvents...)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events, nil
}

// readDayFile reads events from a single day file and applies the filter.
func (s *FileStore) readDayFile(filename string, filter QueryFilter) ([]Event, error) {
	filePath := filepath.Join(s.basePath, filename)
	data, err := fileutil.ReadFileLimited(filePath, MaxEventFileSize)
	if err != nil {
		return nil, err
	}

	var events []Event
	// Parse JSONL: each line is a JSON event
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			line := data[start:i]
			start = i + 1
			if len(line) == 0 {
				continue
			}
			var event Event
			if err := json.Unmarshal(line, &event); err != nil {
				continue
			}
			if matchesFilter(event, filter) {
				events = append(events, event)
			}
		}
	}
	// Handle last line without trailing newline
	if start < len(data) {
		line := data[start:]
		var event Event
		if err := json.Unmarshal(line, &event); err == nil {
			if matchesFilter(event, filter) {
				events = append(events, event)
			}
		}
	}

	return events, nil
}

// matchesFilter returns true if the event matches all filter criteria.
func matchesFilter(event Event, filter QueryFilter) bool {
	if filter.From != nil && event.Timestamp.Before(*filter.From) {
		return false
	}
	if filter.To != nil && event.Timestamp.After(*filter.To) {
		return false
	}
	if filter.EventType != nil && event.Type != *filter.EventType {
		return false
	}
	if filter.ReleaseID != "" && event.ReleaseID != filter.ReleaseID {
		return false
	}
	return true
}
