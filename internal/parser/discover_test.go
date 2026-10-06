package parser

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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
