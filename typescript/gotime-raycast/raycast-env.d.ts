/// <reference types="@raycast/api">

/* 🚧 🚧 🚧
 * This file is auto-generated from the extension's manifest.
 * Do not modify manually. Instead, update the `package.json` file.
 * 🚧 🚧 🚧 */

/* eslint-disable @typescript-eslint/ban-types */

type ExtensionPreferences = {
  /** Days to Show - Number of days to show when listing timers for continue/delete operations */
  "daysToShow": string
}

/** Preferences accessible in all the extension's commands */
declare type Preferences = ExtensionPreferences

declare namespace Preferences {
  /** Preferences accessible in the `gotime` command */
  export type Gotime = ExtensionPreferences & {}
  /** Preferences accessible in the `active-timers` command */
  export type ActiveTimers = ExtensionPreferences & {}
  /** Preferences accessible in the `weekly-report` command */
  export type WeeklyReport = ExtensionPreferences & {}
  /** Preferences accessible in the `start-timer` command */
  export type StartTimer = ExtensionPreferences & {}
  /** Preferences accessible in the `continue-timer` command */
  export type ContinueTimer = ExtensionPreferences & {}
  /** Preferences accessible in the `delete-timer` command */
  export type DeleteTimer = ExtensionPreferences & {}
  /** Preferences accessible in the `set-entry` command */
  export type SetEntry = ExtensionPreferences & {}
  /** Preferences accessible in the `list-entries` command */
  export type ListEntries = ExtensionPreferences & {}
}

declare namespace Arguments {
  /** Arguments passed to the `gotime` command */
  export type Gotime = {}
  /** Arguments passed to the `active-timers` command */
  export type ActiveTimers = {}
  /** Arguments passed to the `weekly-report` command */
  export type WeeklyReport = {}
  /** Arguments passed to the `start-timer` command */
  export type StartTimer = {}
  /** Arguments passed to the `continue-timer` command */
  export type ContinueTimer = {}
  /** Arguments passed to the `delete-timer` command */
  export type DeleteTimer = {}
  /** Arguments passed to the `set-entry` command */
  export type SetEntry = {}
  /** Arguments passed to the `list-entries` command */
  export type ListEntries = {}
}

