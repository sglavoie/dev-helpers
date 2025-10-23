import { describe, it, expect } from "vitest";
import { extractPlaceholders, replacePlaceholders } from "./placeholders";

describe("extractPlaceholders", () => {
  it("should extract basic placeholders", () => {
    const text = "Hello {{name}}, welcome!";
    const result = extractPlaceholders(text);

    expect(result).toHaveLength(1);
    expect(result[0]).toEqual({
      key: "name",
      defaultValue: undefined,
      isRequired: true,
    });
  });

  it("should extract placeholders with default values", () => {
    const text = "Hello {{name|John}}, your ID is {{id|123}}";
    const result = extractPlaceholders(text);

    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({
      key: "name",
      defaultValue: "John",
      isRequired: false,
    });
    expect(result[1]).toEqual({
      key: "id",
      defaultValue: "123",
      isRequired: false,
    });
  });

  it("should handle duplicate placeholders", () => {
    const text = "{{name}} and {{name}} again";
    const result = extractPlaceholders(text);

    expect(result).toHaveLength(1);
    expect(result[0].key).toBe("name");
  });

  it("should handle mixed required and optional placeholders", () => {
    const text = "{{required}} and {{optional|default}}";
    const result = extractPlaceholders(text);

    expect(result).toHaveLength(2);
    expect(result[0].isRequired).toBe(true);
    expect(result[1].isRequired).toBe(false);
  });

  it("should handle no placeholders", () => {
    const text = "No placeholders here";
    const result = extractPlaceholders(text);

    expect(result).toHaveLength(0);
  });

  it("should trim whitespace from keys and defaults", () => {
    const text = "{{ name | John Doe }}";
    const result = extractPlaceholders(text);

    expect(result[0].key).toBe("name");
    expect(result[0].defaultValue).toBe("John Doe");
  });
});

describe("replacePlaceholders", () => {
  it("should replace basic placeholders", () => {
    const text = "Hello {{name}}!";
    const values = { name: "Alice" };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Hello Alice!");
  });

  it("should use default values when no value provided", () => {
    const text = "Hello {{name|Guest}}!";
    const values = {};
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Hello Guest!");
  });

  it("should prefer provided values over defaults", () => {
    const text = "Hello {{name|Guest}}!";
    const values = { name: "Alice" };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Hello Alice!");
  });

  it("should replace multiple occurrences of same placeholder", () => {
    const text = "{{name}} and {{name}} again";
    const values = { name: "Alice" };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Alice and Alice again");
  });

  it("should replace multiple different placeholders", () => {
    const text = "{{first}} {{last}}";
    const values = { first: "John", last: "Doe" };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("John Doe");
  });

  it("should handle placeholders with special regex characters in keys", () => {
    const text = "{{my-variable}} test";
    const values = { "my-variable": "value" };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("value test");
  });

  it("should use empty string when no value and no default", () => {
    const text = "Hello {{name}}!";
    const values = {};
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Hello !");
  });
});
