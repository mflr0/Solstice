<script lang="ts">
  import { untrack } from 'svelte';
  import type { Attachment } from '../core/types.js';
  import { renderWiki, wikiReferences } from '../core/wiki.js';
  import { refs } from '../lib/refs.svelte.js';

  /**
   * Jira wiki markup rendered to HTML. Mentions and issue keys are resolved
   * through the shared reference cache, so they turn into names and links
   * as soon as the server has answered.
   */
  interface Props {
    source: string;
    attachments?: Attachment[] | undefined;
  }
  const { source, attachments = [] }: Props = $props();

  const files = $derived(new Map(attachments.map((a) => [a.filename, a.url])));
  const resolver = $derived(refs.resolver(files));

  $effect(() => {
    const wanted = wikiReferences(source);
    untrack(() => refs.ensure(wanted));
  });

  const html = $derived(renderWiki(source, resolver));
</script>

<!-- eslint-disable-next-line svelte/no-at-html-tags -- renderWiki escapes all text and only emits its own tags -->
<div class="wiki">{@html html}</div>
