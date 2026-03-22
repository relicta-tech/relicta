package reputation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	// maxHistoryPerActor is the maximum number of historical scores kept per actor.
	maxHistoryPerActor = 100
)

// fileStoreData is the on-disk JSON structure.
type fileStoreData struct {
	Scores  map[string]*Score   `json:"scores"`
	History map[string][]Score  `json:"history"`
}

// FileStore implements ReputationStore backed by a JSON file.
type FileStore struct {
	mu   sync.RWMutex
	path string
	data fileStoreData
}

// NewFileStore creates a new file-backed reputation store.
// The file is created if it does not exist.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("directory path is required")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating reputation directory: %w", err)
	}

	path := filepath.Join(dir, "scores.json")

	fs := &FileStore{
		path: path,
		data: fileStoreData{
			Scores:  make(map[string]*Score),
			History: make(map[string][]Score),
		},
	}

	// Load existing data if the file exists.
	if _, err := os.Stat(path); err == nil {
		if err := fs.load(); err != nil {
			return nil, fmt.Errorf("loading reputation data: %w", err)
		}
	}

	return fs, nil
}

// GetScore returns the current reputation score for an actor.
func (fs *FileStore) GetScore(_ context.Context, actorID string) (*Score, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	score, exists := fs.data.Scores[actorID]
	if !exists {
		return nil, fmt.Errorf("no reputation score found for actor: %s", actorID)
	}

	// Return a copy.
	scoreCopy := *score
	return &scoreCopy, nil
}

// SaveScore persists a reputation score for an actor and appends to history.
func (fs *FileStore) SaveScore(_ context.Context, actorID string, score *Score) error {
	if score == nil {
		return fmt.Errorf("score is required")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Save current score.
	scoreCopy := *score
	fs.data.Scores[actorID] = &scoreCopy

	// Append to history.
	fs.data.History[actorID] = append(fs.data.History[actorID], scoreCopy)

	// Trim history to max entries.
	if len(fs.data.History[actorID]) > maxHistoryPerActor {
		excess := len(fs.data.History[actorID]) - maxHistoryPerActor
		fs.data.History[actorID] = fs.data.History[actorID][excess:]
	}

	return fs.saveLocked()
}

// GetHistory returns historical scores for an actor, most recent first.
func (fs *FileStore) GetHistory(_ context.Context, actorID string, limit int) ([]Score, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	history := fs.data.History[actorID]
	if len(history) == 0 {
		return []Score{}, nil
	}

	// Return most recent first, up to limit.
	count := limit
	if count > len(history) {
		count = len(history)
	}

	result := make([]Score, count)
	for i := 0; i < count; i++ {
		result[i] = history[len(history)-1-i]
	}

	return result, nil
}

// load reads the JSON file into memory.
func (fs *FileStore) load() error {
	raw, err := os.ReadFile(fs.path)
	if err != nil {
		return fmt.Errorf("reading reputation file: %w", err)
	}

	if len(raw) == 0 {
		return nil
	}

	if err := json.Unmarshal(raw, &fs.data); err != nil {
		return fmt.Errorf("unmarshaling reputation data: %w", err)
	}

	// Ensure maps are initialized.
	if fs.data.Scores == nil {
		fs.data.Scores = make(map[string]*Score)
	}
	if fs.data.History == nil {
		fs.data.History = make(map[string][]Score)
	}

	return nil
}

// saveLocked writes the current state to disk. Must be called with mu held.
func (fs *FileStore) saveLocked() error {
	raw, err := json.MarshalIndent(fs.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling reputation data: %w", err)
	}

	if err := os.WriteFile(fs.path, raw, 0o644); err != nil {
		return fmt.Errorf("writing reputation file: %w", err)
	}

	return nil
}
