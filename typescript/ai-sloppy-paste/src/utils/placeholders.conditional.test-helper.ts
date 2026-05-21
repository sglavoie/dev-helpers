import { describe, it, expect } from "vitest";
import {
  extractPlaceholders,
  replacePlaceholders,
  processConditionalBlocks,
  extractConditionalBlockBodies,
} from "./placeholders";

describe("processConditionalBlocks", () => {
  it("if-only: truthy value shows body", () => {
    const text = "{{#if name}}Hello{{/if}}";
    expect(processConditionalBlocks(text, { name: "Alice" })).toBe("Hello");
  });

  it("if-only: falsy/empty value omits body", () => {
    const text = "{{#if name}}Hello{{/if}}";
    expect(processConditionalBlocks(text, { name: "" })).toBe("");
  });

  it("if-else: truthy value shows if-body", () => {
    const text = "{{#if cc}}CC: {{cc}}{{#else}}(No CC){{/if}}";
    expect(processConditionalBlocks(text, { cc: "boss@co.com" })).toBe("CC: {{cc}}");
  });

  it("if-else: falsy value shows else-body", () => {
    const text = "{{#if cc}}CC: {{cc}}{{#else}}(No CC){{/if}}";
    expect(processConditionalBlocks(text, { cc: "" })).toBe("(No CC)");
  });

  it("missing key treated as falsy", () => {
    const text = "{{#if missing}}shown{{/if}}";
    expect(processConditionalBlocks(text, {})).toBe("");
  });

  it("whitespace-only value treated as falsy", () => {
    const text = "{{#if key}}shown{{/if}}";
    expect(processConditionalBlocks(text, { key: "   " })).toBe("");
  });

  it("newline cleanup: block on its own line removed cleanly when falsy", () => {
    const text = "Before\n{{#if key}}\nblock content\n{{/if}}\nAfter";
    expect(processConditionalBlocks(text, { key: "" })).toBe("Before\n\nAfter");
  });

  it("newline cleanup: block on its own line kept when truthy, surrounding text unaffected", () => {
    const text = "Before\n{{#if key}}\nblock content\n{{/if}}\nAfter";
    expect(processConditionalBlocks(text, { key: "yes" })).toBe("Before\nblock content\nAfter");
  });

  it("inline usage without own-line newlines", () => {
    const text = "Hello {{#if name}}{{name}}{{/if}} world";
    expect(processConditionalBlocks(text, { name: "Alice" })).toBe("Hello {{name}} world");
    expect(processConditionalBlocks(text, { name: "" })).toBe("Hello  world");
  });

  it("multiple blocks evaluated independently", () => {
    const text = "{{#if a}}A{{/if}} and {{#if b}}B{{/if}}";
    expect(processConditionalBlocks(text, { a: "yes", b: "" })).toBe("A and ");
    expect(processConditionalBlocks(text, { a: "", b: "yes" })).toBe(" and B");
    expect(processConditionalBlocks(text, { a: "yes", b: "yes" })).toBe("A and B");
  });

  it("guard-only key: 'true' is truthy, '' is falsy", () => {
    const text = "{{#if show}}visible{{/if}}";
    expect(processConditionalBlocks(text, { show: "true" })).toBe("visible");
    expect(processConditionalBlocks(text, { show: "" })).toBe("");
  });

  it("labeled if block: truthy value shows body", () => {
    const text = '{{#if SIG "Include signature"}}Best regards{{/if}}';
    expect(processConditionalBlocks(text, { SIG: "true" })).toBe("Best regards");
  });

  it("labeled if block: falsy value omits body", () => {
    const text = '{{#if SIG "Include signature"}}Best regards{{/if}}';
    expect(processConditionalBlocks(text, { SIG: "" })).toBe("");
  });

  it("labeled if-else block works", () => {
    const text = '{{#if FORMAL "Use formal tone"}}Dear Sir{{#else}}Hey{{/if}}';
    expect(processConditionalBlocks(text, { FORMAL: "true" })).toBe("Dear Sir");
    expect(processConditionalBlocks(text, { FORMAL: "" })).toBe("Hey");
  });

  it("labeled block with newlines cleans up properly", () => {
    const text = 'Before\n{{#if SIG "Include signature"}}\nBest regards\n{{/if}}\nAfter';
    expect(processConditionalBlocks(text, { SIG: "" })).toBe("Before\n\nAfter");
    expect(processConditionalBlocks(text, { SIG: "true" })).toBe("Before\nBest regards\nAfter");
  });

  it("+ prefix on key is stripped for value lookup", () => {
    const text = "{{#if +show}}visible{{/if}}";
    expect(processConditionalBlocks(text, { show: "true" })).toBe("visible");
    expect(processConditionalBlocks(text, { show: "" })).toBe("");
  });

  it("+ prefix with label, key correctly resolved", () => {
    const text = '{{#if +SIG "Include signature"}}Best regards{{/if}}';
    expect(processConditionalBlocks(text, { SIG: "true" })).toBe("Best regards");
    expect(processConditionalBlocks(text, { SIG: "" })).toBe("");
  });
});

