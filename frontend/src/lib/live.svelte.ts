/**
 * Live updates: one EventSource for the whole page. Components subscribe
 * with onChange and refetch what they show; events never carry data, so
 * permissions always apply through the regular API.
 */

import type { ChangeEvent } from '../core/types.js';

type Listener = (e: ChangeEvent) => void;

class Live {
  connected = $state(false);
  private source: EventSource | null = null;
  private listeners = new Set<Listener>();

  start(): void {
    if (this.source) return;
    const source = new EventSource('/api/events');
    source.addEventListener('open', () => (this.connected = true));
    source.addEventListener(
      'error',
      () => (this.connected = source.readyState === EventSource.OPEN),
    );
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
