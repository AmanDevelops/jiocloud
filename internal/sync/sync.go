// Package sync implements one-way (local -> remote) folder synchronisation
// against the JioAiCloud API: it mirrors the local directory tree as remote
// folders and uploads files that are missing or changed.
package sync

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AmanDevelops/jiocloud/internal/api"
)

// API is the subset of *api.Client the syncer needs (kept small for testing).
type API interface {
	UserInfo() (*api.UserInfo, error)
	ListFolder(folderKey string) ([]api.Object, error)
	CreateFolder(name, parentKey string) (string, error)
	Upload(path, folderKey string) (*api.UploadResult, error)
}

// Syncer carries the state for a single sync run.
type Syncer struct {
	client   API
	state    *State
	dryRun   bool
	listings map[string][]api.Object // folderKey -> children, cached for this run

	uploaded  int
	skipped   int
	created   int
	bytesSent int64
}

// Run performs a one-way sync of srcDir into the remote folder at remotePath
// (a slash-separated path relative to the user's root; "" means the root).
func Run(client API, srcDir, remotePath string, dryRun bool) error {
	abs, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", abs)
	}

	user, err := client.UserInfo()
	if err != nil {
		return err
	}

	state, err := loadState(abs, remotePath)
	if err != nil {
		return err
	}
	state.Root = user.RootFolderKey

	s := &Syncer{
		client:   client,
		state:    state,
		dryRun:   dryRun,
		listings: map[string][]api.Object{},
	}

	fmt.Printf("Syncing %s -> /%s (root %s)\n", abs, remotePath, user.RootFolderKey)

	baseKey := user.RootFolderKey
	if remotePath != "" {
		baseKey, err = s.ensurePath(user.RootFolderKey, splitPath(remotePath))
		if err != nil {
			return err
		}
	}
	s.state.Folders[""] = baseKey

	if err := s.syncDir(abs, baseKey, ""); err != nil {
		// Persist whatever progress we made before returning the error.
		_ = s.state.save()
		return err
	}

	if err := s.state.save(); err != nil {
		return fmt.Errorf("saving sync state: %w", err)
	}

	fmt.Printf("Done. %d uploaded (%s), %d skipped, %d folders created.\n",
		s.uploaded, humanBytes(s.bytesSent), s.skipped, s.created)
	return nil
}

// ensurePath walks/creates each folder segment, returning the final folder key.
func (s *Syncer) ensurePath(parentKey string, segments []string) (string, error) {
	key := parentKey
	rel := ""
	for _, seg := range segments {
		var err error
		key, err = s.ensureFolder(key, seg)
		if err != nil {
			return "", err
		}
		rel = path(rel, seg)
		s.state.Folders[rel] = key
	}
	return key, nil
}

// syncDir recursively syncs localDir into the remote folder remoteKey. rel is the
// remote path relative to the sync base, used as the state map key.
func (s *Syncer) syncDir(localDir, remoteKey, rel string) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	remote, err := s.getListing(remoteKey)
	if err != nil {
		return err
	}
	remoteFiles := map[string]string{} // name -> hash
	for _, o := range remote {
		if o.ObjectType == api.TypeFile {
			remoteFiles[o.ObjectName] = o.Hash
		}
	}

	for _, e := range entries {
		name := e.Name()
		localPath := filepath.Join(localDir, name)
		childRel := path(rel, name)

		info, err := e.Info()
		if err != nil {
			return err
		}
		// Skip symlinks and other non-regular, non-dir entries.
		if info.Mode()&os.ModeSymlink != 0 {
			fmt.Printf("  skip (symlink) %s\n", childRel)
			continue
		}

		if e.IsDir() {
			childKey, err := s.ensureFolder(remoteKey, name)
			if err != nil {
				return err
			}
			s.state.Folders[childRel] = childKey
			if err := s.syncDir(localPath, childKey, childRel); err != nil {
				return err
			}
			continue
		}

		if !info.Mode().IsRegular() {
			fmt.Printf("  skip (not a regular file) %s\n", childRel)
			continue
		}

		hash, err := md5File(localPath)
		if err != nil {
			return err
		}
		if remoteFiles[name] == hash {
			s.skipped++
			s.state.Files[childRel] = hash
			fmt.Printf("  = %s\n", childRel)
			continue
		}

		if s.dryRun {
			fmt.Printf("  + %s (%s) [dry-run]\n", childRel, humanBytes(info.Size()))
			s.uploaded++
			s.bytesSent += info.Size()
			continue
		}

		fmt.Printf("  + %s (%s)\n", childRel, humanBytes(info.Size()))
		if _, err := s.client.Upload(localPath, remoteKey); err != nil {
			return fmt.Errorf("uploading %s: %w", childRel, err)
		}
		s.uploaded++
		s.bytesSent += info.Size()
		s.state.Files[childRel] = hash
	}
	return nil
}

// ensureFolder returns the key of the named child folder of parentKey, creating
// it if absent. Results are reflected in the cached listing.
func (s *Syncer) ensureFolder(parentKey, name string) (string, error) {
	listing, err := s.getListing(parentKey)
	if err != nil {
		return "", err
	}
	for _, o := range listing {
		if o.ObjectType == api.TypeFolder && o.ObjectName == name {
			return o.ObjectKey, nil
		}
	}

	if s.dryRun {
		fmt.Printf("  mkdir %s [dry-run]\n", name)
		s.created++
		// Use a placeholder key so dry-run recursion can proceed.
		key := "DRYRUN-" + name
		s.listings[parentKey] = append(s.listings[parentKey], api.Object{ObjectKey: key, ObjectType: api.TypeFolder, ObjectName: name})
		return key, nil
	}

	key, err := s.client.CreateFolder(name, parentKey)
	if err != nil {
		return "", err
	}
	s.created++
	s.listings[parentKey] = append(s.listings[parentKey], api.Object{ObjectKey: key, ObjectType: api.TypeFolder, ObjectName: name})
	return key, nil
}

// getListing returns the cached children of folderKey, fetching once per run.
func (s *Syncer) getListing(folderKey string) ([]api.Object, error) {
	if v, ok := s.listings[folderKey]; ok {
		return v, nil
	}
	objs, err := s.client.ListFolder(folderKey)
	if err != nil {
		return nil, err
	}
	s.listings[folderKey] = objs
	return objs, nil
}

func md5File(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// splitPath splits a slash-separated remote path into clean segments.
func splitPath(p string) []string {
	var out []string
	for _, seg := range strings.Split(p, "/") {
		seg = strings.TrimSpace(seg)
		if seg != "" && seg != "." {
			out = append(out, seg)
		}
	}
	return out
}

// path joins a relative parent and a segment with "/".
func path(parent, seg string) string {
	if parent == "" {
		return seg
	}
	return parent + "/" + seg
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
