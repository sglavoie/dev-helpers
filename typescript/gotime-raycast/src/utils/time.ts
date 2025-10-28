/**
 * Time parsing and formatting utilities for GoTime Raycast extension.
 *
 * Supports both 12-hour (AM/PM) and 24-hour time formats.
 */

export interface ParsedTime {
  hours: number;
  minutes: number;
  seconds: number;
}

/**
 * Parse time string to time components.
 *
 * Supports multiple formats:
 * - 24-hour: "14:30", "14:30:00", "9:00"
 * - 12-hour: "2:30 PM", "2:30:00 PM", "9:00 AM"
 *
 * @param input - Time string to parse
 * @returns Parsed time components or null if invalid
 *
 * @example
 * parseTimeInput("14:30")      // { hours: 14, minutes: 30, seconds: 0 }
 * parseTimeInput("2:30 PM")    // { hours: 14, minutes: 30, seconds: 0 }
 * parseTimeInput("09:00")      // { hours: 9, minutes: 0, seconds: 0 }
 * parseTimeInput("12:00 AM")   // { hours: 0, minutes: 0, seconds: 0 }
 * parseTimeInput("12:00 PM")   // { hours: 12, minutes: 0, seconds: 0 }
 */
export function parseTimeInput(input: string): ParsedTime | null {
  const trimmed = input.trim();

  if (!trimmed) {
    return null;
  }

  // 24-hour format: 14:30, 14:30:00, 9:00
  const time24Pattern = /^(\d{1,2}):(\d{2})(?::(\d{2}))?$/;
  // 12-hour format: 2:30 PM, 2:30:00 AM, 9:00 pm
  const time12Pattern = /^(\d{1,2}):(\d{2})(?::(\d{2}))?\s*(AM|PM|am|pm)$/;

  // Try 24-hour format first
  let match = trimmed.match(time24Pattern);
  if (match) {
    const hours = parseInt(match[1], 10);
    const minutes = parseInt(match[2], 10);
    const seconds = match[3] ? parseInt(match[3], 10) : 0;

    if (
      hours >= 0 &&
      hours < 24 &&
      minutes >= 0 &&
      minutes < 60 &&
      seconds >= 0 &&
      seconds < 60
    ) {
      return { hours, minutes, seconds };
    }
    return null; // Invalid values
  }

  // Try 12-hour format
  match = trimmed.match(time12Pattern);
  if (match) {
    let hours = parseInt(match[1], 10);
    const minutes = parseInt(match[2], 10);
    const seconds = match[3] ? parseInt(match[3], 10) : 0;
    const period = match[4].toUpperCase();

    // Validate base values
    if (
      hours < 1 ||
      hours > 12 ||
      minutes < 0 ||
      minutes >= 60 ||
      seconds < 0 ||
      seconds >= 60
    ) {
      return null;
    }

    // Convert 12-hour to 24-hour format
    if (period === "PM" && hours !== 12) {
      hours += 12;
    } else if (period === "AM" && hours === 12) {
      hours = 0;
    }

    return { hours, minutes, seconds };
  }

  return null; // No pattern matched
}

/**
 * Format Date object to time string in 24-hour format.
 *
 * @param date - Date object to format
 * @returns Time string in HH:MM format
 *
 * @example
 * formatTime(new Date("2025-10-28 14:30:00"))  // "14:30"
 * formatTime(new Date("2025-10-28 09:05:00"))  // "09:05"
 */
export function formatTime(date: Date): string {
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${hours}:${minutes}`;
}

/**
 * Format time components to time string in 24-hour format.
 *
 * @param time - Parsed time components
 * @returns Time string in HH:MM or HH:MM:SS format
 *
 * @example
 * formatTimeFromComponents({ hours: 14, minutes: 30, seconds: 0 })  // "14:30"
 * formatTimeFromComponents({ hours: 9, minutes: 5, seconds: 30 })   // "09:05:30"
 */
export function formatTimeFromComponents(time: ParsedTime): string {
  const hours = String(time.hours).padStart(2, "0");
  const minutes = String(time.minutes).padStart(2, "0");

  if (time.seconds > 0) {
    const seconds = String(time.seconds).padStart(2, "0");
    return `${hours}:${minutes}:${seconds}`;
  }

  return `${hours}:${minutes}`;
}

/**
 * Apply time components to a date, preserving the date portion.
 *
 * @param date - Base date to apply time to
 * @param time - Time components to apply
 * @returns New Date object with updated time
 *
 * @example
 * const date = new Date("2025-10-28 10:00:00");
 * const time = { hours: 14, minutes: 30, seconds: 0 };
 * applyTimeToDate(date, time)  // Date("2025-10-28 14:30:00")
 */
export function applyTimeToDate(date: Date, time: ParsedTime): Date {
  const newDate = new Date(date);
  newDate.setHours(time.hours);
  newDate.setMinutes(time.minutes);
  newDate.setSeconds(time.seconds);
  return newDate;
}

/**
 * Validate time string format without parsing.
 *
 * @param input - Time string to validate
 * @returns true if format is valid, false otherwise
 *
 * @example
 * isValidTimeFormat("14:30")     // true
 * isValidTimeFormat("2:30 PM")   // true
 * isValidTimeFormat("25:00")     // false
 * isValidTimeFormat("abc")       // false
 */
export function isValidTimeFormat(input: string): boolean {
  return parseTimeInput(input) !== null;
}

/**
 * Get formatted validation error message for invalid time input.
 *
 * @param input - Invalid time string
 * @returns User-friendly error message
 *
 * @example
 * getTimeValidationError("25:00")    // "Hours must be between 0-23"
 * getTimeValidationError("14:60")    // "Minutes must be between 0-59"
 * getTimeValidationError("abc")      // "Invalid time format. Use: 14:30 or 2:30 PM"
 */
export function getTimeValidationError(input: string): string {
  const trimmed = input.trim();

  if (!trimmed) {
    return "Time is required";
  }

  // Check basic format patterns
  const time24Pattern = /^(\d{1,2}):(\d{2})(?::(\d{2}))?$/;
  const time12Pattern = /^(\d{1,2}):(\d{2})(?::(\d{2}))?\s*(AM|PM|am|pm)$/;

  const match24 = trimmed.match(time24Pattern);
  if (match24) {
    const hours = parseInt(match24[1], 10);
    const minutes = parseInt(match24[2], 10);
    const seconds = match24[3] ? parseInt(match24[3], 10) : 0;

    if (hours < 0 || hours >= 24) {
      return "Hours must be between 0-23 (24-hour format)";
    }
    if (minutes < 0 || minutes >= 60) {
      return "Minutes must be between 0-59";
    }
    if (seconds < 0 || seconds >= 60) {
      return "Seconds must be between 0-59";
    }
  }

  const match12 = trimmed.match(time12Pattern);
  if (match12) {
    const hours = parseInt(match12[1], 10);
    const minutes = parseInt(match12[2], 10);
    const seconds = match12[3] ? parseInt(match12[3], 10) : 0;

    if (hours < 1 || hours > 12) {
      return "Hours must be between 1-12 (12-hour format)";
    }
    if (minutes < 0 || minutes >= 60) {
      return "Minutes must be between 0-59";
    }
    if (seconds < 0 || seconds >= 60) {
      return "Seconds must be between 0-59";
    }
  }

  return "Invalid time format. Use: 14:30 or 2:30 PM";
}