describe("processConditionalBlocks — {{/else}} closing tag", () => {
  it("if-else with {{/else}}: truthy value shows if-body", () => {
    const text = "{{#if x}}yes{{#else}}no{{/else}}{{/if}}";
    expect(processConditionalBlocks(text, { x: "true" })).toBe("yes");
  });

  it("if-else with {{/else}}: falsy value shows else-body without {{/else}}", () => {
    const text = "{{#if x}}yes{{#else}}no{{/else}}{{/if}}";
    expect(processConditionalBlocks(text, { x: "" })).toBe("no");
  });

  it("standard {{#else}}...{{/if}} still works (regression check)", () => {
    const text = "{{#if x}}yes{{#else}}no{{/if}}";
    expect(processConditionalBlocks(text, { x: "true" })).toBe("yes");
    expect(processConditionalBlocks(text, { x: "" })).toBe("no");
  });

  it("repeated variable in multiple {{#if}} blocks", () => {
    const text = "{{#if +loop}}/loop {{/if}}Commit {{#if loop}}each round{{#else}}once{{/else}}{{/if}} done";
    expect(processConditionalBlocks(text, { loop: "true" })).toBe("/loop Commit each round done");
    expect(processConditionalBlocks(text, { loop: "" })).toBe("Commit once done");
  });

  it("nested blocks with {{/else}}", () => {
    const text = "{{#if a}}{{#if b}}both{{#else}}a only{{/else}}{{/if}}{{#else}}none{{/else}}{{/if}}";
    expect(processConditionalBlocks(text, { a: "1", b: "1" })).toBe("both");
    expect(processConditionalBlocks(text, { a: "1", b: "" })).toBe("a only");
    expect(processConditionalBlocks(text, { a: "", b: "1" })).toBe("none");
  });

  it("full template pattern with guard, placeholder, and repeated conditional", () => {
    const text =
      "{{#if +loop}}/loop {{!duration|5}}m {{/if}}Commit {{#if loop}}each round{{#else}}once{{/else}}{{/if}} using /gitsummary";
    const placeholders = extractPlaceholders(text);

    // Should extract duration and loop, but NOT /else
    const keys = placeholders.map((p) => p.key);
    expect(keys).toContain("duration");
    expect(keys).toContain("loop");
    expect(keys).not.toContain("/else");

    // loop guard should be defaultOn
    const loopP = placeholders.find((p) => p.key === "loop")!;
    expect(loopP.isGuardOnly).toBe(true);
    expect(loopP.defaultOn).toBe(true);

    // Process with loop enabled
    const withLoop = processConditionalBlocks(text, { loop: "true" });
    const resultOn = replacePlaceholders(withLoop, { duration: "5", loop: "true" }, placeholders);
    expect(resultOn).toBe("/loop 5m Commit each round using /gitsummary");

    // Process with loop disabled
    const withoutLoop = processConditionalBlocks(text, { loop: "" });
    const resultOff = replacePlaceholders(withoutLoop, { duration: "5", loop: "" }, placeholders);
    expect(resultOff).toBe("Commit once using /gitsummary");
  });
});

