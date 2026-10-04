export type Theme = "dark" | "light";
export type Accent = "moss" | "clay" | "ink";

const themeKey = "youchu-theme";
const accentKey = "youchu-accent";
const paperByTheme: Record<Theme, string> = {
  dark: "#161a18",
  light: "#f6f7f4",
};

function applyThemeColor(theme: Theme): void {
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta) meta.setAttribute("content", paperByTheme[theme]);
}

export function readTheme(): Theme {
  const value = localStorage.getItem(themeKey);
  return value === "light" || value === "dark" ? value : "light";
}

export function readAccent(): Accent {
  const value = localStorage.getItem(accentKey);
  return value === "clay" || value === "ink" || value === "moss" ? value : "moss";
}

export function setTheme(theme: Theme): void {
  localStorage.setItem(themeKey, theme);
  document.documentElement.setAttribute("data-theme", theme);
  applyThemeColor(theme);
}

applyThemeColor(readTheme());

export function setAccent(accent: Accent): void {
  localStorage.setItem(accentKey, accent);
  document.documentElement.setAttribute("data-accent", accent);
}
