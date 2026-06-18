package copier

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// State is the persisted record of a copy target: the remote folder keys created
// for each relative path, and the md5 hashes of files already uploaded. It lets
// re-runs skip work and satisfies "save folder id somewhere".
type State struct {
	Source string `json:"source"` // absolute local source dir
	Remote string `json:"remote"` // remote base path (relative to root)
	Root   string `json:"rootFolderKey"`

	// Folders maps a remote-relative path ("" = base) to its folder object key.
	Folders map[string]string `json:"folders"`
	// Files maps a remote-relative file path to its last-uploaded md5 hash.
	Files map[string]string `json:"files"`
}

func newState(source, remote string) *State {
	return &State{
		Source:  source,
		Remote:  remote,
		Folders: map[string]string{},
		Files:   map[string]string{},
	}
}

// statePath returns the on-disk location for a given source dir's state.
func statePath(source string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(source))
	name := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(dir, "jiocloud", "copy", name), nil
}

// loadState reads existing state for source, or returns a fresh one.
func loadState(source, remote string) (*State, error) {
	p, err := statePath(source)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return newState(source, remote), nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Folders == nil {
		s.Folders = map[string]string{}
	}
	if s.Files == nil {
		s.Files = map[string]string{}
	}
	s.Remote = remote
	return &s, nil
}

// save persists the state to disk.
func (s *State) save() error {
	p, err := statePath(s.Source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
