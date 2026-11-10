<script lang="ts">
  import { tick } from 'svelte';
  import { formatShortcut } from '../core/shortcuts.js';
  import { startsTour, tourSteps, type TourKey } from '../core/tour.js';
  import { t } from '../i18n/t.js';
  import { errorMessage } from '../lib/api.js';
  import { session } from '../lib/session.svelte.js';
  import { appearance, dialogs, mac, markTourSeen, notify } from '../lib/ui.svelte.js';

  /**
   * The guided tour of the top bar: the page dimmed but for one element, and a
   * card next to it. Starts by itself until the account has seen it.
   */

  /** Room between the element and the spotlight's edge, and around the card. */
  const PAD = 4;
  const MARGIN = 12;

  const steps = $derived(
    tourSteps({
      admin: !!session.me?.admin,
      shortcuts: appearance.shortcuts,
      bindings: appearance.bindings,
      mac,
    }),
  );
  let index = $state(0);
  const step = $derived(steps[Math.min(index, steps.length - 1)]!);
  const text = $derived(t.tour.steps[step.id]);
  const last = $derived(index >= steps.length - 1);

  let dialog: HTMLDialogElement | undefined = $state();
  let card: HTMLElement | undefined = $state();
  let nextButton: HTMLButtonElement | undefined = $state();
  let hole = $state<{ top: number; left: number; width: number; height: number } | null>(null);
  let at = $state<{ top: number; left: number } | null>(null);
  let announced = $state('');
  let opener: HTMLElement | null = null;
  const uid = $props.id();

  // Once per sign-in: this component lives as long as the app shell.
  let checked = false;
  $effect(() => {
    if (checked || !session.me) return;
    checked = true;
    if (startsTour(session.me.preferences)) dialogs.tour = true;
  });

  $effect(() => {
    if (!dialog) return;
    if (dialogs.tour && !dialog.open) {
      opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      index = 0;
      dialog.showModal();
      void show().then(() => nextButton?.focus());
    } else if (!dialogs.tour && dialog.open) dialog.close();
  });

  $effect(() => {
    if (!dialogs.tour) return;
    let frame = 0;
    const follow = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => void place(false));
    };
    window.addEventListener('resize', follow);
    window.addEventListener('scroll', follow, true);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener('resize', follow);
      window.removeEventListener('scroll', follow, true);
    };
  });

  /** The step's element, when it is drawn; null for a centered card. */
  function target(): HTMLElement | null {
    if (!step.target) return null;
    const el = document.querySelector<HTMLElement>(`[data-tour="${step.target}"]`);
    if (!el || !el.getClientRects().length || getComputedStyle(el).visibility === 'hidden')
      return null;
    return el;
  }

  async function place(scroll: boolean): Promise<void> {
    await tick();
    const el = target();
    if (el && scroll) el.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    const r = el?.getBoundingClientRect();
    const seen =
      r && r.width > 0 && r.height > 0 && r.right > 0 && r.bottom > 0
        ? r.left < innerWidth && r.top < innerHeight
        : false;
    hole =
      r && seen
        ? {
            top: r.top - PAD,
            left: r.left - PAD,
            width: r.width + 2 * PAD,
            height: r.height + 2 * PAD,
          }
        : null;
    if (!hole || !card) {
      at = null;
      return;
    }
    await tick();
    const w = card.offsetWidth;
    const h = card.offsetHeight;
    const below = hole.top + hole.height + MARGIN;
    const above = hole.top - MARGIN - h;
    const top =
      below + h <= innerHeight - MARGIN
        ? below
        : above >= MARGIN
          ? above
          : innerHeight - MARGIN - h;
    const left = hole.left + hole.width / 2 - w / 2;
    at = {
      top: Math.max(MARGIN, top),
      left: Math.max(MARGIN, Math.min(left, innerWidth - MARGIN - w)),
    };
  }

  async function show(): Promise<void> {
    await place(true);
    announced = t.tour.announce(index + 1, steps.length, text.title);
    const focused = document.activeElement;
    if (!(focused instanceof HTMLButtonElement) || focused.disabled || !card?.contains(focused))
      nextButton?.focus();
  }

  function go(delta: number): void {
    const to = index + delta;
    if (to < 0 || to >= steps.length) return;
    index = to;
    void show();
  }

  function finish(): void {
    if (!dialogs.tour) return;
    dialogs.tour = false;
    dialog?.close();
    const back = opener?.isConnected
      ? opener
      : document.querySelector<HTMLElement>('[data-tour="account"]');
    back?.focus();
    markTourSeen().catch((e: unknown) => notify(errorMessage(e), 'error'));
  }

  function onkeydown(e: KeyboardEvent): void {
    if (e.key === 'Tab') {
      // A modal dialog lets Tab out to the browser's own controls: keep it in the card.
      const buttons = [...(card?.querySelectorAll<HTMLButtonElement>('button:enabled') ?? [])];
      const edge = e.shiftKey ? buttons[0] : buttons.at(-1);
      if (edge && document.activeElement === edge) {
        e.preventDefault();
        (e.shiftKey ? buttons.at(-1) : buttons[0])?.focus();
      }
    } else if (e.key === 'ArrowRight') {
      e.preventDefault();
      go(1);
    } else if (e.key === 'ArrowLeft') {
      e.preventDefault();
      go(-1);
    }
  }

  // Escape: end the tour the same way Skip does.
  function oncancel(e: Event): void {
    e.preventDefault();
    finish();
  }

  const keyText = (k: TourKey) => {
    const key = formatShortcut(k.binding, mac, t.editor.keys);
    return k.action === 'help' ? t.tour.allShortcuts(key) : t.tour.shortcut(key);
  };
