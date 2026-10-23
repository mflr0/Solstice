<script lang="ts">
  import { ago } from '../core/dates.js';
  import { longDay } from '../i18n/format.js';
  import { formatDateTime, renderAgo, t } from '../i18n/t.js';
  import { clock } from '../lib/ui.svelte.js';

  /**
   * An instant said relatively ("3 hours ago"); the exact date on hover.
   * Inside a sentence, an older instant takes the language's date preposition
   * ("on 12 March").
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
</script>

<time datetime={iso} title={formatDateTime(iso)}>{text}</time>
