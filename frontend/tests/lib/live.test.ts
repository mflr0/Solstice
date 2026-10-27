import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { ChangeEvent } from '../../src/core/types.js';
import { Live, reconnectDelay } from '../../src/lib/live.svelte.js';

describe('reconnectDelay', () => {
  it('starts at 5 s and doubles', () => {
    expect([0, 1, 2, 3].map(reconnectDelay)).toEqual([5000, 10000, 20000, 40000]);
  });

  it('never goes past 60 s', () => {
    expect(reconnectDelay(4)).toBe(60000);
    expect(reconnectDelay(50)).toBe(60000);
  });
});

class FakeEventSource extends EventTarget {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;
  static opened: FakeEventSource[] = [];
  readyState = FakeEventSource.CONNECTING;

  constructor(readonly url: string) {
    super();
    FakeEventSource.opened.push(this);
  }

  close(): void {
    this.readyState = FakeEventSource.CLOSED;
  }

  connect(): void {
    this.readyState = FakeEventSource.OPEN;
    this.dispatchEvent(new Event('open'));
  }

  send(e: ChangeEvent): void {
    this.dispatchEvent(new MessageEvent('change', { data: JSON.stringify(e) }));
  }
}

/** An exclusive Web Lock manager for one lock name, as tabs share it. */
class FakeLocks {
  private queue: (() => Promise<void>)[] = [];
  private held = false;

  request(_name: string, opts: { signal?: AbortSignal }, cb: () => unknown): Promise<unknown> {
    return new Promise((resolve, reject) => {
      const run = async () => {
        this.held = true;
        try {
          resolve(await cb());
        } finally {
          this.held = false;
          this.next();
        }
      };
      opts.signal?.addEventListener('abort', () => {
        const i = this.queue.indexOf(run);
        if (i < 0) return;
        this.queue.splice(i, 1);
        reject(new DOMException('aborted', 'AbortError'));
      });
      this.queue.push(run);
      if (!this.held) this.next();
    });
  }

  private next(): void {
    void this.queue.shift()?.();
  }
}

function stream(i: number): FakeEventSource {
  const s = FakeEventSource.opened[i];
  if (!s) throw new Error(`no stream ${i}`);
  return s;
}

const event: ChangeEvent = { key: 'SOL-1', what: 'issue', by: 1 };

// BroadcastChannel delivers on a later turn of the event loop.
const settle = () => new Promise((r) => setTimeout(r, 20));

function tab(): { live: Live; got: ChangeEvent[] } {
  const live = new Live();
  const got: ChangeEvent[] = [];
  live.onChange((e) => got.push(e));
  live.start();
  return { live, got };
}

describe('Live', () => {
  const tabs: Live[] = [];
  const open = () => {
    const t = tab();
    tabs.push(t.live);
    return t;
  };

  beforeEach(() => {
    FakeEventSource.opened = [];
    vi.stubGlobal('EventSource', FakeEventSource);
    vi.stubGlobal('navigator', { locks: new FakeLocks() });
  });

  afterEach(() => {
    for (const t of tabs.splice(0)) t.stop();
    vi.unstubAllGlobals();
  });

  it('opens one stream for all tabs and relays its events', async () => {
    const [a, b, c] = [open(), open(), open()];
    await settle();
    expect(FakeEventSource.opened).toHaveLength(1);

    stream(0).connect();
    stream(0).send(event);
    await settle();
    expect([a.got, b.got, c.got]).toEqual([[event], [event], [event]]);
    expect([a.live.connected, b.live.connected, c.live.connected]).toEqual([true, true, true]);
  });

  it('tells a tab opened later that the stream is connected', async () => {
    open();
    await settle();
    stream(0).connect();
    const late = open();
    await settle();
    expect(late.live.connected).toBe(true);
  });

  it('hands the stream to another tab when the leader stops', async () => {
    const a = open();
    const b = open();
    await settle();
    stream(0).connect();
    await settle();

    a.live.stop();
    await settle();
    expect(stream(0).readyState).toBe(FakeEventSource.CLOSED);
    expect(FakeEventSource.opened).toHaveLength(2);
    expect(b.live.connected).toBe(false);

    stream(1).connect();
    stream(1).send(event);
    await settle();
    expect(b.got).toEqual([event]);
    expect(a.got).toEqual([]);
  });

  it('opens a stream per tab without Web Locks', async () => {
    vi.stubGlobal('navigator', {});
    open();
    open();
    await settle();
    expect(FakeEventSource.opened).toHaveLength(2);
  });
});
