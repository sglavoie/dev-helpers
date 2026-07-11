import { Clipboard } from "@raycast/api";
import { PlaceholderValueToRecord, recordSnippetUse } from "./storage";

type BestEffortOperation = () => unknown | Promise<unknown>;
type FailureHandler = (error: unknown) => unknown | Promise<unknown>;

interface SnippetActionOptions<T> {
  prepare: () => T | Promise<T>;
  primaryOperation: (prepared: T) => Promise<void>;
  snippetId: string;
  placeholderValues?: PlaceholderValueToRecord[];
  onPreparationFailure: FailureHandler;
  onPrimaryFailure: FailureHandler;
}

interface CopySnippetContentOptions {
  snippetId: string;
  content: string;
  onPrimaryFailure: FailureHandler;
}

/**
 * Runs an action whose failure must not change the outcome of a successful
 * clipboard operation. Errors are intentionally logged without the original
 * error object because it might contain snippet content or placeholder values.
 */
export async function runBestEffort(operation: BestEffortOperation, failureLogMessage: string): Promise<void> {
  try {
    await operation();
  } catch {
    console.error(failureLogMessage);
  }
}

/**
 * Records metadata after a successful clipboard action without exposing
 * snippet content or placeholder values if storage is unavailable.
 */
export async function recordSnippetUseBestEffort(
  snippetId: string,
  placeholderValues: PlaceholderValueToRecord[] = [],
): Promise<void> {
  await runBestEffort(
    () => recordSnippetUse(snippetId, placeholderValues),
    `Unable to record snippet use: ${snippetId}`,
  );
}

/**
 * Keeps preparation and the single primary clipboard operation as explicit
 * failure boundaries. Tracking runs only after the primary operation succeeds.
 */
export async function runSnippetAction<T>({
  prepare,
  primaryOperation,
  snippetId,
  placeholderValues,
  onPreparationFailure,
  onPrimaryFailure,
}: SnippetActionOptions<T>): Promise<boolean> {
  let prepared: T;
  try {
    prepared = await prepare();
  } catch (error) {
    await runBestEffort(() => onPreparationFailure(error), "Unable to show snippet preparation failure");
    return false;
  }

  try {
    await primaryOperation(prepared);
  } catch (error) {
    await runBestEffort(() => onPrimaryFailure(error), "Unable to show primary clipboard failure");
    return false;
  }

  await recordSnippetUseBestEffort(snippetId, placeholderValues);
  return true;
}

/**
 * Shared policy for secondary views that copy full stored snippet content.
 * Clipboard metadata helpers (titles, analytics summaries, syntax examples,
 * and form helpers) deliberately do not use this path and remain untracked.
 */
export async function copySnippetContent({
  snippetId,
  content,
  onPrimaryFailure,
}: CopySnippetContentOptions): Promise<boolean> {
  return runSnippetAction({
    prepare: () => content,
    primaryOperation: (snippetContent) => Clipboard.copy(snippetContent),
    snippetId,
    onPreparationFailure: () => undefined,
    onPrimaryFailure,
  });
}
