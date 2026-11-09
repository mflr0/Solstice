<script lang="ts">
  import { issueLinkKey, modalIssue, paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { closeIssue, isPlainClick, router, showIssue } from '../../lib/router.svelte.js';
  import { isTyping, notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';
  import IssuePage from './IssuePage.svelte';

  /** The issue of the address's `?issue=KEY`, over whatever page is open. */
  const key = $derived(modalIssue(router.search));

  // The issue this modal is leaving: the view stays until the address changes,
  // and its own delete also comes back as a live event.
  let leaving: string | null = null;
  $effect.pre(() => {
    if (key !== leaving) leaving = null;
  });

  function leave() {
    leaving = key;
    closeIssue();
  }

  /** A plain click on a link to another issue shows it here. */
  function onclick(e: MouseEvent) {
    if (e.defaultPrevented || !isPlainClick(e)) return;
    const a = (e.target as Element | null)?.closest?.('a');
    if (!a || !a.href || a.target === '_blank' || a.hasAttribute('download')) return;
    const to = issueLinkKey(new URL(a.href), location.origin);
    if (!to) return;
    e.preventDefault();
    if (to !== key) showIssue(to);
  }

  // Escape in a field leaves the field (a draft, a menu), not the issue.
  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && isTyping(e.target)) e.preventDefault();
  }
</script>

<Modal open={!!key} title={key ?? ''} onclose={closeIssue} class="issue-modal">
  {#snippet heading()}
    <a class="key" href={paths.issue(key ?? '')}>{key}</a>
  {/snippet}
  {#snippet actions()}
    <button
      type="button"
      class="btn btn-small"
      onclick={() => key && router.navigate(paths.issue(key))}>{t.issue.openFullPage}</button
    >
  {/snippet}
  {#if key}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="issue-modal-view" {onclick} {onkeydown}>
      <IssuePage
        issueKey={key}
        ownsTitle={false}
        onkeychange={(k) => showIssue(k, { replace: true })}
        ongone={leave}
        onopen={(k) => showIssue(k)}
        onmissing={(message) => {
          if (leaving === key) return;
          notify(message);
          leave();
        }}
      />
    </div>
  {/if}
</Modal>
