/**
 * Context utilities for the informal `Prefix: rest of title` convention.
 *
 * Snippet titles namespace themselves with a prefix (`asl: run`,
 * `Refactor: debt triage`). Contexts are derived from the title at render
 * time — nothing is persisted, so there is no data model change and no
 * migration. Stripping is display-only: search, keywords, and every
 * `searchFilter` code path keep reading the raw `snippet.title`.
 *
 * Only `Image`/`Color` *types* are imported from Raycast so this module stays
 * unit-testable like `./tags`.
 */

import type { Color, Image } from "@raycast/api";
import type { Snippet } from "../types";

export interface TitleContext {
  /** The raw prefix as written in the title, or null when there is none */
  context: string | null;
  /** The title with the `Prefix: ` part removed (unchanged when no context) */
  displayTitle: string;
}

/**
 * Matches `Prefix: rest`, where the prefix is alphanumeric plus spaces,
 * underscores and hyphens, capped at 24 characters.
 *
 * The `\s+` after the colon is what keeps `https://example.com` and
 * `Note:something` out, and `(\S.*)` is what keeps a bare `asl:` intact.
 */
const CONTEXT_PREFIX_REGEX = /^([A-Za-z0-9][A-Za-z0-9 _-]{0,23}):\s+(\S.*)$/;

/** A prefix longer than this many words reads as a sentence, not a namespace */
const MAX_CONTEXT_WORDS = 3;

/** A context this short fits the badge whole, so it is shown verbatim */
const MAX_WHOLE_LENGTH = 5;

/** Characters kept when a context is too long to show whole */
const TRUNCATED_LENGTH = 4;

/**
 * Per-theme hex pairs. These must be raw hex — named Raycast colors
 * (`Color.Blue`) are tokens the renderer resolves, and they mean nothing
 * inside SVG markup.
 */
const CONTEXT_PALETTE: ReadonlyArray<Color.Dynamic> = [
  { light: "#2B6CB0", dark: "#7AA7E9" }, // blue
  { light: "#2F855A", dark: "#68D391" }, // green
  { light: "#C05621", dark: "#F6AD55" }, // orange
  { light: "#C53030", dark: "#FC8181" }, // red
  { light: "#6B46C1", dark: "#B794F4" }, // purple
  { light: "#B83280", dark: "#F687B3" }, // magenta
  { light: "#2C7A7B", dark: "#4FD1C5" }, // teal
  { light: "#975A16", dark: "#ECC94B" }, // yellow
];

/**
 * Splits a context into abbreviation segments. Underscores and hyphens count
 * as separators so `dead-code` abbreviates the same way as `dead code`.
 */
function contextSegments(context: string): string[] {
  return context
    .trim()
    .split(/[\s_-]+/)
    .filter(Boolean);
}

/** Whitespace-delimited word count, used to reject sentence-like prefixes */
function contextWordCount(context: string): number {
  return context.trim().split(/\s+/).filter(Boolean).length;
}

/**
 * Extracts the context prefix from a snippet title.
 *
 * Returns `{ context: null, displayTitle: title }` when the title does not
 * follow the convention — callers then fall back to the plain title.
 */
export function parseTitleContext(title: string): TitleContext {
  const match = CONTEXT_PREFIX_REGEX.exec(title);
  if (!match) {
    return { context: null, displayTitle: title };
  }

  const prefix = match[1].trim();
  if (!prefix || contextWordCount(prefix) > MAX_CONTEXT_WORDS) {
    return { context: null, displayTitle: title };
  }

  return { context: prefix, displayTitle: match[2].trim() };
}

/**
 * Lowercases and trims a context so `Refactor` and `refactor` unify for
 * filtering and colour assignment.
 */
export function normalizeContext(context: string): string {
  return context.trim().toLowerCase();
}

/**
 * Builds the label rendered inside a badge. Always uppercase, so the badge
 * column reads uniformly however the titles themselves are cased.
 *
 * Whole      → `asl` → `ASL`, `PLAN` → `PLAN`, `build` → `BUILD`
 * Truncated  → `workflow` → `WORK`, `Refactor` → `REFA`
 * Multi-word → initials: `Dead Code` → `DC`, `go to def` → `GTD`
 */
