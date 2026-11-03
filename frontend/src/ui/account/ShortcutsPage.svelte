<script lang="ts">
  import { tick } from 'svelte';
  import {
    bindingsInUse,
    clashes,
    formatShortcut,
    MAX_BINDINGS,
    overridesFrom,
    SHORTCUTS,
    shortcutParts,
    type Shortcut,
    type ShortcutAction,
    type ShortcutScope,
  } from '../../core/shortcuts.js';
  import { t } from '../../i18n/t.js';
  import { errorMessage } from '../../lib/api.js';
  import {
    appearance,
    applyBindings,
    applyShortcuts,
    dialogs,
    mac,
    notify,
  } from '../../lib/ui.svelte.js';
  import KeyCaps from '../KeyCaps.svelte';
  import ShortcutRecorder from './ShortcutRecorder.svelte';

  /** The single-key shortcuts (on or off, kept on the account), their list, and their keys. */

  type Keys = Record<ShortcutAction, string[]>;

  const scopes: ShortcutScope[] = ['global', 'results', 'issue'];

  function keysOf(shortcuts: readonly Shortcut[]): Keys {
    return Object.fromEntries(shortcuts.map((s) => [s.action, [...s.keys]])) as Keys;
  }

  let saving = $state(false);
  const saved = $derived(overridesFrom(keysOf(appearance.bindings)));
  let draft = $state(keysOf(appearance.bindings));
  const edited = $derived(bindingsInUse(draft));
  const found = $derived(clashes(edited, mac));
  const changes = $derived(overridesFrom(draft));
  const dirty = $derived(JSON.stringify(changes) !== JSON.stringify(saved));

  function shown(binding: string): string {
    return formatShortcut(binding, mac, t.editor.keys);
  }

  /** The actions a binding of an action clashes with. */
  function clashing(action: ShortcutAction, binding: string): ShortcutAction[] {
    return found.flatMap(([a, b]) => {
      if (a.action === action && a.binding === binding) return [b.action];
      if (b.action === action && b.binding === binding) return [a.action];
      return [];
    });
  }

  function isDefault(action: ShortcutAction): boolean {
    return !Object.hasOwn(overridesFrom({ [action]: draft[action] }), action);
  }

  function add(action: ShortcutAction, binding: string): boolean {
    if (draft[action].includes(binding)) return false;
    draft[action].push(binding);
    return true;
  }

  async function remove(action: ShortcutAction, i: number) {
    draft[action].splice(i, 1);
    // The removed key's button is gone: keep the focus in the row.
    await tick();
    document.getElementById(`add-key-${action}`)?.focus();
  }

  function reset(action: ShortcutAction) {
    draft[action] = [...(SHORTCUTS.find((s) => s.action === action)?.keys ?? [])];
  }

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

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (found.length || !dirty) return;
    saving = true;
    try {
      await applyBindings(changes);
      draft = keysOf(appearance.bindings);
      notify(t.keyboard.saved);
    } catch (err) {
      notify(errorMessage(err), 'error');
    } finally {
      saving = false;
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

<form class="settings-form bindings" aria-labelledby="bindings-title" onsubmit={save}>
  <h2 class="section-title" id="bindings-title">{t.keyboard.bindings}</h2>
  <p class="form-hint">{t.keyboard.bindingsHint}</p>
  {#each scopes as scope (scope)}
    <section class="group" aria-labelledby={`bind-${scope}`}>
      <h3 class="field-label" id={`bind-${scope}`}>{t.keyboard.scopes[scope]}</h3>
      <ul class="rows">
        {#each edited.filter((s) => s.scope === scope) as s (s.action)}
          {@const name = t.keyboard.actions[s.action]}
          <li class="row">
            <span class="action">{name}</span>
            <span class="keys">
              {#each s.keys as key, i (i)}
                {@const others = clashing(s.action, key)}
                <span class="key" class:clash={others.length > 0}>
                  <KeyCaps parts={shortcutParts(key, mac, t.editor.keys)} label={shown(key)} />
                  <button
                    type="button"
                    class="btn btn-quiet btn-small"
                    aria-label={t.keyboard.removeKey(shown(key))}
                    disabled={saving}
                    onclick={() => remove(s.action, i)}>×</button
                  >
                  {#each others as other (other)}
                    <span class="clash-text error-text"
                      >{t.keyboard.clashWith(t.keyboard.actions[other])}</span
                    >
                  {/each}
                </span>
              {:else}
                <span class="muted">{t.keyboard.noKey}</span>
              {/each}
            </span>
            <span class="tools">
              {#if !isDefault(s.action)}
                <button
                  type="button"
                  class="btn btn-quiet btn-small"
                  aria-label={t.keyboard.resetAction(name)}
                  disabled={saving}
                  onclick={() => reset(s.action)}>{t.keyboard.reset}</button
                >
              {/if}
              <ShortcutRecorder
                id={`add-key-${s.action}`}
                action={name}
                disabled={saving || s.keys.length >= MAX_BINDINGS}
                onrecord={(b) => add(s.action, b)}
              />
            </span>
          </li>
        {/each}
      </ul>
    </section>
  {/each}
  <p class="error-text blocked" role="status">{found.length ? t.keyboard.clashBlocked : ''}</p>
  <p class="form-actions">
    <button type="submit" class="btn btn-primary" disabled={saving || !dirty || found.length > 0}
      >{t.common.save}</button
    >
    <button
      type="button"
      class="btn"
      disabled={saving || !Object.keys(changes).length}
      onclick={() => (draft = keysOf(SHORTCUTS))}>{t.keyboard.resetAll}</button
    >
  </p>
</form>

<style>
  .bindings {
    margin-top: 28px;
  }

  .group + .group {
    margin-top: 16px;
  }

  .rows {
    margin: 6px 0 0;
    padding: 0;
    list-style: none;
  }

  .row {
    display: grid;
    grid-template-columns: minmax(8rem, 14rem) minmax(0, 1fr) auto;
    gap: 6px 16px;
    align-items: start;
    padding: 8px 0;
    border-top: 1px solid var(--rule);
  }

  .action,
  .keys {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .keys {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
  }

  .key {
    display: inline-flex;
    flex-wrap: wrap;
    gap: 2px 2px;
    align-items: center;
    padding: 3px 0 3px 4px;
    border: 2px solid transparent;
    border-radius: var(--radius-s);
    background: var(--paper-sunk);
    max-width: 100%;
  }

  .key.clash {
    border-color: var(--accent);
  }

  .clash-text {
    flex-basis: 100%;
    padding-inline-end: 6px;
    font-family: var(--font-body);
    font-size: 0.75rem;
  }

  .tools {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 8px;
    align-items: center;
    justify-content: end;
    min-width: 0;
  }

  .blocked:empty {
    margin: 0;
  }

  .form-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  @media (max-width: 40rem) {
    .row {
      grid-template-columns: minmax(0, 1fr);
    }

    .tools {
      justify-content: start;
    }
  }
</style>
