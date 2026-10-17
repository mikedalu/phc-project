import { ApiError } from './api';

/**
 * One place that decides what a member of staff reads when something fails.
 *
 * ═══ WHY NOT error.message ═══
 *
 * An ApiError from this server carries a sentence written for a human ("This
 * prescription has not been paid for"). A Dexie failure carries
 * "NotFoundError: Failed to execute 'objectStore'..." and a fetch failure
 * carries "Failed to fetch". Showing the second and third to a nurse teaches
 * them that the software is broken and that the message is not worth reading.
 *
 * The rule: the server's own explanation when there is one, a plain sentence
 * otherwise, and never a stack, a class name or an HTTP status on its own.
 */
export function humanError(cause: unknown, fallback = 'Something went wrong. Please try again.'): string {
  if (cause instanceof ApiError) {
    // The API writes its messages for people; trust them, but never a code.
    if (cause.message && !looksTechnical(cause.message)) return cause.message;

    if (cause.code === 'NETWORK_ERROR' || cause.status === 0) {
      return 'The server could not be reached. Your work is saved — try again when the connection is back.';
    }
    if (cause.status === 403) return 'You do not have permission to do that. Ask an administrator.';
    if (cause.status === 404) return 'That record could not be found. It may have been removed.';
    if (cause.status >= 500) return 'The server had a problem. Try again in a moment.';
    return fallback;
  }

  if (cause instanceof Error && cause.message && !looksTechnical(cause.message)) {
    return cause.message;
  }

  return fallback;
}

/**
 * True for text that came from a runtime, not from a person: class names,
 * stack frames, SQLSTATE codes, "undefined", and the like.
 */
function looksTechnical(message: string): boolean {
  return (
    /undefined|null is not|Cannot read propert|Failed to execute|NotFoundError|TypeError|TypeError:|ReferenceError|SQLSTATE|[A-Z_]{6,}:\s|at http|SyntaxError/.test(
      message,
    ) || message.length > 200
  );
}
