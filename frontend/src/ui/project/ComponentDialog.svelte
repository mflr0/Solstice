<script lang="ts">
  import type { ComponentRow } from '../../core/admin-types.js';
  import {
    ASSIGNEE_TYPES,
    componentPayload,
    componentProblem,
    nameProblem,
    type ComponentForm,
  } from '../../core/project-admin.js';
  import { t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';
  import UserPicker from '../UserPicker.svelte';

  /**
   * Creating a component, or changing one: name, description, lead and who
   * new issues with the component are assigned to.
   */
  interface Props {
    open: boolean;
    projectKey: string;
    /** The component to change; null to create one. */
    component: ComponentRow | null;
    /** Every component of the project, to spot a name already taken. */
    components: ComponentRow[];
    onclose: () => void;
    onsaved: (component: ComponentRow, created: boolean) => void;
  }
  const { open, projectKey, component, components, onclose, onsaved }: Props = $props();

  const blank = (): ComponentForm => ({
    name: '',
    description: '',
    lead: null,
    assigneeType: 'project_default',
  });

  let form = $state<ComponentForm>(blank());
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);

  const nameIssue = $derived(nameProblem(form.name, components, component?.id ?? null));
  const leadIssue = $derived(componentProblem(form));
  const nameShown = $derived(
    (tried || touched) && nameIssue ? t.projectComponents.problems[nameIssue] : '',
  );
  const leadShown = $derived(tried && leadIssue ? t.projectComponents.problems[leadIssue] : '');

  $effect(() => {
    if (!open) return;
    form = component
      ? {
          name: component.name,
          description: component.description ?? '',
          lead: component.lead,
          assigneeType: component.assigneeType,
        }
      : blank();
    tried = false;
    touched = false;
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (nameIssue || leadIssue) return;
    busy = true;
    try {
      const body = componentPayload(form);
      const saved = component
        ? await projectAdminApi.updateComponent(projectKey, component.id, body)
        : await projectAdminApi.createComponent(projectKey, body);
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
    ? t.projectComponents.editTitle(component.name)
    : t.projectComponents.createTitle}
  onclose={() => !busy && onclose()}
>
  <form id="component-form" class="component-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.projects.name}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input"
        bind:value={form.name}
        autocomplete="off"
        autofocus
        aria-invalid={!!nameShown}
        onblur={() => (touched = true)}
      />
      {#if nameShown}<span class="field-error">{nameShown}</span>{/if}
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
        noneLabel={t.common.none}
        search={(q) => api.users(q)}
        onpick={(u) => (form.lead = u)}
      />
    </div>
    <label class="field">
      <span>{t.projectComponents.assignee}</span>
      <select class="select" bind:value={form.assigneeType} aria-invalid={!!leadShown}>
        {#each ASSIGNEE_TYPES as a (a)}
          <option value={a}>{t.projectComponents.assigneeTypes[a]}</option>
        {/each}
      </select>
      {#if leadShown}<span class="field-error">{leadShown}</span>{/if}
    </label>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="component-form" disabled={busy}
      >{busy ? t.common.saving : component ? t.common.save : t.projectComponents.create}</button
    >
  {/snippet}
</Modal>

<style>
  .component-form {
    display: grid;
    gap: 14px;
  }

  .field-error {
    font-size: 12.5px;
    color: var(--accent-text);
  }

  .input[aria-invalid='true'],
  .select[aria-invalid='true'] {
    border-color: var(--accent);
  }
</style>
