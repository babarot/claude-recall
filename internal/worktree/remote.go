package worktree

import (
	"bufio"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Remote is the origin of the git repository dir is in, as NormalizeRemote
// writes it, read from the repository's config without running git. ok is
// false when it cannot be known: dir is not on disk, or its git config
// cannot be read. Outside git, or with no origin, it is "" and ok.
//
// The config's include and includeIf are not followed, nor is a
// url.<base>.insteadOf applied.
func Remote(dir string) (remote string, ok bool) {
	if dir == "" {
		return "", false
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "", false
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		real = dir
	}
	for d := real; ; d = filepath.Dir(d) {
		git := filepath.Join(d, ".git")
		fi, err := os.Lstat(git)
		if err == nil {
			config, found := configOf(git, d, fi.Mode().IsRegular())
			if !found {
				return "", false
			}
			u, ok := originURL(config)
			if !ok {
				return "", false
			}
			return NormalizeRemote(u), true
		}
		if parent := filepath.Dir(d); parent == d {
			return "", true
		}
	}
}

// configOf is the config file of the repository whose .git is git, in the
// checkout root. A .git file points at the git dir; a linked worktree's
// shares the config of the common dir its commondir names.
func configOf(git, root string, isFile bool) (string, bool) {
	if !isFile {
		return filepath.Join(git, "config"), true
	}
	b, err := os.ReadFile(git)
	if err != nil {
		return "", false
	}
	gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return "", false
	}
	gitdir = strings.TrimSpace(gitdir)
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(root, gitdir)
	}
	if c, err := os.ReadFile(filepath.Join(gitdir, "commondir")); err == nil {
		common := strings.TrimSpace(string(c))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitdir, common)
		}
		return filepath.Join(filepath.Clean(common), "config"), true
	}
	return filepath.Join(gitdir, "config"), true
}

var (
	sectionLine = regexp.MustCompile(`^\[\s*([A-Za-z0-9.-]+)(?:\s+"((?:[^"\\]|\\.)*)")?\s*\]`)
	keyLine     = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*)\s*(?:=\s*(.*))?$`)
)

// originURL is the first url of [remote "origin"] in the git config at
// path: "" when there is none, and false when the file cannot be read.
func originURL(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	inOrigin := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if m := sectionLine.FindStringSubmatch(line); m != nil {
			inOrigin = strings.EqualFold(m[1], "remote") && m[2] == "origin"
			line = strings.TrimSpace(line[len(m[0]):])
			if line == "" {
				continue
			}
		}
		if !inOrigin {
			continue
		}
		if m := keyLine.FindStringSubmatch(line); m != nil && strings.EqualFold(m[1], "url") {
			return configValue(m[2]), true
		}
	}
	return "", sc.Err() == nil
}

// configValue reads a git config value: quotes are dropped, a # or ;
// outside them starts a comment, and \" \\ \t \n are escapes.
func configValue(s string) string {
	var b strings.Builder
	quoted := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			quoted = !quoted
		case c == '\\' && i+1 < len(s):
			i++
			switch s[i] {
			case 't':
				b.WriteByte('\t')
			case 'n':
				b.WriteByte('\n')
			default:
				b.WriteByte(s[i])
			}
		case (c == '#' || c == ';') && !quoted:
			return strings.TrimSpace(b.String())
		default:
			b.WriteByte(c)
		}
	}
	return strings.TrimSpace(b.String())
}

var scpLike = regexp.MustCompile(`^(?:[^@/]+@)?([^:/]+):(.+)$`)

// NormalizeRemote turns a git remote URL into host/path, the same for the
// ways of writing one: https://github.com/owner/repo.git,
// git@github.com:owner/repo and ssh://git@github.com/owner/repo are all
// github.com/owner/repo, and so is ssh.github.com, GitHub's SSH over
// port 443. A user, a password or token, and a port are
// dropped. A local path is kept as it is, cleaned.
func NormalizeRemote(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var host, path string
	switch {
	case strings.Contains(raw, "://"):
		u, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		if u.Scheme == "file" {
			return filepath.Clean(u.Path)
		}
		host, path = u.Hostname(), u.Path
	case filepath.IsAbs(raw) || strings.HasPrefix(raw, "."):
		return filepath.Clean(raw)
	default:
		m := scpLike.FindStringSubmatch(raw)
		if m == nil {
			return filepath.Clean(raw)
		}
		host, path = m[1], m[2]
	}
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")
	host = strings.ToLower(host)
	if host == "ssh.github.com" {
		// GitHub's SSH over port 443, the same repositories.
		host = "github.com"
	}
	if host == "" {
		return path
	}
	if path == "" {
		return host
	}
	return host + "/" + path
}
