<script lang="ts">
  import { ago } from '../core/dates.js';
  import { clockTime, longDay, timeZone, weekdayDate } from '../i18n/format.js';
  import { renderAgo, t } from '../i18n/t.js';
  import { clock } from '../lib/ui.svelte.js';

  /**
   * An instant said relatively ("3 hours ago"); the exact date in a popup on
   * hover, and to screen readers as hidden text. Inside a sentence, an older
   * instant takes the language's date preposition ("on 12 March").
   */
  interface Props {
    iso: string;
    inSentence?: boolean;
  }
  const { iso, inSentence = false }: Props = $props();
  const text = $derived.by(() => {
    const a = ago(iso, clock.now);
    return inSentence && a.kind === 'date' ? t.time.onDate(longDay(a.day)) : renderAgo(a);
  });
  const day = $derived(weekdayDate(iso));
  const time = $derived(clockTime(iso));
  const zone = timeZone();

  let anchor: HTMLElement | undefined = $state();
  let open = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  function enter(e: PointerEvent) {
    // A tap opens the row under it: no hover to show the popup on.
    if (e.pointerType === 'touch') return;
    clearTimeout(timer);
    timer = setTimeout(() => (open = true), 250);
  }

  function leave() {
    clearTimeout(timer);
    open = false;
  }

  $effect(() => () => clearTimeout(timer));

  /**
   * Shows the popup in the top layer, so no scroll box clips it: above the
   * date, its right edge on the date's, or under it when there is no room above.
   */
  function above(node: HTMLElement, target: HTMLElement) {
    node.showPopover?.();
    const place = () => {
      const r = target.getBoundingClientRect();
      const width = document.documentElement.clientWidth;
      const left = Math.min(r.right, width - 8) - node.offsetWidth;
      node.style.left = `${Math.max(8, left)}px`;
      const top = r.top - node.offsetHeight - 6;
      node.style.top = `${top >= 8 ? top : r.bottom + 6}px`;
    };
    place();
    addEventListener('scroll', place, { capture: true, passive: true });
    addEventListener('resize', place);
    return {
      destroy() {
        removeEventListener('scroll', place, { capture: true });
        removeEventListener('resize', place);
      },
    };
  }
</script>

<time
  class="reltime"
  datetime={iso}
  bind:this={anchor}
  onpointerenter={enter}
  onpointerleave={leave}>{text}<span class="sr-only">, {day}, {time} {zone}</span></time
>{#if open && anchor}<span
    class="reltime-tip"
    popover="manual"
    aria-hidden="true"
    use:above={anchor}
    ><span class="reltime-day">{day}</span><span class="reltime-time">{time} · {zone}</span></span
  >{/if}
