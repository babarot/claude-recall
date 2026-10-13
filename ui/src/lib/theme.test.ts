import { afterEach, describe, expect, it } from "vitest";
import { applyTheme, themeCSS } from "./theme";

describe("theme", () => {
  afterEach(() => applyTheme(null));

  it("draws a fixed scheme over the stylesheet", () => {
    const css = themeCSS({ name: "tokyo-night", fixed: true, dark: true, colors: { bg: "#1a1b26", prompt: "#e0af68" } });
    expect(css).toContain("--bg: #1a1b26;");
    expect(css).toContain("--prompt: #e0af68;");
    expect(css).toContain("color-scheme: dark;");
    expect(css.startsWith(":root:root:root")).toBe(true);
  });

  it("leaves auto to the stylesheet", () => {
    expect(themeCSS({ name: "auto", fixed: false })).toBe("");
    applyTheme({ name: "nord", fixed: true, dark: true, colors: { bg: "#2e3440" } });
    expect(document.getElementById("recall-theme")?.textContent).toContain("#2e3440");
    applyTheme({ name: "auto", fixed: false });
    expect(document.getElementById("recall-theme")).toBeNull();
  });
});
