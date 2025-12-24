import { fetchObject } from "./s3";
import { THEME_DEFAULTS } from "./themeDefaults";

export type ThemeSettings = {
  primary: string;
  background: string;
  surface: string;
  text: string;
  border: string;
  sidebar: string;
  fontFamily: string;
  fontSize: number;
  radius: number;
  shadow: boolean;
};

export const DEFAULT_THEME: ThemeSettings = { ...THEME_DEFAULTS };

const THEME_KEY = "theme.json";

export async function getThemeSettings(prefix?: string | null): Promise<ThemeSettings> {
  const key = buildKey(prefix);
  try {
    const res = await fetchObject(key);
    if (res.status !== 200 || !res.body) return DEFAULT_THEME;
    const parsed = JSON.parse(res.body);
    return { ...DEFAULT_THEME, ...(parsed || {}) };
  } catch {
    return DEFAULT_THEME;
  }
}

export function themeToCssVars(settings: ThemeSettings) {
  const tokens = deriveTokens(settings);
  const entries: [string, string | number][] = [
    ["--design-primary", tokens.primary],
    ["--design-primary-hover", tokens.primaryHover],
    ["--design-primary-active", tokens.primaryActive],
    ["--design-primary-disabled", tokens.primaryDisabled],
    ["--design-bg", tokens.background],
    ["--design-surface", tokens.surface],
    ["--design-surface-muted", tokens.surfaceMuted],
    ["--design-surface-hover", tokens.surfaceHover],
    ["--design-surface-muted-hover", tokens.surfaceMutedHover],
    ["--design-sidebar", tokens.sidebar],
    ["--design-border", tokens.border],
    ["--design-text", tokens.text],
    ["--design-radius", `${tokens.radius}px`],
    ["--design-font", tokens.fontFamily],
    ["--design-font-size", `${tokens.fontSize}px`],
    ["--design-shadow", tokens.shadow],
    ["--color-accent", tokens.primary],
    ["--color-accent-strong", tokens.primaryHover],
    ["--color-accent-stronger", tokens.primaryActive],
    ["--color-bg", tokens.background],
    ["--color-surface", tokens.surface],
    ["--color-surface-muted", tokens.surfaceMuted],
    ["--color-surface-hover", tokens.surfaceHover],
    ["--color-surface-muted-hover", tokens.surfaceMutedHover],
    ["--color-sidebar", tokens.sidebar],
    ["--color-text", tokens.text],
    ["--color-border", tokens.border],
    ["--radius-md", `${tokens.radius}px`],
    ["--radius-lg", `${Math.max(tokens.radius, 14)}px`],
    ["--radius-sm", `${Math.max(Math.round(tokens.radius * 0.65), 6)}px`],
    ["--color-shadow", tokens.shadow],
    ["--font-body", tokens.fontFamily],
    ["--font-heading", tokens.fontFamily],
    ["--font", tokens.fontFamily],
    ["--font-size-base", `${tokens.fontSize}px`],
    ["--radius", `${tokens.radius}px`],
  ];
  return entries
    .filter(([, v]) => v !== undefined && v !== null && `${v}`.length > 0)
    .map(([k, v]) => `${k}:${v}`)
    .join(";");
}

function deriveTokens(settings: ThemeSettings) {
  const primary = settings.primary || DEFAULT_THEME.primary;
  const surface = settings.surface || DEFAULT_THEME.surface;
  const background = settings.background || DEFAULT_THEME.background;
  const border = settings.border || DEFAULT_THEME.border;
  const text = settings.text || DEFAULT_THEME.text;
  const sidebar = settings.sidebar || DEFAULT_THEME.sidebar;
  return {
    primary,
    primaryHover: adjustLightness(primary, -10),
    primaryActive: adjustLightness(primary, -16),
    primaryDisabled: adjustLightness(primary, 18, 0.5),
    background,
    surface,
    surfaceMuted: adjustLightness(surface, -4),
    surfaceHover: adjustSurfaceHover(surface),
    surfaceMutedHover: adjustSurfaceHover(adjustLightness(surface, -4)),
    border,
    sidebar,
    text,
    radius: settings.radius ?? DEFAULT_THEME.radius,
    shadow: settings.shadow ? "0 8px 20px rgba(0,0,0,0.08)" : "none",
    fontFamily: settings.fontFamily || DEFAULT_THEME.fontFamily,
    fontSize: settings.fontSize || DEFAULT_THEME.fontSize,
  };
}

function buildKey(prefix?: string | null) {
  const clean = (prefix || "").replace(/^\/+|\/+$/g, "");
  return clean ? `${clean}/${THEME_KEY}` : THEME_KEY;
}

function adjustLightness(color: string, delta: number, opacity = 1) {
  const { h, s, l } = hexToHsl(color || "#000000");
  const nextL = clamp(l + delta, 0, 100);
  return hslToHex(h, s, nextL, opacity);
}

function hexToHsl(hex: string) {
  let sanitized = hex.trim();
  if (sanitized.startsWith("#")) sanitized = sanitized.slice(1);
  if (sanitized.length === 3) {
    sanitized = sanitized
      .split("")
      .map((c) => c + c)
      .join("");
  }
  const num = parseInt(sanitized || "000000", 16);
  const r = (num >> 16) & 255;
  const g = (num >> 8) & 255;
  const b = num & 255;

  const rNorm = r / 255;
  const gNorm = g / 255;
  const bNorm = b / 255;
  const max = Math.max(rNorm, gNorm, bNorm);
  const min = Math.min(rNorm, gNorm, bNorm);
  let h = 0;
  let s = 0;
  const l = (max + min) / 2;

  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    switch (max) {
      case rNorm:
        h = (gNorm - bNorm) / d + (gNorm < bNorm ? 6 : 0);
        break;
      case gNorm:
        h = (bNorm - rNorm) / d + 2;
        break;
      case bNorm:
        h = (rNorm - gNorm) / d + 4;
        break;
    }
    h /= 6;
  }

  return { h: h * 360, s: s * 100, l: l * 100 };
}

function hslToHex(h: number, s: number, l: number, opacity = 1) {
  const sNorm = s / 100;
  const lNorm = l / 100;
  const k = (n: number) => (n + h / 30) % 12;
  const a = sNorm * Math.min(lNorm, 1 - lNorm);
  const f = (n: number) => lNorm - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
  const r = Math.round(255 * f(0));
  const g = Math.round(255 * f(8));
  const b = Math.round(255 * f(4));
  const toHex = (x: number) => x.toString(16).padStart(2, "0");
  const hex = `#${toHex(r)}${toHex(g)}${toHex(b)}`;
  if (opacity >= 1) return hex;
  const alpha = toHex(Math.round(clamp(opacity, 0, 1) * 255));
  return `${hex}${alpha}`;
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function adjustSurfaceHover(color: string) {
  const { l } = hexToHsl(color || "#ffffff");
  const delta = l > 50 ? -6 : 6;
  return adjustLightness(color || "#ffffff", delta);
}
