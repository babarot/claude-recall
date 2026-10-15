import { describe, expect, it } from "vitest";
import { pickLang } from "./i18n";

describe("pickLang", () => {
  it("is Japanese only when the browser's first language is", () => {
    expect(pickLang(["ja"])).toBe("ja");
    expect(pickLang(["ja-JP", "en-US"])).toBe("ja");
    expect(pickLang(["en-US", "ja"])).toBe("en");
    expect(pickLang(["fr-FR"])).toBe("en");
    expect(pickLang(["jam"])).toBe("en");
    expect(pickLang([])).toBe("en");
  });

  // Every word the page shows comes from i18n.ts: Japanese written into
  // another file would show to a reader of English as well.
  it("finds no Japanese outside i18n.ts", () => {
    const files = import.meta.glob(["../**/*.ts", "../**/*.tsx", "!../**/*.test.*", "!./i18n.ts"], { query: "?raw", import: "default", eager: true });
    expect(Object.keys(files).length).toBeGreaterThan(5);
    const found = Object.entries(files).flatMap(([path, text]) =>
      (text as string).split("\n").flatMap((line, i) => (/[\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Han}]/u.test(line) ? [`${path}:${i + 1}: ${line.trim()}`] : [])));
    expect(found).toEqual([]);
  });
});
