<script lang="ts">
  import { t } from '../../i18n/t.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';

  /**
   * Creating or changing something that has a name and a description (a
   * group, a role): the form, its checks and the server's refusal in place.
   * The page does the saving and words the problems.
   */
  interface Props {
    open: boolean;
    title: string;
    /** What the fields start with ('' to create). */
    name: string;
    description: string;
    /** The check of a name: the message to show, or '' when it is fine. */
    problem: (name: string) => string;
    /** Under the name field, when there is no problem to show. */
    hint?: string | undefined;
    nameLabel: string;
    descriptionLabel: string;
    /** The button that saves ("Create the group"; "Save" when changing). */
    submit?: string | undefined;
    onsave: (input: { name: string; description: string }) => Promise<unknown>;
    onclose: () => void;
  }
  const {
    open,
    title,
    name: initialName,
    description: initialDescription,
    problem,
    hint,
    nameLabel,
    descriptionLabel,
    submit: submitLabel = t.common.save,
    onsave,
    onclose,
  }: Props = $props();

  let name = $state('');
  let description = $state('');
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);

  const shown = $derived(tried || touched ? problem(name) : '');

  $effect(() => {
    if (!open) return;
    name = initialName;
    description = initialDescription;
    tried = false;
    touched = false;
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (problem(name)) return;
    busy = true;
    try {
      await onsave({ name: name.trim(), description: description.trim() });
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal {open} {title} onclose={() => !busy && onclose()}>
  <form id="named-form" class="named-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{nameLabel}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input"
        bind:value={name}
        autocomplete="off"
        autofocus
        aria-invalid={!!shown}
        onblur={() => (touched = true)}
      />
      {#if shown}
        <span class="field-error">{shown}</span>
      {:else if hint}
        <span class="hint">{hint}</span>
      {/if}
    </label>
    <label class="field">
      <span>{descriptionLabel}</span>
      <textarea class="textarea short" bind:value={description} rows="2"></textarea>
    </label>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="named-form" disabled={busy}
      >{busy ? t.common.saving : submitLabel}</button
    >
  {/snippet}
</Modal>

<style>
  .named-form {
    display: grid;
    gap: 14px;
  }

  .short {
    min-height: 0;
    font: 14px/1.4 var(--font-body);
  }
</style>