export function contextAbbreviation(context: string): string {
  const trimmed = context.trim();
  if (!trimmed) return "";

  const label = (() => {
    if (trimmed.length <= MAX_WHOLE_LENGTH) return trimmed;

    const segments = contextSegments(trimmed);
    if (segments.length > 1) {
      return segments
        .slice(0, MAX_WHOLE_LENGTH)
        .map((segment) => segment[0])
        .join("");
    }

    return trimmed.slice(0, TRUNCATED_LENGTH);
  })();

  return label.toUpperCase();
}

/**
 * Deterministic 32-bit string hash over the *normalized* context, so casing
 * variants land on the same palette entry.
 */
function hashContext(normalized: string): number {
  let hash = 0;
  for (let index = 0; index < normalized.length; index++) {
    hash = (hash * 31 + normalized.charCodeAt(index)) | 0;
  }
  return Math.abs(hash);
}

/**
 * Maps a context onto a stable per-theme colour pair. Same context always
 * gets the same colour; different contexts usually differ.
 */
export function contextColor(context: string): Color.Dynamic {
  const normalized = normalizeContext(context);
  return CONTEXT_PALETTE[hashContext(normalized) % CONTEXT_PALETTE.length];
}

function escapeXml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}

/**
 * Shrink the type as the label grows so even five characters fit the pill,
 * which is only ~32 units wide inside its 1-unit inset. Sized for uppercase,
 * which runs noticeably wider than the lowercase it replaced — the 4/5
 * character steps leave room for M- and W-heavy words like `MODAL`.
 */
function badgeFontSize(abbreviation: string): number {
  switch (abbreviation.length) {
    case 1:
      return 16;
    case 2:
      return 14;
    case 3:
      return 12;
    case 4:
      return 10;
    default:
      return 9;
  }
}

/**
 * Base64 rather than URL-encoding: the markup contains `#`, `<` and `"`,
 * all of which would otherwise need escaping inside the data URI.
 * Explicit width/height/viewBox keep Raycast from sizing the icon oddly.
 */
function badgeDataUri(abbreviation: string, color: string): string {
  const fontSize = badgeFontSize(abbreviation);
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 40 40">
  <rect x="1" y="8" width="38" height="24" rx="6" fill="${color}" fill-opacity="0.22"/>
  <text x="20" y="20" font-family="-apple-system, Helvetica, Arial, sans-serif" font-size="${fontSize}" font-weight="600" fill="${color}" text-anchor="middle" dominant-baseline="central">${escapeXml(abbreviation)}</text>
</svg>`;

  return `data:image/svg+xml;base64,${Buffer.from(svg).toString("base64")}`;
}

/**
 * Badge icons are rebuilt for every row on every render, so cache them by
 * the only inputs that matter.
 */
const badgeCache = new Map<string, Image.ImageLike>();

/**
 * Renders a context as a coloured badge suitable for `List.Item`'s `icon`
 * slot — the only slot left of the title that Raycast lets us style
 * (`ItemProps.title` is a single uniform label).
 */
export function contextBadgeIcon(context: string): Image.ImageLike {
  const abbreviation = contextAbbreviation(context);
  const color = contextColor(context);
  const cacheKey = `${abbreviation}|${color.light}|${color.dark}`;

  const cached = badgeCache.get(cacheKey);
  if (cached) return cached;

  const icon: Image.ImageLike = {
    source: {
      light: badgeDataUri(abbreviation, color.light),
      dark: badgeDataUri(abbreviation, color.dark),
    },
  };
  badgeCache.set(cacheKey, icon);
  return icon;
}

/**
 * Distinct normalized contexts across a snippet set, sorted. Feeds the
 * `ctx:` autocomplete.
 */
export function getAllContexts(snippets: Snippet[]): string[] {
  const contexts = new Set<string>();
  for (const snippet of snippets) {
    const { context } = parseTitleContext(snippet.title);
    if (context) contexts.add(normalizeContext(context));
  }
  return Array.from(contexts).sort((a, b) => a.localeCompare(b));
}

/**
 * Convenience for filtering: the normalized context of a snippet, or null.
 */
export function getSnippetContext(snippet: Snippet): string | null {
  const { context } = parseTitleContext(snippet.title);
  return context ? normalizeContext(context) : null;
}
