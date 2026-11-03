<script lang="ts">
  import { t } from '../../i18n/t.js';
  import { errorMessage } from '../../lib/api.js';
  import { appearance, applyShortcuts, dialogs, notify } from '../../lib/ui.svelte.js';

  /** The single-key shortcuts (on or off, kept on the account) and their list. */

  let saving = $state(false);

  async function toggle(input: HTMLInputElement) {
    saving = true;
    try {
      await applyShortcuts(input.checked);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      saving = false;
      // The switch shows what the account holds, also after a failed save.
      input.checked = appearance.shortcuts;
    }
  }
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
      disabled={saving}
      onchange={(e) => toggle(e.currentTarget)}
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
