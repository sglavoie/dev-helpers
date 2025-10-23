import { describe, it, expect } from "vitest";
import { validateTitle, validateContent, validateTag, getCharacterInfo, VALIDATION_LIMITS } from "./validation";

describe("validateTitle", () => {
  it("should accept valid titles", () => {
    expect(validateTitle("Valid Title")).toEqual({ isValid: true });
    expect(validateTitle("  Trimmed  ")).toEqual({ isValid: true });
  });

  it("should reject empty titles", () => {
    expect(validateTitle("")).toEqual({
      isValid: false,
      error: "Title is required",
    });
    expect(validateTitle("   ")).toEqual({
      isValid: false,
      error: "Title is required",
    });
  });

  it("should reject titles that are too long", () => {
    const longTitle = "a".repeat(VALIDATION_LIMITS.TITLE_MAX_LENGTH + 1);
    const result = validateTitle(longTitle);
    expect(result.isValid).toBe(false);
    expect(result.error).toContain("200 characters or less");
  });
});

describe("validateContent", () => {
  it("should accept valid content", () => {
    expect(validateContent("Valid content")).toEqual({ isValid: true });
    expect(validateContent("  Trimmed content  ")).toEqual({ isValid: true });
  });

  it("should reject empty content", () => {
    expect(validateContent("")).toEqual({
      isValid: false,
      error: "Content is required",
    });
    expect(validateContent("   ")).toEqual({
      isValid: false,
      error: "Content is required",
    });
  });

  it("should reject content that is too large", () => {
    const largeContent = "a".repeat(VALIDATION_LIMITS.CONTENT_MAX_LENGTH + 1);
    const result = validateContent(largeContent);
    expect(result.isValid).toBe(false);
    expect(result.error).toContain("too large");
  });
});

describe("validateTag", () => {
  it("should accept valid tags", () => {
    expect(validateTag("valid-tag")).toEqual({ isValid: true });
    expect(validateTag("Tag_123")).toEqual({ isValid: true });
    expect(validateTag("  trimmed  ")).toEqual({ isValid: true });
  });

  it("should accept hierarchical tags with slashes", () => {
    expect(validateTag("work/projects")).toEqual({ isValid: true });
    expect(validateTag("work/projects/client-a")).toEqual({ isValid: true });
    expect(validateTag("dev/backend/api")).toEqual({ isValid: true });
  });

  it("should reject empty tags", () => {
    expect(validateTag("")).toEqual({
      isValid: false,
      error: "Tag name is required",
    });
  });

  it("should reject tags that are too long", () => {
    const longTag = "a".repeat(VALIDATION_LIMITS.TAG_MAX_LENGTH + 1);
    const result = validateTag(longTag);
    expect(result.isValid).toBe(false);
    expect(result.error).toContain("50 characters or less");
  });

  it("should reject tags with invalid characters", () => {
    expect(validateTag("invalid@tag").isValid).toBe(false);
    expect(validateTag("invalid#tag").isValid).toBe(false);
    expect(validateTag("invalid.tag").isValid).toBe(false);
  });

  it("should reject tags with spaces", () => {
    expect(validateTag("invalid tag").isValid).toBe(false);
    expect(validateTag("work project").isValid).toBe(false);
  });

  it("should accept tags with valid special characters", () => {
    expect(validateTag("valid-tag")).toEqual({ isValid: true });
    expect(validateTag("valid_tag")).toEqual({ isValid: true });
  });

  it("should reject tags with leading or trailing slashes", () => {
    expect(validateTag("/invalid").isValid).toBe(false);
    expect(validateTag("invalid/").isValid).toBe(false);
    expect(validateTag("/invalid/tag").isValid).toBe(false);
    expect(validateTag("invalid/tag/").isValid).toBe(false);
  });

  it("should reject tags with consecutive slashes", () => {
    expect(validateTag("invalid//tag").isValid).toBe(false);
    expect(validateTag("work///projects").isValid).toBe(false);
  });

  it("should reject tags exceeding hierarchy depth limit", () => {
    const deepTag = "a/b/c/d/e/f"; // 6 levels (max is 5)
    const result = validateTag(deepTag);
    expect(result.isValid).toBe(false);
    expect(result.error).toContain("hierarchy too deep");
  });

  it("should accept tags at maximum hierarchy depth", () => {
    const maxDepthTag = "a/b/c/d/e"; // 5 levels (exactly at limit)
    expect(validateTag(maxDepthTag)).toEqual({ isValid: true });
  });
});

describe("getCharacterInfo", () => {
  it("should return correct character counts", () => {
    const result = getCharacterInfo("hello", 100);
    expect(result.count).toBe(5);
    expect(result.remaining).toBe(95);
  });

  it("should show simple info when below 50%", () => {
    const result = getCharacterInfo("hello", 100);
    expect(result.info).toBe("5 / 100");
  });

  it("should show remaining count when 50-90%", () => {
    const result = getCharacterInfo("a".repeat(60), 100);
    expect(result.info).toContain("remaining");
  });

  it("should show warning when 90-100%", () => {
    const result = getCharacterInfo("a".repeat(95), 100);
    expect(result.info).toContain("⚠️");
  });

  it("should show error when exceeding limit", () => {
    const result = getCharacterInfo("a".repeat(110), 100);
    expect(result.info).toContain("❌");
    expect(result.info).toContain("Exceeds limit");
  });
});
