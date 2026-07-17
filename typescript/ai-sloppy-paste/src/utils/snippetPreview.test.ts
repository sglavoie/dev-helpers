import { describe, expect, it } from "vitest";
import { buildSnippetPreview } from "./snippetPreview";

describe("buildSnippetPreview", () => {
  it("returns an empty preview when there is no placeholder syntax", () => {
    expect(buildSnippetPreview("Plain content")).toBe("");
  });

  it("preserves ordinary defaults, no-save labels, and wrapper formatting", () => {
    expect(buildSnippetPreview("{{name}} {{role|Editor}} {{!$:amount: USD|10}}")).toBe(
      '⟨name⟩ ⟨role = "Editor"⟩ ⟨!$amount USD = "10"⟩',
    );
  });

  it("shows decoded authored choices and an explicit default", () => {
    expect(buildSnippetPreview("{{tone[Formal|A\\|B|\\[Custom\\]|Path\\\\Name]|Formal}} then {{tone}}")).toBe(
      '⟨tone — choices: "Formal", "A|B", "[Custom]", "Path\\\\Name"; default: "Formal"⟩ then ⟨tone⟩',
    );
  });

  it("distinguishes a choice declaration without a default from an explicit empty default", () => {
    expect(buildSnippetPreview("{{tone[Formal|Casual]}} / {{detail[Short|Long]|}}")).toBe(
      '⟨tone — choices: "Formal", "Casual"⟩ / ⟨detail — choices: "Short", "Long"; default: ""⟩',
    );
  });

  it("previews conditional blocks before the placeholders inside them", () => {
    expect(buildSnippetPreview("{{#if name}}Hi {{name}}{{#else}}Hello{{/if}}")).toBe(
      "⟨if name: Hi ⟨name⟩ | else: Hello⟩",
    );
  });

  it("resolves system placeholders before user placeholders", () => {
    const preview = buildSnippetPreview("On {{DATE}}, greet {{name}}");
    expect(preview).toMatch(/^On ⟨DATE → .+⟩, greet ⟨name⟩$/);
    expect(preview).not.toContain("{{DATE}}");
  });

  it("caps the final preview at 500 characters", () => {
    const preview = buildSnippetPreview(`{{name}}${"x".repeat(600)}`);
    expect(preview).toHaveLength(500);
    expect(preview.startsWith("⟨name⟩")).toBe(true);
  });
});
