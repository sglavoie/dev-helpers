import { describe, it, expect } from "vitest";
import { buildFieldPreview } from "./fieldPreview";
import { Placeholder } from "../types";

function makePlaceholder(overrides: Partial<Placeholder> & { key: string }): Placeholder {
  return {
    isRequired: false,
    isSaved: true,
    ...overrides,
  };
}

describe("buildFieldPreview", () => {
  it("returns undefined for required fields", () => {
    const p = makePlaceholder({ key: "name", isRequired: true });
    expect(buildFieldPreview(p, "Hello {{name}}", "")).toBeUndefined();
  });

  it("formats guard-only with a single {{#if k}} body", () => {
    const p = makePlaceholder({ key: "k", isGuardOnly: true });
    const content = "{{#if k}}Hello world{{/if}}";
    expect(buildFieldPreview(p, content, "")).toBe('Toggles: "Hello world"');
  });

  it("formats guard-only with else body", () => {
    const p = makePlaceholder({ key: "k", isGuardOnly: true });
    const content = "{{#if k}}A{{#else}}B{{/if}}";
    expect(buildFieldPreview(p, content, "")).toBe('Toggles: "A" • Else: "B"');
  });

  it("truncates guard-only bodies longer than 80 chars", () => {
    const p = makePlaceholder({ key: "k", isGuardOnly: true });
    const body = "x".repeat(100);
    const content = `{{#if k}}${body}{{/if}}`;
    const result = buildFieldPreview(p, content, "");
    expect(result).toBe(`Toggles: "${"x".repeat(80)}…"`);
  });

  it("returns the full body when truncate=false", () => {
    const p = makePlaceholder({ key: "k", isGuardOnly: true });
    const body = "x".repeat(100);
    const content = `{{#if k}}${body}{{/if}}`;
    expect(buildFieldPreview(p, content, "", { truncate: false })).toBe(`Toggles: "${"x".repeat(100)}"`);
  });

  it("returns the full else body when truncate=false", () => {
    const p = makePlaceholder({ key: "k", isGuardOnly: true });
    const ifBody = "a".repeat(100);
    const elseBody = "b".repeat(100);
    const content = `{{#if k}}${ifBody}{{#else}}${elseBody}{{/if}}`;
    expect(buildFieldPreview(p, content, "", { truncate: false })).toBe(
      `Toggles: "${"a".repeat(100)}" • Else: "${"b".repeat(100)}"`,
    );
  });

  it("collapses whitespace runs in guard-only bodies", () => {
    const p = makePlaceholder({ key: "k", isGuardOnly: true });
    const content = "{{#if k}}line1\n\n  line2\tline3{{/if}}";
    expect(buildFieldPreview(p, content, "")).toBe('Toggles: "line1 line2 line3"');
  });

  it("preserves nested {{key}} template text verbatim in guard-only bodies", () => {
    const p = makePlaceholder({ key: "k", isGuardOnly: true });
    const content = "{{#if k}}Hello {{name}}{{/if}}";
    expect(buildFieldPreview(p, content, "")).toBe('Toggles: "Hello {{name}}"');
  });

  it("returns undefined for guard-only when extraction yields no blocks", () => {
    const p = makePlaceholder({ key: "k", isGuardOnly: true });
    expect(buildFieldPreview(p, "no conditionals here", "")).toBeUndefined();
  });

  it("formats wrapper field with prefix and suffix and a value", () => {
    const p = makePlaceholder({ key: "link", prefixWrapper: "(", suffixWrapper: ")" });
    expect(buildFieldPreview(p, "{{(:link:)}}", "foo")).toBe("Wraps as: (foo)");
  });

  it("uses <value> placeholder when wrapper field value is empty", () => {
    const p = makePlaceholder({ key: "link", prefixWrapper: "(", suffixWrapper: ")" });
    expect(buildFieldPreview(p, "{{(:link:)}}", "")).toBe("Wraps as: (<value>)");
  });

  it("handles wrapper field with only a suffix", () => {
    const p = makePlaceholder({ key: "label", suffixWrapper: ":" });
    expect(buildFieldPreview(p, "{{label:}}", "")).toBe("Wraps as: <value>:");
  });

  it("formats plain optional with a default value", () => {
    const p = makePlaceholder({ key: "name", defaultValue: "Alice" });
    expect(buildFieldPreview(p, "Hello {{name|Alice}}", "")).toBe('Empty → uses default: "Alice"');
  });

  it("formats plain optional with an empty-string default (the {{k|}} form)", () => {
    const p = makePlaceholder({ key: "k", defaultValue: "" });
    expect(buildFieldPreview(p, "Hello {{k|}}", "")).toBe("Empty → renders as empty string.");
  });

  it("formats plain optional with no defaultValue", () => {
    const p = makePlaceholder({ key: "k" });
    expect(buildFieldPreview(p, "Hello {{k}}", "")).toBe("Optional — leave blank to omit.");
  });
});
