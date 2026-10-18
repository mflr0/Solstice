<script lang="ts">
  import type { ComponentRow } from '../../core/admin-types.js';
  import {
    ASSIGNEE_TYPES,
    componentForm,
    componentPayload,
    uniqueNameProblem,
    type ComponentForm,
  } from '../../core/project-admin.js';
  import type { UserRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';
  import UserPicker from '../UserPicker.svelte';

  /** Creating a component, or changing one: name, description, lead and who its issues go to. */
  interface Props {
    projectKey: string;
    open: boolean;
    /** The component to change; null to create one. */
    component: ComponentRow | null;
    /** Every component of the project, to spot a name already taken. */
    components: ComponentRow[];
    onclose: () => void;
    onsaved: (component: ComponentRow, created: boolean) => void;
  }
  const { projectKey, open, component, components, onclose, onsaved }: Props = $props();

  let form = $state<ComponentForm>(componentForm(null));
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);

  const problem = $derived(uniqueNameProblem(form.name, components, component?.id ?? null));
  const shown = $derived((tried || touched) && problem ? t.projectConfig.problems[problem] : '');
  const formId = 'component-form';

  $effect(() => {
    if (!open) return;
    form = componentForm(component);
    tried = false;
    touched = false;
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (problem) return;
    const payload = componentPayload(form, component);
    if (component && Object.keys(payload).length === 0) {
      onclose();
      return;
    }
    busy = true;
    try {
      const saved = component
        ? await projectAdminApi.updateComponent(projectKey, component.id, payload)
        : await projectAdminApi.createComponent(projectKey, payload);
      onsaved(saved, component === null);
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal
  {open}
  title={component
    ? t.projectConfig.components.editTitle(component.name)
    : t.projectConfig.components.newTitle}
  onclose={() => !busy && onclose()}
>
  <form id={formId} class="component-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.projects.name}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input"
        bind:value={form.name}
        placeholder={t.projectConfig.components.namePlaceholder}
        autocomplete="off"
        autofocus
        aria-invalid={!!shown}
        onblur={() => (touched = true)}
      />
      {#if shown}<span class="field-error">{shown}</span>{/if}
    </label>

    <label class="field">
      <span>{t.projectsAdmin.description}</span>
      <textarea class="textarea" bind:value={form.description} rows="2"></textarea>
    </label>

    <div class="field">
      <span>{t.projects.lead}</span>
      <UserPicker
        field
        value={form.lead}
        label={t.projects.lead}
        noneLabel={t.projectConfig.components.noLead}
        search={(q) => api.users(q)}
        onpick={(u: UserRef | null) => (form.lead = u)}
      />
    </div>

    <label class="field">
      <span>{t.projectConfig.components.assignee}</span>
      <select class="select" bind:value={form.assigneeType}>
        {#each ASSIGNEE_TYPES as a (a)}
          <option value={a}>{t.projectConfig.components.assignees[a]}</option>
        {/each}
      </select>
      <span class="hint">{t.projectConfig.components.assigneeHint}</span>
    </label>

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form={formId} disabled={busy}
      >{busy ? t.common.saving : component ? t.common.save : t.catalogAdmin.create}</button
    >
  {/snippet}
</Modal>

<style>
  .component-form {
    display: grid;
    gap: 14px;
  }

  .hint {
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
