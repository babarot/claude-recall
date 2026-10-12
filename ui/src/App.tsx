import { useEffect, useRef, useState } from "preact/hooks";
import { effect } from "@preact/signals";
import {
  bodyWords, density, focus, folder, folders, found, loadSessions, loadSettings, now, query, reading, refreshSession,
  saveSettings, selected, setQuery, showFolders, sort, tab, theme, toast, visible, worktree,
} from "./store/app";
import * as conv from "./store/conversation";
import { subscribeSSE } from "./lib/sse-bus";
import { eventKey, footer, HELP_GROUPS, KEYMAP, keyName, lookup } from "./lib/keymap";
import type { Action, Pane } from "./lib/keymap";
import { Folders, folderRows, pick } from "./components/Folders";
import { SessionList, SORTS } from "./components/SessionList";
import { copyID, copyResume, findInput, Reader, stepHit } from "./components/Reader";
import { atBottom, conversationEl, currentPrompt, goTo, zoom } from "./components/Conversation";
import { StatsView } from "./components/StatsView";
import { api } from "./api";

// URL: / or /session/<id>, with ?q= for the search; /stats for the stats.
function readURL(): { id: string | null; q: string; stats: boolean } {
  const m = location.pathname.match(/^\/session\/([^/]+)$/);
  return { id: m ? decodeURIComponent(m[1]) : null, q: new URLSearchParams(location.search).get("q") ?? "", stats: location.pathname === "/stats" };
}

function writeURL(): void {
  const id = selected.peek();
  const q = query.peek();
  const path = (id ? `/session/${encodeURIComponent(id)}` : "/") + (q ? `?q=${encodeURIComponent(q)}` : "");
  if (location.pathname + location.search !== path) history.replaceState(null, "", path);
}

const narrow = () => matchMedia("(max-width: 860px)").matches;
const midWidth = () => matchMedia("(max-width: 1180px)").matches;

