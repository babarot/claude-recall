import { describe, expect, it, vi } from "vitest";
import { render } from "preact";
import { findQuery, spans, total } from "../store/conversation";
import { Conversation, markWords } from "./Conversation";
import type { Message } from "../api";

const msg = (i: number, content = `m${i}`): Message => ({
  cursor: `${i}.0`, uuid: `u${i}`, role: i % 2 ? "assistant" : "user", type: "text", content,
  toolName: null, toolInput: null, timestamp: null,
});

describe("Conversation", () => {
  // Marking a search's words, and taking the marks off, must not touch the
  // text Preact draws, or what it draws there stops changing.
  it("counts the messages between the spans as more are read, around a search", async () => {
    total.value = 100;
    findQuery.value = "m1";
    spans.value = [{ msgs: [0, 1].map((i) => msg(i)), atStart: true, atEnd: false }, { msgs: [98, 99].map((i) => msg(i)), atStart: false, atEnd: true }];
    const el = document.createElement("div");
    render(<Conversation sessionId="s" />, el);
    // Preact draws, and marks the words, after the signals change: wait for
    // it rather than a fixed time, which a slow machine can outlast.
    await vi.waitFor(() => {
      expect(el.querySelector(".gap-marker")?.textContent).toContain("96 件");
      expect(el.querySelectorAll("mark").length).toBeGreaterThan(0);
    });
    findQuery.value = "";
    spans.value = [{ msgs: [0, 1, 2, 3].map((i) => msg(i)), atStart: true, atEnd: false }, spans.value[1]];
    await vi.waitFor(() => {
      expect(el.querySelector(".gap-marker")?.textContent).toContain("94 件");
      expect(el.querySelectorAll("mark").length).toBe(0);
    });
    render(null, el);
  });
});

describe("markWords", () => {
  it("marks words in the Markdown only, ignoring case, and takes the marks off", () => {
    const root = document.createElement("div");
    root.innerHTML = `<div class="body"><div><p>Remux and remux</p></div><button>remux</button></div>`;
    markWords(root, "REMUX");
    expect(root.querySelectorAll("mark").length).toBe(2);
    expect(root.querySelector("button")?.innerHTML).toBe("remux");
    markWords(root, "");
    expect(root.querySelectorAll("mark").length).toBe(0);
    expect(root.querySelector("p")?.childNodes.length).toBe(1);
  });
});
