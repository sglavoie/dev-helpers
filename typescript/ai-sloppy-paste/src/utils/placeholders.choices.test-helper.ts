import { describe, expect, it } from "vitest";
import { parsePlaceholderSyntax } from "./placeholderSyntax";
import { extractPlaceholders, replacePlaceholders } from "./placeholders";

describe("authored placeholder choices", () => {
  it("extracts choices while preserving explicit-default semantics", () => {
    expect(extractPlaceholders("{{tone[Formal|Casual|Technical]}}")[0]).toEqual({
      key: "tone",
      choices: ["Formal", "Casual", "Technical"],
      defaultValue: undefined,
      isRequired: true,
      isSaved: true,
      prefixWrapper: undefined,
      suffixWrapper: undefined,
    });
    expect(extractPlaceholders("{{tone[Formal|Casual]|Casual}}")[0]).toMatchObject({
      choices: ["Formal", "Casual"],
      defaultValue: "Casual",
      isRequired: false,
    });
    expect(extractPlaceholders("{{tone[Formal|Casual]|Custom}}")[0]).toMatchObject({
      defaultValue: "Custom",
    });
    expect(extractPlaceholders("{{tone[Formal|Casual]|}}")[0]).toMatchObject({
      defaultValue: "",
      isRequired: false,
    });
  });

  it("combines no-save, wrappers, choices, and a default", () => {
    expect(extractPlaceholders("{{!$:amount[10|20]: USD|10}}")[0]).toEqual({
      key: "amount",
      choices: ["10", "20"],
      defaultValue: "10",
      isRequired: false,
      isSaved: false,
      prefixWrapper: "$",
      suffixWrapper: " USD",
    });
  });

  it("trims and decodes choices without treating colons as wrapper separators", () => {
    const escaped = extractPlaceholders("{{value[ A\\|B | \\[C\\] | D\\\\E ]}}")[0];
    expect(escaped.choices).toEqual(["A|B", "[C]", "D\\E"]);

    const withColons = extractPlaceholders("{{mode[fast:cheap|slow:careful]}}")[0];
    expect(withColons.key).toBe("mode");
    expect(withColons.choices).toEqual(["fast:cheap", "slow:careful"]);
  });

  it("lets a choice declaration own metadata regardless of plain-reference order", () => {
    const before = extractPlaceholders("{{tone|legacy}} {{tone[Formal|Casual]|Casual}} {{tone}}");
    const after = extractPlaceholders("{{tone[Formal|Casual]|Casual}} {{tone|legacy}}");

    expect(before).toHaveLength(1);
    expect(before[0]).toMatchObject({
      key: "tone",
      choices: ["Formal", "Casual"],
      defaultValue: "Casual",
      isSaved: true,
      isRequired: false,
    });
    expect(after[0]).toMatchObject(before[0]);
  });

  it("uses occurrence-specific wrappers for declarations and plain references", () => {
    const text = "{{$:amount[10|20]: USD|10}} / {{~:amount[10|20]:kg|10}} / {{amount}}";
    const placeholders = extractPlaceholders(text);

    expect(parsePlaceholderSyntax(text).diagnostics).toEqual([]);
    expect(replacePlaceholders(text, { amount: "25" }, placeholders)).toBe("$25 USD / ~25kg / 25");
    expect(replacePlaceholders(text, { amount: "   " }, placeholders)).toBe(" /  / ");
  });

  it("reports every declaration when field-level metadata conflicts", () => {
    const text = "{{tone[A|B]}} {{tone[A|C]}} {{!tone[A|B]}}";
    const diagnostics = parsePlaceholderSyntax(text).diagnostics;

    expect(diagnostics).toHaveLength(3);
    expect(diagnostics.map((diagnostic) => diagnostic.expression)).toEqual([
      "{{tone[A|B]}}",
      "{{tone[A|C]}}",
      "{{!tone[A|B]}}",
    ]);
    expect(diagnostics.every((diagnostic) => diagnostic.message.includes("Conflicting authored choices"))).toBe(true);
  });

  it("allows wrapper text to differ but diagnoses required/optional disagreement", () => {
    expect(parsePlaceholderSyntax("{{$:amount[10|20]: USD}} {{~:amount[10|20]:kg}}").diagnostics).toEqual([]);

    const diagnostics = parsePlaceholderSyntax("{{amount[10|20]}} {{$:amount[10|20]: USD}}").diagnostics;
    expect(diagnostics).toHaveLength(2);
    expect(diagnostics[0].message).toContain("required/optional status");
  });

  it("returns raw expressions and exact source ranges", () => {
    const text = "Before {{tone[A|B]}} after";
    const occurrence = parsePlaceholderSyntax(text).occurrences[0];

    expect(occurrence.raw).toBe("{{tone[A|B]}}");
    expect(text.slice(occurrence.range.start, occurrence.range.end)).toBe(occurrence.raw);
    expect(occurrence.hasExplicitDefault).toBe(false);
  });

  it("keeps brackets in ordinary defaults and wrappers on the legacy path", () => {
    const defaultText = "{{key|[none]}}";
    expect(parsePlaceholderSyntax(defaultText).diagnostics).toEqual([]);
    expect(extractPlaceholders(defaultText)[0]).toMatchObject({ key: "key", defaultValue: "[none]" });
    expect(replacePlaceholders(defaultText, {}, extractPlaceholders(defaultText))).toBe("[none]");

    const wrapperText = "{{pre[fix]:key:suf[fix]}}";
    expect(parsePlaceholderSyntax(wrapperText).diagnostics).toEqual([]);
    expect(extractPlaceholders(wrapperText)[0]).toMatchObject({
      key: "key",
      prefixWrapper: "pre[fix]",
      suffixWrapper: "suf[fix]",
    });
  });

  it("diagnoses authored choices combined with a bracket-containing prefix", () => {
    const choiceText = "{{pre[fix]:key[a|b]:suffix}}";
    const parsedChoice = parsePlaceholderSyntax(choiceText);

    expect(parsedChoice.diagnostics).toHaveLength(1);
    expect(parsedChoice.diagnostics[0].message).toContain(
      "brackets in a prefix wrapper cannot be combined with authored choices",
    );
    expect(parsedChoice.diagnostics[0].message).toContain("remove the brackets from the prefix wrapper");
    expect(() => replacePlaceholders(choiceText, {}, extractPlaceholders(choiceText))).not.toThrow();

    const legacyText = "{{pre[fix]:key:suffix}}";
    const parsedLegacy = parsePlaceholderSyntax(legacyText);
    expect(parsedLegacy.diagnostics).toEqual([]);
    expect(parsedLegacy.occurrences[0]).toMatchObject({
      key: "key",
      prefixWrapper: "pre[fix]",
      suffixWrapper: "suffix",
      isChoiceDeclaration: false,
    });
  });

  it("diagnoses brackets in choice suffix wrappers and default values precisely", () => {
    const suffixText = "{{pre:key[a|b]:suf[fix]}}";
    const suffixDiagnostic = parsePlaceholderSyntax(suffixText).diagnostics[0];
    expect(suffixDiagnostic.message).toContain("brackets in a suffix wrapper");
    expect(suffixDiagnostic.message).toContain("remove the brackets from the suffix wrapper");
    expect(suffixDiagnostic.message).not.toContain("only one choice list is allowed");

    const defaultText = "{{key[a|b]|c[d]}}";
    const defaultDiagnostic = parsePlaceholderSyntax(defaultText).diagnostics[0];
    expect(defaultDiagnostic.message).toContain("brackets in a default value");
    expect(defaultDiagnostic.message).toContain("remove the brackets from the default value");
    expect(defaultDiagnostic.message).not.toContain("only one choice list is allowed");

    const secondListDiagnostic = parsePlaceholderSyntax("{{key[a|b][c|d]}}").diagnostics[0];
    expect(secondListDiagnostic.message).toContain("only one choice list is allowed");
  });

  it("gives actionable guidance for an unmatched closing bracket in a key", () => {
    const diagnostic = parsePlaceholderSyntax("{{tone]}}").diagnostics[0];

    expect(diagnostic.message).toContain("brackets are reserved in choice-capable key positions");
    expect(diagnostic.message).toContain("rename the placeholder key");
    expect(diagnostic.message).not.toContain("escape a literal bracket as");
  });

  it("diagnoses unmatched closing brackets in choice suffix wrappers and default values precisely", () => {
    const suffixDiagnostic = parsePlaceholderSyntax("{{pre:key[a|b]:suf]}}").diagnostics[0];
    expect(suffixDiagnostic.message).toContain("brackets in a suffix wrapper");
    expect(suffixDiagnostic.message).toContain("remove the brackets from the suffix wrapper");
    expect(suffixDiagnostic.message).not.toContain("rename the placeholder key");

    const defaultDiagnostic = parsePlaceholderSyntax("{{key[a|b]|c]}}").diagnostics[0];
    expect(defaultDiagnostic.message).toContain("brackets in a default value");
    expect(defaultDiagnostic.message).toContain("remove the brackets from the default value");
    expect(defaultDiagnostic.message).not.toContain("rename the placeholder key");

    const keyDiagnostic = parsePlaceholderSyntax("{{ke]y[a|b]}}").diagnostics[0];
    expect(keyDiagnostic.message).toContain("brackets are reserved in choice-capable key positions");
    expect(keyDiagnostic.message).toContain("rename the placeholder key");

    expect(parsePlaceholderSyntax("{{key|c]}}").diagnostics).toEqual([]);
    expect(parsePlaceholderSyntax("{{pre:key:suf]|x}}").diagnostics).toEqual([]);
  });

  it.each([
    "{{tone[]}}",
    "{{tone[Only]}}",
    "{{tone[A||B]}}",
    "{{tone[A|A]}}",
    "{{tone[A|B}}",
    "{{tone]}}",
    "{{tone[A[B|C]}}",
    "{{tone[A|B\\}}",
    "{{tone[A|B\\q]}}",
    "{{prefix:tone[A|B]}}",
  ])("diagnoses malformed choices and keeps runtime fallback safe: %s", (text) => {
    const parsed = parsePlaceholderSyntax(text);
    const fallback = extractPlaceholders(text);

    expect(parsed.diagnostics.length).toBeGreaterThan(0);
    expect(parsed.diagnostics[0].range.start).toBe(0);
    expect(() => replacePlaceholders(text, {}, fallback)).not.toThrow();
  });
});