</script>

<dialog
  bind:this={dialog}
  class="tour"
  class:centered={!hole}
  aria-label={t.tour.label}
  aria-describedby={`tour-body-${uid}`}
  {oncancel}
  onclose={finish}
  {onkeydown}
>
  {#if dialogs.tour}
    {#if hole}
      <div
        class="tour-hole"
        aria-hidden="true"
        style:top="{hole.top}px"
        style:left="{hole.left}px"
        style:width="{hole.width}px"
        style:height="{hole.height}px"
      ></div>
    {/if}
    <div
      class="tour-card"
      class:placed={!!at}
      bind:this={card}
      style:top={at ? `${at.top}px` : undefined}
      style:left={at ? `${at.left}px` : undefined}
    >
      <p class="tour-position" aria-hidden="true">{t.tour.position(index + 1, steps.length)}</p>
      <h2 class="tour-title">{text.title}</h2>
      <div id={`tour-body-${uid}`}>
        <p>{text.body}</p>
        {#each step.keys as k (k.action)}<p>{keyText(k)}</p>{/each}
      </div>
      <div class="tour-actions">
        <button type="button" class="btn btn-quiet" onclick={finish}>{t.tour.skip}</button>
        <button type="button" class="btn" disabled={index === 0} onclick={() => go(-1)}
          >{t.tour.back}</button
        >
        <button
          type="button"
          class="btn btn-primary"
          bind:this={nextButton}
          onclick={() => (last ? finish() : go(1))}>{last ? t.tour.done : t.tour.next}</button
        >
      </div>
    </div>
    <p class="sr-only" aria-live="polite">{announced}</p>
  {/if}
</dialog>

<style>
  .tour {
    position: fixed;
    inset: 0;
    width: 100%;
    height: 100%;
    max-width: none;
    max-height: none;
    margin: 0;
    padding: 0;
    border: 0;
    overflow: hidden;
    background: transparent;
    color: var(--ink);
  }

  .tour::backdrop {
    background: transparent;
  }

  .tour.centered {
    background: var(--backdrop);
  }

  .tour-hole {
    position: fixed;
    border-radius: var(--radius-m);
    box-shadow: 0 0 0 200vmax var(--backdrop);
    outline: 2px solid var(--accent);
    outline-offset: 2px;
    pointer-events: none;
  }

  .tour-card {
    position: fixed;
    top: 50%;
    left: 50%;
    width: min(22rem, calc(100vw - 24px));
    max-height: calc(100vh - 24px);
    overflow: auto;
    padding: 14px 16px;
    border: var(--frame) solid var(--rule-strong);
    border-radius: var(--radius-l);
    background: var(--paper-raised);
    box-shadow: var(--shadow-pop);
    transform: translate(-50%, -50%);
  }

  .tour-card.placed {
    transform: none;
  }

  .tour-card p {
    margin: 0 0 8px;
  }

  .tour-position {
    font: 500 0.75rem/1.2 var(--font-mono);
    color: var(--muted);
  }

  .tour-title {
    margin: 0 0 6px;
    font: var(--title-weight) 1.0625rem/1.25 var(--font-title);
  }

  .tour-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    justify-content: flex-end;
    margin-top: 12px;
  }

  .tour-actions .btn-quiet {
    margin-right: auto;
  }

  @media (prefers-reduced-motion: no-preference) {
    .tour-hole,
    .tour-card.placed {
      transition:
        top 0.15s ease,
        left 0.15s ease,
        width 0.15s ease,
        height 0.15s ease;
    }
  }
</style>
