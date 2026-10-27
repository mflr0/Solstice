/**
 * Live updates: one EventSource for all the tabs of the browser. Components
 * subscribe with onChange and refetch what they show; events never carry
 * data, so permissions always apply through the regular API.
 *
 * Over HTTP/1.1 a browser opens at most 6 connections per host, shared by
 * all its tabs, and each stream holds one for good: with a stream per tab,
 * the seventh tab would not even load. So the tabs elect a leader with a
 * Web Lock; it holds the stream and relays its events on a BroadcastChannel.
 * The lock is freed when the leader's tab closes or reloads, and the next
 * tab in line takes over.
 */

import type { ChangeEvent } from '../core/types.js';

type Listener = (e: ChangeEvent) => void;

type Message =
  { type: 'change'; event: ChangeEvent } | { type: 'connected'; value: boolean } | { type: 'ask' };

const lockName = 'solstice-live';

/**
 * The server refuses with a 429 when there are too many streams, and the
 * browser then gives up on its own, so the stream must be reopened later.
 */
export function reconnectDelay(failures: number): number {
  return Math.min(5000 * 2 ** failures, 60000);
}

function canShare(): boolean {
  return (
    typeof BroadcastChannel !== 'undefined' && typeof navigator !== 'undefined' && !!navigator.locks
  );
}

/** Live is exported for tests; the page uses the live singleton. */
export class Live {
  connected = $state(false);
  private source: EventSource | null = null;
  // eslint-disable-next-line svelte/prefer-svelte-reactivity -- listeners, not state
  private listeners = new Set<Listener>();
  private failures = 0;
  private retry: ReturnType<typeof setTimeout> | undefined;
  private channel: BroadcastChannel | null = null;
  private abort: AbortController | null = null;
  private unlock: (() => void) | null = null;

  start(): void {
    if (this.source || this.channel) return;
    if (!canShare()) {
      this.open();
      return;
    }
    const channel = new BroadcastChannel(lockName);
    channel.onmessage = (ev: MessageEvent<Message>) => this.receive(ev.data);
    this.channel = channel;
    const abort = new AbortController();
    this.abort = abort;
    navigator.locks
      .request(lockName, { signal: abort.signal }, () => {
        if (this.abort !== abort) return;
        return new Promise<void>((resolve) => {
          this.unlock = resolve;
          this.open();
        });
      })
      // Aborted by stop() before the lock came.
      .catch(() => {});
    channel.postMessage({ type: 'ask' } satisfies Message);
  }

  stop(): void {
    clearTimeout(this.retry);
    this.retry = undefined;
    this.failures = 0;
    if (this.source) {
      this.source.close();
      this.source = null;
      this.post({ type: 'connected', value: false });
    }
    this.abort?.abort();
    this.abort = null;
    this.unlock?.();
    this.unlock = null;
    this.channel?.close();
    this.channel = null;
    this.connected = false;
  }

  onChange(fn: Listener): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  private open(): void {
    const source = new EventSource('/api/events');
    source.addEventListener('open', () => {
      this.failures = 0;
      this.setConnected(true);
    });
    source.addEventListener('error', () => {
      this.setConnected(source.readyState === EventSource.OPEN);
      // Closed by the browser: it will not retry, so reopen it later.
      if (source.readyState !== EventSource.CLOSED || this.retry !== undefined) return;
      this.retry = setTimeout(() => {
        this.retry = undefined;
        this.open();
      }, reconnectDelay(this.failures++));
    });
    source.addEventListener('change', (ev) => {
      let data: ChangeEvent;
      try {
        data = JSON.parse((ev as MessageEvent<string>).data) as ChangeEvent;
      } catch {
        return;
      }
      this.post({ type: 'change', event: data });
      this.emit(data);
    });
    this.source = source;
  }

  private receive(msg: Message): void {
    switch (msg.type) {
      case 'change':
        this.emit(msg.event);
        break;
      case 'connected':
        this.connected = msg.value;
        break;
      case 'ask':
        if (this.source) this.post({ type: 'connected', value: this.connected });
        break;
    }
  }

  private setConnected(value: boolean): void {
    this.connected = value;
    this.post({ type: 'connected', value });
  }

  private post(msg: Message): void {
    this.channel?.postMessage(msg);
  }

  private emit(e: ChangeEvent): void {
    for (const l of this.listeners) l(e);
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
