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
      isSaved: true,
      prefixWrapper: undefined,
      suffixWrapper: undefined,
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
      isSaved: true,
      prefixWrapper: undefined,
      suffixWrapper: undefined,
    });
    expect(result[1]).toEqual({
      key: "id",
      defaultValue: "123",
      isRequired: false,
      isSaved: true,
      prefixWrapper: undefined,
      suffixWrapper: undefined,
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

  it("should handle empty default values with pipe syntax", () => {
    const text = "Hello {{name|}}!";
    const result = extractPlaceholders(text);

    expect(result).toHaveLength(1);
    expect(result[0]).toEqual({
      key: "name",
      defaultValue: "",
      isRequired: false,
      isSaved: true,
      prefixWrapper: undefined,
      suffixWrapper: undefined,
    });
  });

  it("should distinguish between no default and empty default", () => {
    const text = "{{noDefault}} vs {{emptyDefault|}}";
    const result = extractPlaceholders(text);

    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({
      key: "noDefault",
      defaultValue: undefined,
      isRequired: true,
      isSaved: true,
      prefixWrapper: undefined,
      suffixWrapper: undefined,
    });
    expect(result[1]).toEqual({
      key: "emptyDefault",
      defaultValue: "",
      isRequired: false,
      isSaved: true,
      prefixWrapper: undefined,
      suffixWrapper: undefined,
    });
  });

  it("should parse no-save flag with ! prefix", () => {
    const text = "Date: {{!date}}";
    const result = extractPlaceholders(text);

    expect(result).toHaveLength(1);
    expect(result[0]).toEqual({
      key: "date",
      defaultValue: undefined,
      isRequired: true,
      isSaved: false,
      prefixWrapper: undefined,
      suffixWrapper: undefined,
    });
  });

  it("should parse no-save flag with default value", () => {
    const text = "{{!timestamp|123}}";
    const result = extractPlaceholders(text);

    expect(result[0]).toEqual({
      key: "timestamp",
      defaultValue: "123",
      isRequired: false,
      isSaved: false,
      prefixWrapper: undefined,
      suffixWrapper: undefined,
    });
  });

  it("should parse prefix wrapper", () => {
    const text = "Order {{#:id:}}";
    const result = extractPlaceholders(text);

    expect(result).toHaveLength(1);
    expect(result[0]).toEqual({
      key: "id",
      defaultValue: undefined,
      isRequired: true,
      isSaved: true,
      prefixWrapper: "#",
      suffixWrapper: undefined,
    });
  });

  it("should parse suffix wrapper", () => {
    const text = "Price {{:amount:%}}";
    const result = extractPlaceholders(text);

    expect(result[0]).toEqual({
      key: "amount",
      defaultValue: undefined,
      isRequired: true,
      isSaved: true,
      prefixWrapper: undefined,
      suffixWrapper: "%",
    });
  });

  it("should parse both prefix and suffix wrappers", () => {
    const text = "Price {{$:amount: USD}}";
    const result = extractPlaceholders(text);

    expect(result[0]).toEqual({
      key: "amount",
      defaultValue: undefined,
      isRequired: true,
      isSaved: true,
      prefixWrapper: "$",
      suffixWrapper: " USD",
    });
  });

  it("should parse no-save flag with wrappers and default", () => {
    const text = "{{!$:price: USD|0.00}}";
    const result = extractPlaceholders(text);

    expect(result[0]).toEqual({
      key: "price",
      defaultValue: "0.00",
      isRequired: false,
      isSaved: false,
      prefixWrapper: "$",
      suffixWrapper: " USD",
    });
  });

  it("should handle empty wrappers explicitly", () => {
    const text = "{{:key:}}";
    const result = extractPlaceholders(text);

    expect(result[0]).toEqual({
      key: "key",
      defaultValue: undefined,
      isRequired: true,
      isSaved: true,
      prefixWrapper: undefined,
      suffixWrapper: undefined,
    });
  });

  it("should treat invalid colon count as literal key", () => {
    const text = "{{key:value}}"; // Only 1 colon - invalid
    const result = extractPlaceholders(text);

    expect(result[0].key).toBe("key:value");
    expect(result[0].prefixWrapper).toBeUndefined();
    expect(result[0].suffixWrapper).toBeUndefined();
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

  it("should use empty string default when specified with pipe syntax", () => {
    const text = "Hello {{name|}}!";
    const values = {};
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Hello !");
  });

  it("should allow user value to override empty default", () => {
    const text = "Hello {{name|}}!";
    const values = { name: "Alice" };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Hello Alice!");
  });

  it("should apply prefix wrapper when value is non-empty", () => {
    const text = "Order {{#:id:}}";
    const values = { id: "12345" };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Order #12345");
  });

  it("should apply suffix wrapper when value is non-empty", () => {
    const text = "Price {{:amount:%}}";
    const values = { amount: "25" };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Price 25%");
  });

  it("should apply both prefix and suffix wrappers", () => {
    const text = "Price {{$:amount: USD}}";
    const values = { amount: "25.50" };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Price $25.50 USD");
  });

  it("should NOT apply wrappers when value is empty", () => {
    const text = "Message{{with :context:}}";
    const values = { context: "" };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Message");
  });

  it("should NOT apply wrappers when value is whitespace-only", () => {
    const text = "Message{{with :context:}}";
    const values = { context: "   " };
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Message");
  });

  it("should apply wrappers to default values if non-empty", () => {
    const text = "Price {{$:amount: USD|0.00}}";
    const values = {};
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Price $0.00 USD");
  });

  it("should NOT apply wrappers to empty default values", () => {
    const text = "Message{{with :context:| }}";
    const values = {};
    const placeholders = extractPlaceholders(text);

    const result = replacePlaceholders(text, values, placeholders);
    expect(result).toBe("Message");
  });
});
