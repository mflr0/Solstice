<script lang="ts">
  import { ago } from '../core/dates.js';
  import { longDay } from '../i18n/format.js';
  import { formatDateTime, renderAgo, t } from '../i18n/fr.js';
  import { clock } from '../lib/ui.svelte.js';

  /**
   * An instant said relatively ("il y a 3 h"); the exact date on hover.
   * Inside a sentence, an older instant takes the language's date preposition
   * ("le 12 mars", "am 12. März").
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
