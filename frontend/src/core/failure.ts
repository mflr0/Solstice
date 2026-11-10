/**
 * What to tell a person when a request fails without a message of the
 * server's own: shared by the application's client and the setup wizard's,
 * each with its own catalogue.
 */

/** The texts for failed requests, as both catalogues hold them under `errors`. */
export interface FailureMessages {
  network: string;
  sessionEnded: string;
  forbidden: string;
  missing: string;
  tooLarge: string;
  tooMany: (seconds: number | null) => string;
  server: string;
  generic: string;
}

/**
 * The message for an answer of this status (0: no answer) that brought no
 * readable message. A success whose body can't be read came from something
 * other than Solstice (a proxy), so the server counts as not reached.
 */
export function failureMessage(
  m: FailureMessages,
  status: number,
  retryAfter: string | null = null,
  now = Date.now(),
): string {
  if (status < 400 || status === 502 || status === 503 || status === 504) return m.network;
  switch (status) {
    case 401:
      return m.sessionEnded;
    case 403:
      return m.forbidden;
    case 404:
      return m.missing;
    case 413:
      return m.tooLarge;
    case 429:
      return m.tooMany(retrySeconds(retryAfter, now));
  }
  return status >= 500 ? m.server : m.generic;
}

/** A Retry-After header (seconds, or an HTTP date) as seconds from now; null when absent or past. */
export function retrySeconds(header: string | null, now = Date.now()): number | null {
  const v = header?.trim();
  if (!v) return null;
  const s = /^\d+$/.test(v) ? Number(v) : Math.ceil((Date.parse(v) - now) / 1000);
  return Number.isFinite(s) && s > 0 ? s : null;
}

/** The `{error}` of an error body, when it is Solstice's JSON. */
export function serverMessage(data: unknown): string | null {
  return data && typeof data === 'object' && 'error' in data && typeof data.error === 'string'
    ? data.error
    : null;
}

/** A body read as JSON, or undefined when it is empty or not JSON. */
export function parseBody(text: string): unknown {
  try {
    return text ? (JSON.parse(text) as unknown) : undefined;
  } catch {
    return undefined;
  }
}
