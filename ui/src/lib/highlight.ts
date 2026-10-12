import { escapeHTML } from "../store/app";

/** Escapes text and marks the words in it, ignoring case. */
export function highlight(text: string, words: string[]): string {
  const ws = words.filter((w) => w.length > 0).map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
  if (ws.length === 0) return escapeHTML(text);
  const re = new RegExp(ws.join("|"), "gi");
  let out = "";
  let at = 0;
  for (const m of text.matchAll(re)) {
    out += escapeHTML(text.slice(at, m.index)) + `<mark>${escapeHTML(m[0])}</mark>`;
    at = m.index! + m[0].length;
  }
  return out + escapeHTML(text.slice(at));
}
