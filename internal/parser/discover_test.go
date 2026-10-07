package parser

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// put writes tree/project/id.jsonl and returns its path.
func put(t *testing.T, tree, project, id string) string {
	t.Helper()
	dir := filepath.Join(tree, project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, id+".jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func ids(files []File) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Dir+"|"+f.Project+"|"+f.SessionID)
	}
	slices.Sort(out)
	return out
}

func TestDiscoverTrees(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	put(t, a, "-p", "s1")
	put(t, b, "-q", "s2")
	os.WriteFile(filepath.Join(a, "-p", "notes.txt"), nil, 0o644)
	os.WriteFile(filepath.Join(a, "stray.jsonl"), nil, 0o644) // not in a project directory
	got := ids(Discover(a, b, filepath.Join(a, "missing")))
	want := []string{a + "|-p|s1", b + "|-q|s2"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A symlinked project directory, transcript or tree is read as if it were
// there, and a file reached twice is listed once, under its first path.
func TestDiscoverFollowsSymlinks(t *testing.T) {
	elsewhere := t.TempDir()
	put(t, elsewhere, "-linked", "s1")
	file := put(t, elsewhere, "-files", "s2")

	tree := t.TempDir()
	if err := os.Symlink(filepath.Join(elsewhere, "-linked"), filepath.Join(tree, "-linked")); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(tree, "-own"), 0o755)
	os.Symlink(file, filepath.Join(tree, "-own", "s2.jsonl"))
	os.Symlink(filepath.Join(elsewhere, "nowhere"), filepath.Join(tree, "-own", "s3.jsonl")) // dangling

	root := filepath.Join(t.TempDir(), "root")
	os.Symlink(tree, root)

	got := ids(Discover(root))
	want := []string{root + "|-linked|s1", root + "|-own|s2"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// elsewhere holds the same two files: listed once, where first reached.
	got = ids(Discover(root, elsewhere))
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestChoose(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := func(dir, id string, mtime time.Duration, size int64) File {
		return File{Dir: dir, SessionID: id, ModTime: at.Add(mtime), Size: size}
	}
	files := []File{
		f("primary", "older-here", 0, 10), f("primary", "newer-here", time.Minute, 10),
		f("primary", "tie-size", 0, 10), f("primary", "tie", 0, 10), f("primary", "alone", 0, 1),
		f("extra", "older-here", time.Minute, 10), f("extra", "newer-here", 0, 10),
		f("extra", "tie-size", 0, 20), f("extra", "tie", 0, 10),
	}
	var got []string
	for _, c := range Choose(files) {
		got = append(got, c.Dir+"|"+c.SessionID)
	}
	want := []string{"primary|newer-here", "primary|tie", "primary|alone", "extra|older-here", "extra|tie-size"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// FindSession lists what Discover lists for the session, in the same
// order and under the same paths, symlinks and second mounts included.
func TestFindSessionMatchesDiscover(t *testing.T) {
	elsewhere := t.TempDir()
	put(t, elsewhere, "-linked", "s1")
	put(t, elsewhere, "-other", "s1")
	file := put(t, elsewhere, "-files", "s2")
	tree := t.TempDir()
	os.Symlink(filepath.Join(elsewhere, "-linked"), filepath.Join(tree, "-linked"))
	os.MkdirAll(filepath.Join(tree, "-own"), 0o755)
	os.Symlink(file, filepath.Join(tree, "-own", "s2.jsonl"))
	put(t, tree, "-own", "s1")
	os.WriteFile(filepath.Join(tree, "s1.jsonl"), nil, 0o644) // not in a project directory

	dirs := []string{tree, elsewhere, filepath.Join(tree, "missing")}
	all := Discover(dirs...)
	for _, id := range []string{"s1", "s2", "s3"} {
		var want []File
		for _, f := range all {
			if f.SessionID == id {
				want = append(want, f)
			}
		}
		if got := FindSession(dirs, id); !slices.Equal(got, want) {
			t.Errorf("FindSession(%s) = %v, want %v", id, got, want)
		}
	}
}