describe("processConditionalBlocks — nested blocks", () => {
  it("2-level: outer true + inner true shows both bodies", () => {
    const text = "{{#if a}}outer {{#if b}}inner{{/if}}{{/if}}";
    expect(processConditionalBlocks(text, { a: "yes", b: "yes" })).toBe("outer inner");
  });

  it("2-level: outer true + inner false omits inner body", () => {
    const text = "{{#if a}}outer {{#if b}}inner{{/if}}{{/if}}";
    expect(processConditionalBlocks(text, { a: "yes", b: "" })).toBe("outer ");
  });

  it("2-level: outer false skips everything including inner", () => {
    const text = "{{#if a}}outer {{#if b}}inner{{/if}}{{/if}}";
    expect(processConditionalBlocks(text, { a: "", b: "yes" })).toBe("");
  });

  it("nested if-else blocks resolve correctly", () => {
    const text = "{{#if a}}{{#if b}}both{{#else}}a only{{/if}}{{#else}}neither{{/if}}";
    expect(processConditionalBlocks(text, { a: "yes", b: "yes" })).toBe("both");
    expect(processConditionalBlocks(text, { a: "yes", b: "" })).toBe("a only");
    expect(processConditionalBlocks(text, { a: "", b: "yes" })).toBe("neither");
    expect(processConditionalBlocks(text, { a: "", b: "" })).toBe("neither");
  });

  it("3-level deep nesting resolves correctly", () => {
    const text = "{{#if a}}{{#if b}}{{#if c}}all three{{/if}}{{/if}}{{/if}}";
    expect(processConditionalBlocks(text, { a: "1", b: "1", c: "1" })).toBe("all three");
    expect(processConditionalBlocks(text, { a: "1", b: "1", c: "" })).toBe("");
    expect(processConditionalBlocks(text, { a: "1", b: "", c: "1" })).toBe("");
    expect(processConditionalBlocks(text, { a: "", b: "1", c: "1" })).toBe("");
  });

  it("nested blocks with different keys are each resolved independently", () => {
    // Note: {{name}} is a regular placeholder — processConditionalBlocks only resolves
    // the branch structure, not the placeholder substitution inside the chosen branch.
    const text = "{{#if formal}}Dear {{#if name}}{{name}}{{#else}}Sir{{/if}}{{#else}}Hey{{/if}}";
    expect(processConditionalBlocks(text, { formal: "yes", name: "Alice" })).toBe("Dear {{name}}");
    expect(processConditionalBlocks(text, { formal: "yes", name: "" })).toBe("Dear Sir");
    expect(processConditionalBlocks(text, { formal: "", name: "Alice" })).toBe("Hey");
  });

  it("nested blocks mixed with sibling blocks at same level", () => {
    const text = "{{#if a}}A{{/if}} {{#if b}}{{#if c}}BC{{/if}}{{/if}}";
    expect(processConditionalBlocks(text, { a: "1", b: "1", c: "1" })).toBe("A BC");
    expect(processConditionalBlocks(text, { a: "1", b: "1", c: "" })).toBe("A ");
    expect(processConditionalBlocks(text, { a: "", b: "1", c: "1" })).toBe(" BC");
  });

  it("newline cleanup works correctly with nesting", () => {
    const text = "Before\n{{#if a}}\n{{#if b}}\nInner\n{{/if}}\n{{/if}}\nAfter";
    expect(processConditionalBlocks(text, { a: "1", b: "1" })).toBe("Before\nInner\nAfter");
    expect(processConditionalBlocks(text, { a: "1", b: "" })).toBe("Before\n\nAfter");
    expect(processConditionalBlocks(text, { a: "", b: "1" })).toBe("Before\n\nAfter");
  });

  it("malformed/unbalanced tags do not cause infinite loop", () => {
    // Extra {{#if without matching {{/if}} — should terminate within iteration limit
    const text = "{{#if a}}{{#if b}}unclosed{{/if}}";
    expect(() => processConditionalBlocks(text, { a: "1", b: "1" })).not.toThrow();
  });
});

