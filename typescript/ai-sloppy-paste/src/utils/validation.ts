/**
 * Input validation utilities
 */

import { normalizeTag } from "./tags";

export const VALIDATION_LIMITS = {
  TITLE_MAX_LENGTH: 200,
  CONTENT_MAX_LENGTH: 100000, // ~100KB of text
  TAG_MAX_LENGTH: 50,
  TAG_MIN_LENGTH: 1,
  TAG_MAX_DEPTH: 5, // Maximum hierarchy depth (e.g., a/b/c/d/e)
} as const;

export interface ValidationResult {
  isValid: boolean;
  error?: string;
}

/**
 * Validates snippet title
 */
export function validateTitle(title: string): ValidationResult {
  const trimmed = title.trim();

  if (trimmed.length === 0) {
    return { isValid: false, error: "Title is required" };
  }

  if (trimmed.length > VALIDATION_LIMITS.TITLE_MAX_LENGTH) {
    return {
      isValid: false,
      error: `Title must be ${VALIDATION_LIMITS.TITLE_MAX_LENGTH} characters or less (currently ${trimmed.length})`,
    };
  }

  return { isValid: true };
}

/**
 * Validates snippet content
 */
export function validateContent(content: string): ValidationResult {
  const trimmed = content.trim();

  if (trimmed.length === 0) {
    return { isValid: false, error: "Content is required" };
  }

  if (trimmed.length > VALIDATION_LIMITS.CONTENT_MAX_LENGTH) {
    const sizeKB = (trimmed.length / 1000).toFixed(1);
    const maxKB = (VALIDATION_LIMITS.CONTENT_MAX_LENGTH / 1000).toFixed(0);
    return {
      isValid: false,
      error: `Content is too large (${sizeKB}KB). Maximum size is ${maxKB}KB.`,
    };
  }

  return { isValid: true };
}

/**
 * Validates tag name
 * Tags are automatically normalized to lowercase
 */
export function validateTag(tag: string): ValidationResult {
  const trimmed = tag.trim();

  if (trimmed.length === 0) {
    return { isValid: false, error: "Tag name is required" };
  }

  if (trimmed.length < VALIDATION_LIMITS.TAG_MIN_LENGTH) {
    return { isValid: false, error: "Tag name is too short" };
  }

  if (trimmed.length > VALIDATION_LIMITS.TAG_MAX_LENGTH) {
    return {
      isValid: false,
      error: `Tag name must be ${VALIDATION_LIMITS.TAG_MAX_LENGTH} characters or less (currently ${trimmed.length})`,
    };
  }

  // Check for leading/trailing slashes
  if (trimmed.startsWith("/") || trimmed.endsWith("/")) {
    return {
      isValid: false,
      error: "Tag cannot start or end with a slash",
    };
  }

  // Check for consecutive slashes
  if (trimmed.includes("//")) {
    return {
      isValid: false,
      error: "Tag cannot contain consecutive slashes",
    };
  }

  // Allow alphanumeric, hyphens, underscores, and slashes (for hierarchy)
  // No spaces allowed to prevent parsing issues
  const validPattern = /^[a-zA-Z0-9\-_/]+$/;
  if (!validPattern.test(trimmed)) {
    return {
      isValid: false,
      error: "Tag can only contain letters, numbers, hyphens, underscores, and slashes (no spaces)",
    };
  }

  // Check hierarchy depth
  const depth = trimmed.split("/").length;
  if (depth > VALIDATION_LIMITS.TAG_MAX_DEPTH) {
    return {
      isValid: false,
      error: `Tag hierarchy too deep (max ${VALIDATION_LIMITS.TAG_MAX_DEPTH} levels)`,
    };
  }

  // Check each segment is not empty (catches cases like "a//b" that might slip through)
  const segments = trimmed.split("/");
  if (segments.some((segment) => segment.length === 0)) {
    return {
      isValid: false,
      error: "Tag hierarchy segments cannot be empty",
    };
  }

  return { isValid: true };
}

/**
 * Formats byte size to human-readable string
 */
export function formatSize(bytes: number): string {
  if (bytes < 1000) return `${bytes} bytes`;
  if (bytes < 1000000) return `${(bytes / 1000).toFixed(1)} KB`;
  return `${(bytes / 1000000).toFixed(2)} MB`;
}

/**
 * Gets character count info for display
 */
export function getCharacterInfo(text: string, maxLength: number): { count: number; remaining: number; info: string } {
  const count = text.length;
  const remaining = maxLength - count;
  const percentage = (count / maxLength) * 100;

  let info: string;
  if (percentage < 50) {
    info = `${count} / ${maxLength}`;
  } else if (percentage < 90) {
    info = `${count} / ${maxLength} (${remaining} remaining)`;
  } else if (percentage < 100) {
    info = `⚠️ ${remaining} characters remaining`;
  } else {
    info = `❌ Exceeds limit by ${Math.abs(remaining)} characters`;
  }

  return { count, remaining, info };
}
