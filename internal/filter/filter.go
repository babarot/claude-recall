// Package filter is the filter the TUI and the web UI narrow sessions
// with: words to find in a session's title, folder, branch, ID or
// conversation, and key:value terms that look in one field only: folder:
// (in: for short), text: (conversation), title:, branch:, worktree: and id:.
// A new key gets a constant, a field in Query, a line in Query.fields and a
// case in Match.
package filter

import "strings"

// The keys, with their colon.
const (
	FolderKey   = "folder:"
	InKey       = "in:" // folder: for short
	TextKey     = "text:"
	TitleKey    = "title:"
	BranchKey   = "branch:"
	WorktreeKey = "worktree:"
	IDKey       = "id:"
)

// Keys lists the keys in the order they are offered.
var Keys = []string{FolderKey, InKey, TextKey, TitleKey, BranchKey, WorktreeKey, IDKey}

// MinBodyWord is the shortest plain word looked up in the conversation; a
// single letter is in nearly every session.
const MinBodyWord = 2

// Query is the parsed filter, lower-cased.
type Query struct {
	Words []string
	// In are folder: (or in:) name fragments; a session matches when its
	// folder contains any of them. They override the folder the list is
	// narrowed to.
	In []string
	// Text are words to find only in the conversation.
	Text []string
	// Title, Branch and Worktree are parts of those fields, ID the start of
	// the session ID. Several of one key match any of them.
	Title, Branch, Worktree, ID []string
}

// fields maps each key to where its values go.
func (q *Query) fields() map[string]*[]string {
	return map[string]*[]string{
		FolderKey: &q.In, InKey: &q.In, TextKey: &q.Text, TitleKey: &q.Title,
		BranchKey: &q.Branch, WorktreeKey: &q.Worktree, IDKey: &q.ID,
	}
}

// Parse reads a filter. A word with an unknown key, such as a URL, is a
// plain word; a known key with nothing after it is nothing.
func Parse(s string) Query {
	var q Query
	fields := q.fields()
	for w := range strings.FieldsSeq(strings.ToLower(s)) {
		if key, v, ok := strings.Cut(w, ":"); ok {
			if dst, known := fields[key+":"]; known {
				if v != "" {
					*dst = append(*dst, v)
				}
				continue
			}
		}
		q.Words = append(q.Words, w)
	}
	return q
}

// BodyWords are the words of q to look up in the conversation: the plain
// words of at least MinBodyWord letters, and the text: words.
func (q Query) BodyWords() []string {
	var out []string
	for _, w := range q.Words {
		if len([]rune(w)) >= MinBodyWord {
			out = append(out, w)
		}
	}
	return append(out, q.Text...)
}

// InFolder reports whether a folder name fuzzy-matches any folder:
// fragment, as the folder list's search does.
func (q Query) InFolder(name string) bool {
	for _, v := range q.In {
		if _, _, ok := Fuzzy(v, name); ok {
			return true
		}
	}
	return false
}

// Fields are what a session is matched on.
type Fields struct {
	Title, Folder, Worktree, Branch, ID string
	// Search is the lower-cased text a plain word is looked for in: the
	// title, folder, worktree, branch and ID.
	Search string
}

// Match reports whether a session passes the query, the folder scope
// aside. A plain word may be in the session's fields or its conversation;
// a text: word only in the conversation. said reports whether a word was
// found in the session's conversation; a word still being looked up should
// report false, so it matches only the fields until it is found.
func Match(q Query, f Fields, said func(word string) bool) bool {
	if len(q.In) > 0 && !q.InFolder(f.Folder) {
		return false
	}
	in := func(field string) func(string) bool {
		field = strings.ToLower(field)
		return func(v string) bool { return strings.Contains(field, v) }
	}
	if !anyOf(q.Title, in(f.Title)) || !anyOf(q.Branch, in(f.Branch)) || !anyOf(q.Worktree, in(f.Worktree)) ||
		!anyOf(q.ID, func(v string) bool { return strings.HasPrefix(strings.ToLower(f.ID), v) }) {
		return false
	}
	for _, w := range q.Words {
		if !strings.Contains(f.Search, w) && !said(w) {
			return false
		}
	}
	for _, w := range q.Text {
		if !said(w) {
			return false
		}
	}
	return true
}

// anyOf reports whether test holds for one of vs, or vs is empty.
func anyOf(vs []string, test func(v string) bool) bool {
	if len(vs) == 0 {
		return true
	}
	for _, v := range vs {
		if test(v) {
			return true
		}
	}
	return false
}