describe("extractPlaceholders — conditional blocks", () => {
  it("guard-only key extracted from {{#if key}} when key not seen elsewhere", () => {
    const text = "{{#if include_sig}}\nBest regards\n{{/if}}";
    const result = extractPlaceholders(text);
    expect(result).toHaveLength(1);
    expect(result[0].key).toBe("include_sig");
    expect(result[0].isGuardOnly).toBe(true);
    expect(result[0].isRequired).toBe(false);
    expect(result[0].isSaved).toBe(false);
  });

  it("guard-only key NOT re-extracted when key already appears as {{key}}", () => {
    const text = "{{#if cc}}\nCC: {{cc}}\n{{/if}}";
    const result = extractPlaceholders(text);
    expect(result).toHaveLength(1);
    expect(result[0].key).toBe("cc");
    expect(result[0].isGuardOnly).toBeUndefined();
  });

  it("{{#if key}} control token not treated as a regular placeholder", () => {
    const text = "{{#if foo}}\ncontent\n{{/if}}";
    const result = extractPlaceholders(text);
    const keys = result.map((p) => p.key);
    expect(keys).not.toContain("#if foo");
    expect(keys).not.toContain("#if");
  });

  it("{{/if}}, {{#else}}, and {{/else}} not extracted as placeholder keys", () => {
    const text = "{{#if x}}\nA\n{{#else}}\nB\n{{/else}}\n{{/if}}";
    const result = extractPlaceholders(text);
    const keys = result.map((p) => p.key);
    expect(keys).not.toContain("/if");
    expect(keys).not.toContain("#else");
    expect(keys).not.toContain("/else");
  });

  it("isGuardOnly is true, isRequired false, isSaved false on guard-only entries", () => {
    const text = "{{#if toggle}}\nyes\n{{/if}}";
    const result = extractPlaceholders(text);
    expect(result[0]).toMatchObject({
      key: "toggle",
      isGuardOnly: true,
      isRequired: false,
      isSaved: false,
    });
  });

  it("labeled guard-only key extracts label correctly", () => {
    const text = '{{#if SIG "Include signature"}}\nBest regards\n{{/if}}';
    const result = extractPlaceholders(text);
    expect(result).toHaveLength(1);
    expect(result[0].key).toBe("SIG");
    expect(result[0].label).toBe("Include signature");
    expect(result[0].isGuardOnly).toBe(true);
  });

  it("unlabeled guard-only key has undefined label (backward compat)", () => {
    const text = "{{#if toggle}}\nyes\n{{/if}}";
    const result = extractPlaceholders(text);
    expect(result[0].label).toBeUndefined();
  });

  it("label with special characters", () => {
    const text = '{{#if TERMS "Accept terms & conditions"}}\nI agree\n{{/if}}';
    const result = extractPlaceholders(text);
    expect(result[0].key).toBe("TERMS");
    expect(result[0].label).toBe("Accept terms & conditions");
  });

  it('key used in both {{#if KEY "label"}} and {{KEY}} is not guard-only, no label', () => {
    const text = '{{#if cc "Show CC field"}}\nCC: {{cc}}\n{{/if}}';
    const result = extractPlaceholders(text);
    expect(result).toHaveLength(1);
    expect(result[0].key).toBe("cc");
    expect(result[0].isGuardOnly).toBeUndefined();
    expect(result[0].label).toBeUndefined();
  });

  it("guard-only key with + prefix sets defaultOn to true", () => {
    const text = "{{#if +include_sig}}\nBest regards\n{{/if}}";
    const result = extractPlaceholders(text);
    expect(result).toHaveLength(1);
    expect(result[0].key).toBe("include_sig");
    expect(result[0].isGuardOnly).toBe(true);
    expect(result[0].defaultOn).toBe(true);
  });

  it("guard-only key without + prefix sets defaultOn to false", () => {
    const text = "{{#if toggle}}\nyes\n{{/if}}";
    const result = extractPlaceholders(text);
    expect(result[0].defaultOn).toBe(false);
  });

  it("guard-only key with + prefix and label", () => {
    const text = '{{#if +SIG "Include signature"}}\nBest regards\n{{/if}}';
    const result = extractPlaceholders(text);
    expect(result).toHaveLength(1);
    expect(result[0].key).toBe("SIG");
    expect(result[0].label).toBe("Include signature");
    expect(result[0].defaultOn).toBe(true);
    expect(result[0].isGuardOnly).toBe(true);
  });

  it("key used in both {{#if +KEY}} and {{KEY}} is not guard-only", () => {
    const text = "{{#if +cc}}\nCC: {{cc}}\n{{/if}}";
    const result = extractPlaceholders(text);
    expect(result).toHaveLength(1);
    expect(result[0].key).toBe("cc");
    expect(result[0].isGuardOnly).toBeUndefined();
    expect(result[0].defaultOn).toBeUndefined();
  });
});