export function App() {
  const [stats, setStats] = useState(readURL().stats);
  const [version, setVersion] = useState("");
  const [keysOpen, setKeysOpen] = useState(false);
  const [sortOpen, setSortOpen] = useState(false);
  const [foldersOpen, setFoldersOpen] = useState(false); // on a mid-width screen
  const searchRef = useRef<HTMLInputElement>(null);
  const [searchFocused, setSearchFocused] = useState(false);

  useEffect(() => {
    loadSettings();
    const u = readURL();
    if (u.id) {
      selected.value = u.id;
      reading.value = true;
    }
    if (u.q) setQuery(u.q, 0);
    void loadSessions();
    api.status().then((s) => setVersion(s.version)).catch(() => {});
    const tick = setInterval(() => { now.value = new Date(); }, 30000);
    const unsub = subscribeSSE((e) => {
      if (e.type === "reconnected") {
        // Back from hidden, or the server restarted: read what was missed.
        void loadSessions();
        if (selected.value) void conv.follow(selected.value, atBottom);
        return;
      }
      if (e.type !== "session_updated" || typeof e.sessionId !== "string") return;
      const id = e.sessionId;
      void refreshSession(id);
      if (id === selected.value) void conv.follow(id, atBottom);
    });
    return () => { clearInterval(tick); unsub(); };
  }, []);

  // The theme on <html>.
  useEffect(() => effect(() => {
    const r = document.documentElement;
    if (theme.value === "auto") r.removeAttribute("data-theme");
    else r.setAttribute("data-theme", theme.value);
  }), []);

  // Opening a session: its conversation, and from a search, the place it
  // was found.
  useEffect(() => effect(() => {
    const id = selected.value;
    writeURL();
    if (!id) return void conv.close();
    tab.value = "conv";
    const q = query.peek();
    const snippet = found.peek()?.snippets?.[id];
    void conv.open(id).then(async () => {
      const words = bodyWords(q);
      if (words.length) await conv.findInConversation(words[0]);
      if (snippet) void goTo(snippet.cursor);
    });
  }), []);
  useEffect(() => effect(() => {
    void query.value;
    writeURL();
  }), []);

  // With nothing selected, or the selection filtered out, a wide screen
  // shows the first session.
  useEffect(() => effect(() => {
    const list = visible.value;
    if (narrow() || list.length === 0) return;
    if (!list.some((s) => s.id === selected.peek())) selected.value = list[0].id;
  }), []);

  useEffect(() => {
    const close = (e: MouseEvent) => {
      if (!(e.target as HTMLElement).closest(".menu, .sortbtn")) setSortOpen(false);
    };
    window.addEventListener("click", close);
    return () => window.removeEventListener("click", close);
  }, []);

  const moveSelection = (d: number | "top" | "bottom") => {
    const list = visible.value;
    if (list.length === 0) return;
    const i = list.findIndex((s) => s.id === selected.value);
    const j = d === "top" ? 0 : d === "bottom" ? list.length - 1 : Math.max(0, Math.min(list.length - 1, (i < 0 ? 0 : i) + d));
    selected.value = list[j].id;
  };

  const moveFolder = (d: number) => {
    const rows = folderRows(folders.value, "");
    const i = Math.max(0, rows.findIndex((r) => r.key === folder.value && r.wt === worktree.value));
    const r = rows[Math.max(0, Math.min(rows.length - 1, i + d))];
    pick(r.key, r.wt);
  };

  const toggleFolders = () => {
    if (midWidth()) setFoldersOpen(!foldersOpen);
    else {
      showFolders.value = !showFolders.value;
      saveSettings();
    }
  };

  const panes = (): Pane[] => (showFolders.value && !midWidth()) || foldersOpen ? ["folders", "list", "reader"] : ["list", "reader"];

  const promptStep = (d: number) => {
    const list = conv.outline.value;
    if (list.length === 0) return;
    const i = list.findIndex((p) => p.cursor === currentPrompt.value);
    const j = Math.max(0, Math.min(list.length - 1, (i < 0 ? -1 : i) + d));
    void goTo(list[j].cursor);
  };

  const runAction = (a: Action, pane: Pane) => {
    const el = conversationEl.current;
    switch (a) {
      case "search":
        if (pane === "reader") findInput.current?.focus();
        else searchRef.current?.focus();
        break;
      case "focusNext":
      case "focusPrev": {
        const ps = panes();
        const i = ps.indexOf(pane);
        focus.value = ps[(i + (a === "focusNext" ? 1 : -1) + ps.length) % ps.length];
        break;
      }
      case "folders": toggleFolders(); break;
      case "sort": setSortOpen(!sortOpen); break;
      case "help": setKeysOpen(true); break;
      case "copyId": copyID(); break;
      case "copyResume": copyResume(); break;
      case "back":
        if (pane === "reader" || pane === "folders") {
          focus.value = "list";
          reading.value = false;
        } else if (query.value) setQuery("");
        break;
      case "down": if (pane === "folders") moveFolder(1); else moveSelection(1); break;
      case "up": if (pane === "folders") moveFolder(-1); else moveSelection(-1); break;
      case "top": moveSelection("top"); break;
      case "bottom": moveSelection("bottom"); break;
      case "open":
        focus.value = "reader";
        reading.value = true;
        break;
      case "pickFolder": focus.value = "list"; break;
      case "scrollDown": el?.scrollBy({ top: 80 }); break;
      case "scrollUp": el?.scrollBy({ top: -80 }); break;
      case "prevPrompt": promptStep(-1); break;
      case "nextPrompt": promptStep(1); break;
      case "nextHit": stepHit(1); break;
      case "prevHit": stepHit(-1); break;
      case "density":
        density.value = density.value === "talk" ? "all" : "talk";
        saveSettings();
        break;
      case "tabConv": tab.value = "conv"; break;
      case "tabDone": tab.value = "done"; break;
    }
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (keysOpen) {
        if (["Escape", "?", "q"].includes(e.key)) setKeysOpen(false);
        return;
      }
      if (zoom.value) {
        if (e.key === "Escape") zoom.value = null;
        return;
      }
      const t = e.target as HTMLElement;
      if (t.tagName === "INPUT" || e.metaKey || e.ctrlKey || e.altKey) return;
      if (sortOpen && /^[1-4]$/.test(e.key)) {
        sort.value = SORTS[Number(e.key) - 1].key;
        saveSettings();
        setSortOpen(false);
        return;
      }
      const a = lookup(focus.value, eventKey(e));
      if (!a) return;
      e.preventDefault();
      runAction(a, focus.value);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  if (stats) return <StatsView onBack={() => { history.pushState(null, "", "/"); setStats(false); }} />;

  const mainClass = [
    !showFolders.value && "no-folders",
    foldersOpen && "folders-open",
    reading.value && "reading",
  ].filter(Boolean).join(" ");

  return (
    <>
      <header>
        <div class="brand"><b>recall</b>{version && <span class="mono">{version}</span>}</div>
        <label class="search">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
          <input ref={searchRef} value={query.value} autocomplete="off"
            placeholder="セッションを検索：タイトル・フォルダ・ブランチ・会話の中身"
            onFocus={() => setSearchFocused(true)} onBlur={() => setSearchFocused(false)}
            onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                e.stopPropagation();
                setQuery("");
                searchRef.current?.blur();
                focus.value = "list";
              } else if (e.key === "Enter" || e.key === "ArrowDown") {
                e.preventDefault();
                searchRef.current?.blur();
                focus.value = "list";
              }
            }} />
          <span class="hint"><code>in:</code> <code>branch:</code> <code>text:</code></span>
          <kbd>/</kbd>
          <div class={`suggest ${searchFocused && !query.value ? "show" : ""}`}>
            {[
              ["text:<word>", "会話の中身だけ"],
              ["title:<part>", "タイトルの一部"],
              ["in:<folder>", "フォルダ（あいまい一致、in:crec で claude-recall）"],
              ["branch:<part>", "ブランチの一部"],
              ["worktree:<part>", "worktree 名の一部"],
              ["id:<prefix>", "セッション ID の先頭"],
            ].map(([k, v]) => (
              <div key={k} onMouseDown={(e) => { e.preventDefault(); setQuery(k.replace(/<.*>/, ""), 0); }}>
                <code>{k}</code><span>{v}</span>
              </div>
            ))}
          </div>
        </label>
        <div class="hdr-right">
          <span class="live-pill" title="新しい会話は、開いたまま流れてきます"><span class="dot pulse" />live</span>
          <button class="iconbtn" title="Stats" onClick={() => { history.pushState(null, "", "/stats"); setStats(true); }}>
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 20V10M10 20V4M16 20v-7M22 20H2" /></svg>
          </button>
          <button class="iconbtn" title="テーマ：auto / dark / light" onClick={() => {
            theme.value = ({ auto: "dark", dark: "light", light: "auto" } as const)[theme.value];
            saveSettings();
          }}>
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9" /><path d="M12 3a9 9 0 0 0 0 18z" fill="currentColor" /></svg>
            <span>{theme.value}</span>
          </button>
          <button class="iconbtn" title="キー一覧" onClick={() => setKeysOpen(true)}><kbd>?</kbd></button>
        </div>
      </header>

      <main class={mainClass}>
        <div onMouseDown={() => { focus.value = "folders"; }} style="display:contents">
          <Folders focused={focus.value === "folders"} />
        </div>
        <div onMouseDown={() => { focus.value = "list"; }} style="display:contents">
          <SessionList focused={focus.value === "list"} sortOpen={sortOpen}
            setSortOpen={(v) => { setSortOpen(v); saveSettings(); }}
            onPick={(id) => { selected.value = id; reading.value = true; }} />
        </div>
        <div onMouseDown={() => { focus.value = "reader"; }} style="display:contents">
          <Reader focused={focus.value === "reader"} />
        </div>
      </main>

      <footer>
        {footer(focus.value).map((f) => <span key={f.keys}><span class="k">{f.keys}</span>{f.label}</span>)}
        <span class="right"><span class="k">?</span>すべてのキー</span>
      </footer>

      <div class={`overlay ${keysOpen ? "show" : ""}`} onClick={(e) => { if (e.target === e.currentTarget) setKeysOpen(false); }}>
        <div class="modal">
          <h2>キー</h2>
          <div class="keys">
            {HELP_GROUPS.flatMap((g) => [
              <h4 key={g.where}>{g.title}</h4>,
              ...KEYMAP.filter((b) => b.where === g.where)
                .map((b) => (
                  <div key={g.where + b.action + b.keys[0]}>
                    <span class="ks">{b.keys.map((k) => <kbd key={k}>{keyName(k)}</kbd>)}</span>{b.label}
                  </div>
                )),
            ])}
          </div>
        </div>
      </div>
      {zoom.value && <div class="overlay zoom show" onClick={() => { zoom.value = null; }}><img src={zoom.value} alt="" /></div>}
      <Toast />
    </>
  );
}

function Toast() {
  const t = toast.value;
  const [shown, setShown] = useState(false);
  useEffect(() => {
    if (!t) return;
    setShown(true);
    const timer = setTimeout(() => setShown(false), 1800);
    return () => clearTimeout(timer);
  }, [t?.at]);
  return <div class={`toast ${shown ? "show" : ""}`} dangerouslySetInnerHTML={{ __html: t?.html ?? "" }} />;
}
