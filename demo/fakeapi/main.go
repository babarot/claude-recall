// Command fakeapi stands in for the Anthropic Messages API while
// demo/claude-*.tape record the real Claude Code, the way demo/bin/claude
// stands in for Claude Code in the TUI demo: Claude Code is pointed at it
// with ANTHROPIC_BASE_URL and gets the same replies every time, so the demo
// needs no login and costs nothing.
//
// It answers one conversation: asked with recall_search among the tools, it
// calls it for "token" in this repository (loading it through ToolSearch
// first when Claude Code defers it), then sums up what came back. A request
// with no tools (the session's title) gets a short title. A streamed reply
// waits a moment before it starts and comes a few words at a time, as a
// model's does; the waits are random from a fixed seed, so every recording
// has the same ones. Run it from the Makefile: go run ./demo/fakeapi
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	searchQuery = "token"
	answer      = "Two sessions in this repository worked on token refresh. c2088111 fixed the race when concurrent requests refreshed at once, and af2ef85b added tests for it under load."
	title       = "Token refresh"

	loadID   = "toolu_demo_load"
	searchID = "toolu_demo_search"
)

type block struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type request struct {
	Stream bool `json:"stream"`
	Tools  []struct {
		Name string `json:"name"`
	} `json:"tools"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
}

var seq atomic.Int64

// pace makes the streamed replies take as long as a model's: random waits
// from a fixed seed, scaled by -pace (0 for none).
var pace = struct {
	sync.Mutex
	rng   *rand.Rand
	scale float64
}{rng: rand.New(rand.NewSource(1))}

// wait sleeps between lo and hi milliseconds, scaled.
func wait(lo, hi int) {
	pace.Lock()
	ms := float64(lo + pace.rng.Intn(hi-lo+1))
	pace.Unlock()
	time.Sleep(time.Duration(ms*pace.scale) * time.Millisecond)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:47123", "address to listen on")
	flag.Float64Var(&pace.scale, "pace", 1, "how long replies take, against a model's (0 answers at once)")
	flag.Parse()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/messages/count_tokens"):
			writeJSON(w, map[string]any{"input_tokens": 10})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/messages"):
			var req request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			blocks, stop := reply(req)
			if req.Stream {
				// The title is asked for beside the turn; it answers at once,
				// so the turn's waits come in the same order every time.
				stream(w, blocks, stop, len(req.Tools) > 0)
				return
			}
			writeJSON(w, map[string]any{"id": msgID(), "type": "message", "role": "assistant", "model": "claude-opus-5-5",
				"content": blocks, "stop_reason": stop, "stop_sequence": nil, "usage": usage()})
		default:
			// Anything else Claude Code asks for (settings, models) is absent.
			http.NotFound(w, r)
		}
	})
	log.Printf("fakeapi listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

// reply is the next turn of the one conversation the demo has.
func reply(req request) ([]block, string) {
	search := ""
	hasToolSearch := false
	for _, t := range req.Tools {
		if strings.HasSuffix(t.Name, "__recall_search") {
			search = t.Name
		}
		hasToolSearch = hasToolSearch || t.Name == "ToolSearch"
	}
	answered := toolResults(req)
	switch {
	case len(req.Tools) == 0:
		return []block{{Type: "text", Text: title}}, "end_turn"
	case answered[searchID]:
		return []block{{Type: "text", Text: answer}}, "end_turn"
	case search != "":
		return []block{{Type: "tool_use", ID: searchID, Name: search,
			Input: map[string]any{"query": searchQuery, "repo": "."}}}, "tool_use"
	case hasToolSearch && !answered[loadID]:
		return []block{{Type: "tool_use", ID: loadID, Name: "ToolSearch",
			Input: map[string]any{"query": "select:mcp__plugin_claude-recall_claude-recall__recall_search", "max_results": 1}}}, "tool_use"
	}
	return []block{{Type: "text", Text: answer}}, "end_turn"
}

// toolResults is the IDs of the tool calls the conversation has answered.
func toolResults(req request) map[string]bool {
	out := map[string]bool{}
	for _, m := range req.Messages {
		var blocks []struct {
			Type      string `json:"type"`
			ToolUseID string `json:"tool_use_id"`
		}
		if json.Unmarshal(m.Content, &blocks) != nil {
			continue // a plain string
		}
		for _, b := range blocks {
			if b.Type == "tool_result" {
				out[b.ToolUseID] = true
			}
		}
	}
	return out
}

// stream writes the reply as the server-sent events of a streamed message:
// after a moment of thought, the text a few words at a time.
func stream(w http.ResponseWriter, blocks []block, stop string, paced bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	event := func(name string, data any) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
		if flusher != nil {
			flusher.Flush()
		}
	}
	pause := func(lo, hi int) {
		if paced {
			wait(lo, hi)
		}
	}
	pause(1200, 2000)
	event("message_start", map[string]any{"type": "message_start", "message": map[string]any{
		"id": msgID(), "type": "message", "role": "assistant", "model": "claude-opus-5-5",
		"content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": usage()}})
	for i, b := range blocks {
		if b.Type == "text" {
			event("content_block_start", map[string]any{"type": "content_block_start", "index": i,
				"content_block": map[string]any{"type": "text", "text": ""}})
			for _, piece := range pieces(b.Text) {
				event("content_block_delta", map[string]any{"type": "content_block_delta", "index": i,
					"delta": map[string]any{"type": "text_delta", "text": piece}})
				pause(25, 70)
			}
		} else {
			input, _ := json.Marshal(b.Input)
			pause(300, 600)
			event("content_block_start", map[string]any{"type": "content_block_start", "index": i,
				"content_block": map[string]any{"type": "tool_use", "id": b.ID, "name": b.Name, "input": map[string]any{}}})
			event("content_block_delta", map[string]any{"type": "content_block_delta", "index": i,
				"delta": map[string]any{"type": "input_json_delta", "partial_json": string(input)}})
		}
		event("content_block_stop", map[string]any{"type": "content_block_stop", "index": i})
	}
	event("message_delta", map[string]any{"type": "message_delta",
		"delta": map[string]any{"stop_reason": stop, "stop_sequence": nil}, "usage": map[string]any{"output_tokens": 20}})
	event("message_stop", map[string]any{"type": "message_stop"})
}

// pieces splits text into runs of a word or two, the size a model streams.
func pieces(text string) []string {
	var out []string
	words := strings.SplitAfter(text, " ")
	for i := 0; i < len(words); i += 2 {
		out = append(out, strings.Join(words[i:min(i+2, len(words))], ""))
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func msgID() string { return fmt.Sprintf("msg_demo_%d", seq.Add(1)) }

func usage() map[string]any { return map[string]any{"input_tokens": 10, "output_tokens": 1} }
