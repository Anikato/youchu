export type Theme = "dark" | "light";
export type Accent = "moss" | "clay" | "ink";

const themeKey = "youchu-theme";
const accentKey = "youchu-accent";

export function readTheme(): Theme {
  const value = localStorage.getItem(themeKey);
  return value === "light" || value === "dark" ? value : "dark";
}

export function readAccent(): Accent {
  const value = localStorage.getItem(accentKey);
  return value === "clay" || value === "ink" || value === "moss" ? value : "moss";
}

export function setTheme(theme: Theme): void {
  localStorage.setItem(themeKey, theme);
  document.documentElement.setAttribute("data-theme", theme);
}

export function setAccent(accent: Accent): void {
  localStorage.setItem(accentKey, accent);
  document.documentElement.setAttribute("data-accent", accent);
}
