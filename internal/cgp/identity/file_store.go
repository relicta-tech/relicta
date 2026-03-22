package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// defaultActorsFile is the filename used for persisting actor identities.
const defaultActorsFile = "actors.json"

// FileStore implements RegistryStore by persisting actor identities
// as JSON in a file on disk. It is safe for concurrent use.
type FileStore struct {
	mu       sync.Mutex
	basePath string
	filePath string
}

// fileStoreData is the on-disk JSON structure.
type fileStoreData struct {
	Actors []*ActorIdentity `json:"actors"`
}

// NewFileStore creates a new file-based registry store.
// It creates the directory at basePath if it does not exist.
// The actors are stored in basePath/actors.json.
func NewFileStore(basePath string) (*FileStore, error) {
	if basePath == "" {
		return nil, fmt.Errorf("base path is required")
	}

	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("creating directory %s: %w", basePath, err)
	}

	return &FileStore{
		basePath: basePath,
		filePath: filepath.Join(basePath, defaultActorsFile),
	}, nil
}

// LoadAll reads all actor identities from the JSON file.
// Returns an empty slice if the file does not exist yet.
func (s *FileStore) LoadAll(ctx context.Context) ([]*ActorIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*ActorIdentity{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", s.filePath, err)
	}

	if len(data) == 0 {
		return []*ActorIdentity{}, nil
	}

	var storeData fileStoreData
	if err := json.Unmarshal(data, &storeData); err != nil {
		return nil, fmt.Errorf("unmarshaling actors from %s: %w", s.filePath, err)
	}

	if storeData.Actors == nil {
		return []*ActorIdentity{}, nil
	}

	return storeData.Actors, nil
}

// Save persists an actor identity. If the actor already exists (by ID),
// it is replaced; otherwise it is appended.
func (s *FileStore) Save(ctx context.Context, actor *ActorIdentity) error {
	if actor == nil {
		return fmt.Errorf("actor is required")
	}
	if actor.ID == "" {
		return fmt.Errorf("actor ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	actors, err := s.loadLocked()
	if err != nil {
		return err
	}

	// Replace existing or append.
	found := false
	for i, a := range actors {
		if a.ID == actor.ID {
			actors[i] = actor
			found = true
			break
		}
	}
	if !found {
		actors = append(actors, actor)
	}

	return s.saveLocked(actors)
}

// Delete removes an actor identity by ID. Returns an error if the actor
// is not found.
func (s *FileStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("actor ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	actors, err := s.loadLocked()
	if err != nil {
		return err
	}

	found := false
	filtered := make([]*ActorIdentity, 0, len(actors))
	for _, a := range actors {
		if a.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, a)
	}

	if !found {
		return fmt.Errorf("actor not found: %s", id)
	}

	return s.saveLocked(filtered)
}

// loadLocked reads actors from disk. Must be called with mu held.
func (s *FileStore) loadLocked() ([]*ActorIdentity, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*ActorIdentity{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", s.filePath, err)
	}

	if len(data) == 0 {
		return []*ActorIdentity{}, nil
	}

	var storeData fileStoreData
	if err := json.Unmarshal(data, &storeData); err != nil {
		return nil, fmt.Errorf("unmarshaling actors: %w", err)
	}

	if storeData.Actors == nil {
		return []*ActorIdentity{}, nil
	}

	return storeData.Actors, nil
}

// saveLocked writes actors to disk atomically. Must be called with mu held.
func (s *FileStore) saveLocked(actors []*ActorIdentity) error {
	storeData := fileStoreData{Actors: actors}

	data, err := json.MarshalIndent(storeData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling actors: %w", err)
	}

	// Write atomically via temp file + rename.
	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("writing temp file %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		// Clean up temp file on rename failure.
		_ = os.Remove(tmpFile)
		return fmt.Errorf("renaming %s to %s: %w", tmpFile, s.filePath, err)
	}

	return nil
}
