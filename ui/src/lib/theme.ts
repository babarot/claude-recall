import type { Theme } from "../api";

// The TUI's color scheme on the page. A scheme of its own ([tui] theme set
// to tokyo-night, say) comes from the server as the stylesheet's custom
// properties; auto, and ansi, leave the stylesheet's catppuccin-mocha and
// latte, and the switch between them, as they are.

const STYLE_ID = "recall-theme";

/**
 * The CSS that draws the page in a fixed scheme. It outranks every rule
 * of the stylesheet that sets the properties, the ones for a light system
 * and for the switch included, so the scheme holds whatever they say.
 */
export function themeCSS(t: Theme): string {
  if (!t.fixed || !t.colors) return "";
  const vars = Object.entries(t.colors).map(([k, v]) => `--${k}: ${v};`);
  // What the stylesheet derives from the prompt color, and the shadow
  // each of its light and dark sets has.
  vars.push(
    `--shadow: ${t.dark ? "0 12px 40px rgba(0,0,0,.45)" : "0 12px 40px rgba(76,79,105,.18)"};`,
    `--user-bg: color-mix(in srgb, var(--prompt) ${t.dark ? 6 : 7}%, transparent);`,
    `--mark: color-mix(in srgb, var(--prompt) ${t.dark ? 28 : 25}%, transparent);`,
    `color-scheme: ${t.dark ? "dark" : "light"};`,
    // The Stats page's series: the stylesheet's, for dark or light, since a
    // scheme's own colors are not checked for telling series apart.
    ...(t.dark
      ? ["--s1: #3987e5;", "--s2: #d95926;", "--s3: #199e70;", "--s4: #c98500;", "--s5: #d55181;", "--s-other: #5c5b73;"]
      : ["--s1: #2a78d6;", "--s2: #eb6834;", "--s3: #1baf7a;", "--s4: #eda100;", "--s5: #e87ba4;", "--s-other: #a3a7b6;"]),
  );
  return `:root:root:root { ${vars.join(" ")} }`;
}

/** Puts the scheme on the page, or takes it off. */
export function applyTheme(t: Theme | null): void {
  let el = document.getElementById(STYLE_ID);
  const css = t ? themeCSS(t) : "";
  if (!css) {
    el?.remove();
    return;
  }
  if (!el) {
    el = document.createElement("style");
    el.id = STYLE_ID;
    document.head.appendChild(el);
  }
  el.textContent = css;
}
