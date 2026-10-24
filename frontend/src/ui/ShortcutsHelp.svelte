<script lang="ts">
  import { paths } from '../core/route.js';
  import { formatShortcut, isMac, SHORTCUTS, type ShortcutScope } from '../core/shortcuts.js';
  import { t } from '../i18n/t.js';
  import { appearance, dialogs } from '../lib/ui.svelte.js';
  import Modal from './Modal.svelte';

  /** Every keyboard shortcut, by where it works. Opened with "?". */

  const mac = isMac(typeof navigator !== 'undefined' ? navigator.platform : '');
  const scopes: ShortcutScope[] = ['global', 'results', 'issue'];
</script>

<Modal
  open={dialogs.shortcuts}
  title={t.keyboard.title}
  onclose={() => (dialogs.shortcuts = false)}
>
  {#if !appearance.shortcuts}
    <p class="form-hint off">
      {t.keyboard.off}
      <a href={paths.account('appearance')} onclick={() => (dialogs.shortcuts = false)}
        >{t.keyboard.settings}</a
      >
    </p>
  {/if}
  {#each scopes as scope (scope)}
    <section class="group" aria-labelledby={`keys-${scope}`}>
      <h3 class="field-label" id={`keys-${scope}`}>{t.keyboard.scopes[scope]}</h3>
      <dl class="keys">
        {#each SHORTCUTS.filter((s) => s.scope === scope) as s (s.action)}
          <dt>
            {#each s.keys as key, i (key)}
              {#if i > 0}<span class="muted">{t.keyboard.or}</span>{/if}
              <kbd>{formatShortcut(key, mac, t.editor.keys)}</kbd>
            {/each}
          </dt>
          <dd>{t.keyboard.actions[s.action]}</dd>
        {/each}
      </dl>
    </section>
  {/each}
</Modal>

<style>
  .off {
    margin-top: 0;
  }

  .group + .group {
    margin-top: 16px;
  }

  .keys {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 6px 16px;
    margin: 6px 0 0;
  }

  dt {
    display: flex;
    gap: 6px;
    align-items: baseline;
    white-space: nowrap;
  }

  dd {
    margin: 0;
  }
</style>
