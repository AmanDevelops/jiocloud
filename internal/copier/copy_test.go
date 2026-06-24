package copier

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/AmanDevelops/jiocloud/internal/api"
)

// fakeAPI is an in-memory stand-in for the JioAiCloud client.
type fakeAPI struct {
	folders map[string][]api.Object // folderKey -> children
	nextKey int
	uploads []string // folderKey/name of uploaded files
	created []string // names of created folders
	trashed []string // names of trashed objects
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{folders: map[string][]api.Object{"root": nil}}
}

func (f *fakeAPI) UserInfo() (*api.UserInfo, error) {
	return &api.UserInfo{UserID: "u", RootFolderKey: "root"}, nil
}

func (f *fakeAPI) ListFolder(folderKey string) ([]api.Object, error) {
	return f.folders[folderKey], nil
}

func (f *fakeAPI) CreateFolder(name, parentKey string) (string, error) {
	f.nextKey++
	key := "k" + string(rune('0'+f.nextKey))
	f.folders[parentKey] = append(f.folders[parentKey], api.Object{
		ObjectKey: key, ObjectType: api.TypeFolder, ObjectName: name,
	})
	f.folders[key] = nil
	f.created = append(f.created, name)
	return key, nil
}

func (f *fakeAPI) Upload(path, folderKey string) (*api.UploadResult, error) {
	f.uploads = append(f.uploads, folderKey+"/"+filepath.Base(path))
	return &api.UploadResult{ObjectName: filepath.Base(path)}, nil
}

func (f *fakeAPI) Trash(obj api.Object) error {
	f.trashed = append(f.trashed, obj.ObjectName)
	// For testing, just remove it from the parent folder's listing
	if obj.ParentObjectKey != "" {
		var filtered []api.Object
		for _, o := range f.folders[obj.ParentObjectKey] {
			if o.ObjectKey != obj.ObjectKey {
				filtered = append(filtered, o)
			}
		}
		f.folders[obj.ParentObjectKey] = filtered
	} else {
		// If parent is empty, remove it from everywhere it could be (for simplicity)
		for parent, children := range f.folders {
			var filtered []api.Object
			for _, o := range children {
				if o.ObjectKey != obj.ObjectKey {
					filtered = append(filtered, o)
				}
			}
			f.folders[parent] = filtered
		}
	}
	return nil
}

func md5str(b []byte) string {
	s := md5.Sum(b)
	return hex.EncodeToString(s[:])
}

func TestCopyCreatesFoldersAndUploads(t *testing.T) {
	// Use a temp config dir so state writes don't touch the real home.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "a.txt"), "hello")
	mustWrite(t, filepath.Join(src, "sub", "b.txt"), "world")

	f := newFakeAPI()
	if err := Run(f, src, "", false, false); err != nil {
		t.Fatal(err)
	}

	if len(f.created) != 1 || f.created[0] != "sub" {
		t.Errorf("created folders = %v, want [sub]", f.created)
	}
	if len(f.uploads) != 2 {
		t.Fatalf("uploads = %v, want 2", f.uploads)
	}
}

func TestCopySkipsUnchanged(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "a.txt"), "hello")

	f := newFakeAPI()
	// Pretend a.txt with matching md5 already exists in the root.
	f.folders["root"] = []api.Object{
		{ObjectKey: "x", ObjectType: api.TypeFile, ObjectName: "a.txt", Hash: md5str([]byte("hello"))},
	}

	if err := Run(f, src, "", false, false); err != nil {
		t.Fatal(err)
	}
	if len(f.uploads) != 0 {
		t.Errorf("uploads = %v, want none (file unchanged)", f.uploads)
	}
}

func TestSyncDeletesExtraneousRemote(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "keep.txt"), "hello")

	f := newFakeAPI()
	// Remote root already has the matching file plus an extraneous one.
	f.folders["root"] = []api.Object{
		{ObjectKey: "k", ObjectType: api.TypeFile, ObjectName: "keep.txt", Hash: md5str([]byte("hello")), ParentObjectKey: "root"},
		{ObjectKey: "g", ObjectType: api.TypeFile, ObjectName: "gone.txt", Hash: md5str([]byte("stale")), ParentObjectKey: "root"},
	}

	// deleteExtraneous = true (sync semantics).
	if err := Run(f, src, "", false, true); err != nil {
		t.Fatal(err)
	}

	if len(f.trashed) != 1 || f.trashed[0] != "gone.txt" {
		t.Errorf("trashed = %v, want [gone.txt]", f.trashed)
	}
	if len(f.uploads) != 0 {
		t.Errorf("uploads = %v, want none (keep.txt unchanged)", f.uploads)
	}
}

func TestCopyDoesNotDeleteExtraneousRemote(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "keep.txt"), "hello")

	f := newFakeAPI()
	f.folders["root"] = []api.Object{
		{ObjectKey: "k", ObjectType: api.TypeFile, ObjectName: "keep.txt", Hash: md5str([]byte("hello")), ParentObjectKey: "root"},
		{ObjectKey: "g", ObjectType: api.TypeFile, ObjectName: "gone.txt", Hash: md5str([]byte("stale")), ParentObjectKey: "root"},
	}

	// deleteExtraneous = false (copy semantics): remote-only files are left alone.
	if err := Run(f, src, "", false, false); err != nil {
		t.Fatal(err)
	}

	if len(f.trashed) != 0 {
		t.Errorf("trashed = %v, want none (copy must not delete)", f.trashed)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
