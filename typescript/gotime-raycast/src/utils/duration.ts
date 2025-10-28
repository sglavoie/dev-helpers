/**
 * Duration parsing and formatting utilities for GoTime Raycast extension.
 *
 * Parsing rules:
 * - Pure numbers are treated as MINUTES (e.g., "90" = 90 minutes = 5400 seconds)
 * - Supports human-readable formats: "1h30m", "90m", "2h"
 * - Returns duration in seconds (CLI format)
 */

/**
 * Parse duration string to seconds.
 *
 * @param input - Duration string (e.g., "1h30m", "90", "2h")
 * @returns Duration in seconds
 * @throws Error if input format is invalid
 *
 * @example
 * parseDuration("90")      // 5400 (90 minutes)
 * parseDuration("1h30m")   // 5400
 * parseDuration("2h")      // 7200
 * parseDuration("45m")     // 2700
 */
export function parseDuration(input: string): number {
  if (!input || input.trim() === "") {
    throw new Error("Duration cannot be empty");
  }

  const trimmed = input.trim();

  // Check if it's a pure number (treat as minutes)
  if (/^\d+$/.test(trimmed)) {
    const minutes = parseInt(trimmed, 10);
    if (isNaN(minutes) || minutes < 0) {
      throw new Error("Invalid duration: must be a positive number");
    }
    return minutes * 60; // Convert minutes to seconds
  }

  // Parse human-readable format (e.g., "1h30m", "2h", "45m")
  const hourMatch = trimmed.match(/(\d+)h/);
  const minuteMatch = trimmed.match(/(\d+)m/);

  if (!hourMatch && !minuteMatch) {
    throw new Error(
      'Invalid duration format. Use: "1h30m", "90m", "2h", or "90" (minutes)',
    );
  }

  const hours = hourMatch ? parseInt(hourMatch[1], 10) : 0;
  const minutes = minuteMatch ? parseInt(minuteMatch[1], 10) : 0;

  if (hours < 0 || minutes < 0) {
    throw new Error("Invalid duration: hours and minutes must be positive");
  }

  if (minutes >= 60) {
    throw new Error("Invalid duration: minutes must be less than 60");
  }

  const totalSeconds = hours * 3600 + minutes * 60;

  if (totalSeconds === 0) {
    throw new Error("Invalid duration: must be greater than 0");
  }

  return totalSeconds;
}

/**
 * Format seconds into human-readable duration string.
 *
 * @param seconds - Duration in seconds
 * @returns Formatted duration string (e.g., "1h 30m", "45m", "2h")
 *
 * @example
 * formatDuration(5400)  // "1h 30m"
 * formatDuration(7200)  // "2h"
 * formatDuration(2700)  // "45m"
 * formatDuration(90)    // "1m"
 */
export function formatDuration(seconds: number): string {
  if (seconds < 0) {
    return "0m";
  }

  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  if (hours > 0 && minutes > 0) {
    return `${hours}h ${minutes}m`;
  } else if (hours > 0) {
    return `${hours}h`;
  } else if (minutes > 0) {
    return `${minutes}m`;
  } else {
    return "< 1m";
  }
}

/**
 * Format seconds into compact duration string without spaces.
 *
 * @param seconds - Duration in seconds
 * @returns Compact formatted duration string (e.g., "1h30m", "45m", "2h")
 *
 * @example
 * formatDurationCompact(5400)  // "1h30m"
 * formatDurationCompact(7200)  // "2h"
 * formatDurationCompact(2700)  // "45m"
 */
export function formatDurationCompact(seconds: number): string {
  if (seconds < 0) {
    return "0m";
  }

  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  if (hours > 0 && minutes > 0) {
    return `${hours}h${minutes}m`;
  } else if (hours > 0) {
    return `${hours}h`;
  } else if (minutes > 0) {
    return `${minutes}m`;
  } else {
    return "<1m";
  }
}

/**
 * Calculate duration in seconds between two dates.
 *
 * @param startDate - Start date/time
 * @param endDate - End date/time
 * @returns Duration in seconds
 * @throws Error if end date is before start date
 *
 * @example
 * const start = new Date("2025-10-28 09:00:00");
 * const end = new Date("2025-10-28 10:30:00");
 * calculateDuration(start, end)  // 5400
 */
export function calculateDuration(startDate: Date, endDate: Date): number {
  const durationMs = endDate.getTime() - startDate.getTime();

  if (durationMs < 0) {
    throw new Error("End time must be after start time");
  }

  return Math.floor(durationMs / 1000);
}
