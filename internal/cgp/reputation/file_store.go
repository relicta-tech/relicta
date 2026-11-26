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
	Scores  map[string]*Score  `json:"scores"`
	History map[string][]Score `json:"history"`
}

// FileStore implements ReputationStore backed by a JSON file.
type FileStore struct {
	mu   sync.RWMutex
	path string
	data fileStoreData
}

// NewFileStore creates a new file-backed reputation store. Scores are written
// on the first SaveScore call.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("directory path is required")
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
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
	if actorID == "" {
		return fmt.Errorf("actor ID is required")
	}
	if score == nil {
		return fmt.Errorf("score is required")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	oldScore, hadScore := fs.data.Scores[actorID]
	oldHistory := append([]Score(nil), fs.data.History[actorID]...)

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

	if err := fs.saveLocked(); err != nil {
		if hadScore {
			fs.data.Scores[actorID] = oldScore
		} else {
			delete(fs.data.Scores, actorID)
		}
		if len(oldHistory) == 0 {
			delete(fs.data.History, actorID)
		} else {
			fs.data.History[actorID] = oldHistory
		}
		return err
	}
	return nil
}

// GetHistory returns historical scores for an actor, most recent first.
func (fs *FileStore) GetHistory(_ context.Context, actorID string, limit int) ([]Score, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	history := fs.data.History[actorID]
	if len(history) == 0 || limit <= 0 {
		return []Score{}, nil
	}

	// Return most recent first, up to limit. A non-positive limit is an
	// explicit empty page; in particular, a negative value must not reach make.
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

	tmp, err := os.CreateTemp(filepath.Dir(fs.path), ".scores-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary reputation file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("setting reputation file permissions: %w", err)
	}
	if _, err := tmp.Write(raw); err != nil {
		return fmt.Errorf("writing temporary reputation file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing temporary reputation file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temporary reputation file: %w", err)
	}
	if err := os.Rename(tmpPath, fs.path); err != nil {
		return fmt.Errorf("committing reputation file: %w", err)
	}
	committed = true

	return nil
}
