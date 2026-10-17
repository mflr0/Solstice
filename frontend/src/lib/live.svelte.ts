/**
 * Live updates: one EventSource for the whole page. Components subscribe
 * with onChange and refetch what they show; events never carry data, so
 * permissions always apply through the regular API.
 */

import type { ChangeEvent } from '../core/types.js';

type Listener = (e: ChangeEvent) => void;

/**
 * Wait before reopening the stream after its n-th failure in a row (0 for the
 * first): 5 s, doubling up to 60 s. The server refuses with a 429 when there
 * are too many streams, and the browser then gives up on its own.
 */
export function reconnectDelay(failures: number): number {
  return Math.min(5000 * 2 ** failures, 60000);
}

class Live {
  connected = $state(false);
  private source: EventSource | null = null;
  private listeners = new Set<Listener>();
  private failures = 0;
  private retry: ReturnType<typeof setTimeout> | undefined;

  start(): void {
    if (this.source) return;
    const source = new EventSource('/api/events');
    source.addEventListener('open', () => {
      this.connected = true;
      this.failures = 0;
    });
    source.addEventListener('error', () => {
      this.connected = source.readyState === EventSource.OPEN;
      // Closed by the browser: it will not retry, so reopen it later.
      if (source.readyState !== EventSource.CLOSED || this.retry !== undefined) return;
      this.retry = setTimeout(() => {
        this.retry = undefined;
        this.source = null;
        this.start();
      }, reconnectDelay(this.failures++));
    });
    source.addEventListener('change', (ev) => {
      let data: ChangeEvent;
      try {
        data = JSON.parse((ev as MessageEvent<string>).data) as ChangeEvent;
      } catch {
        return;
      }
      for (const l of this.listeners) l(data);
    });
    this.source = source;
  }

  stop(): void {
    clearTimeout(this.retry);
    this.retry = undefined;
    this.failures = 0;
    this.source?.close();
    this.source = null;
    this.connected = false;
  }

  onChange(fn: Listener): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }
}

export const live = new Live();

/** Calls fn at most once per delay, after the last of a burst of events. */
export function debounce(fn: () => void, delay: number): () => void {
  let timer: ReturnType<typeof setTimeout> | undefined;
  return () => {
    clearTimeout(timer);
    timer = setTimeout(fn, delay);
  };
}