describe("extractConditionalBlockBodies", () => {
  it("extracts a simple if-only block", () => {
    expect(extractConditionalBlockBodies("{{#if k}}A{{/if}}", "k")).toEqual([{ ifBody: "A" }]);
  });

  it("extracts if/else block", () => {
    expect(extractConditionalBlockBodies("{{#if k}}A{{#else}}B{{/if}}", "k")).toEqual([{ ifBody: "A", elseBody: "B" }]);
  });

  it("extracts if/else block with optional {{/else}} closing tag", () => {
    expect(extractConditionalBlockBodies("{{#if k}}A{{#else}}B{{/else}}{{/if}}", "k")).toEqual([
      { ifBody: "A", elseBody: "B" },
    ]);
  });

  it("matches default-on guard with + prefix", () => {
    expect(extractConditionalBlockBodies("{{#if +k}}A{{/if}}", "k")).toEqual([{ ifBody: "A" }]);
  });

  it("matches labeled guard", () => {
    expect(extractConditionalBlockBodies('{{#if k "Label"}}A{{/if}}', "k")).toEqual([{ ifBody: "A" }]);
  });

  it("extracts outer block without descending into nested inner block", () => {
    const text = "{{#if outer}}x{{#if inner}}y{{/if}}z{{/if}}";
    expect(extractConditionalBlockBodies(text, "outer")).toEqual([{ ifBody: "x{{#if inner}}y{{/if}}z" }]);
  });

  it("extracts inner block when querying for inner key", () => {
    const text = "{{#if outer}}x{{#if inner}}y{{/if}}z{{/if}}";
    expect(extractConditionalBlockBodies(text, "inner")).toEqual([{ ifBody: "y" }]);
  });

  it("skips non-matching sibling block", () => {
    const text = "{{#if other}}A{{/if}}{{#if k}}B{{/if}}";
    expect(extractConditionalBlockBodies(text, "k")).toEqual([{ ifBody: "B" }]);
  });

  it("returns multiple entries for repeated same-key sibling blocks", () => {
    const text = "{{#if k}}A{{/if}} {{#if k}}B{{/if}}";
    expect(extractConditionalBlockBodies(text, "k")).toEqual([{ ifBody: "A" }, { ifBody: "B" }]);
  });

  it("returns [] for unterminated block", () => {
    expect(extractConditionalBlockBodies("{{#if k}}A", "k")).toEqual([]);
  });

  it("returns [] when text contains no matching {{#if k}}", () => {
    expect(extractConditionalBlockBodies("plain text {{name}} no blocks", "k")).toEqual([]);
  });

  it("handles {{#if k}} with surrounding text — body excludes tokens themselves", () => {
    const text = "before {{#if k}}middle{{/if}} after";
    expect(extractConditionalBlockBodies(text, "k")).toEqual([{ ifBody: "middle" }]);
  });

  it("handles three siblings in document order", () => {
    const text = "{{#if k}}A{{/if}}-{{#if k}}B{{/if}}-{{#if k}}C{{/if}}";
    expect(extractConditionalBlockBodies(text, "k")).toEqual([{ ifBody: "A" }, { ifBody: "B" }, { ifBody: "C" }]);
  });

  it("returns [] when scanning would run off the end of text without {{/if}}", () => {
    const text = "{{#if k}}A{{#if nested}}B";
    expect(extractConditionalBlockBodies(text, "k")).toEqual([]);
  });

  it("does NOT promote nested {{#else}} to outer block", () => {
    const text = "{{#if outer}}{{#if inner}}{{#else}}fallback{{/if}}{{/if}}";
    expect(extractConditionalBlockBodies(text, "outer")).toEqual([{ ifBody: "{{#if inner}}{{#else}}fallback{{/if}}" }]);
  });
});
