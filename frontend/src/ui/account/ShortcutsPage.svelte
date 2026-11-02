<script lang="ts">
  import { t } from '../../i18n/t.js';
  import { appearance, applyShortcuts, dialogs } from '../../lib/ui.svelte.js';

  /** The single-key shortcuts (on or off, kept on this browser) and their list. */
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.account.title}</p>
    <h1 class="page-title">{t.account.sections.shortcuts}</h1>
  </div>
</header>

<section class="settings-form" aria-label={t.account.sections.shortcuts}>
  <label class="check">
    <input
      type="checkbox"
      checked={appearance.shortcuts}
      onchange={(e) => applyShortcuts(e.currentTarget.checked)}
    />
    <span>{t.keyboard.toggle}</span>
  </label>
  <p class="form-hint">{t.keyboard.toggleHint}</p>
  <p>
    <button type="button" class="btn" onclick={() => (dialogs.shortcuts = true)}
      >{t.keyboard.list}</button
    >
  </p>
</section>
