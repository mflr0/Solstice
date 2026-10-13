<script lang="ts">
  import { initials } from '../../core/marks.js';
  import type { Person } from '../../core/types.js';
  import { t } from '../../i18n/fr.js';
  import { api, errorMessage } from '../../lib/api.js';
  import IssueTable from '../IssueTable.svelte';

  /** Someone's page: who they are and what is open in their name. */
  interface Props {
    username: string;
  }
  const { username }: Props = $props();
  let person = $state<Person | null>(null);
  let error = $state('');

  $effect(() => {
    const u = username;
    person = null;
    error = '';
    api
      .person(u)
      .then((p) => {
        if (u !== username) return;
        person = p;
        document.title = `${p.user.name} — ${t.appName}`;
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  });
</script>

<div class="page">
  {#if error}
    <p class="error-text" role="alert">{error}</p>
  {:else if !person}
    <p class="muted">{t.common.loading}</p>
  {:else}
    <header class="page-head">
      <div class="id">
        <span class="mark" aria-hidden="true">{initials(person.user.name)}</span>
        <div>
          <h1>{person.user.name}</h1>
          <p class="meta">
            <span class="key">{person.user.username}</span>
            {#if person.email}<a href={`mailto:${person.email}`}>{person.email}</a>{/if}
            {#if !person.user.active}<span class="stamp">{t.person.inactive}</span>{/if}
          </p>
        </div>
      </div>
    </header>
    <h2 class="section-title">
      {t.person.assigned} <span class="count">{person.assigned.length}</span>
    </h2>
    {#if person.assigned.length}
      <IssueTable issues={person.assigned} showAssignee={false} />
    {:else}
      <p class="empty">{t.person.empty}</p>
    {/if}
  {/if}
</div>

<style>
  .id {
    display: flex;
    gap: 16px;
    align-items: center;
  }

  .mark {
    display: grid;
    place-items: center;
    width: 54px;
    height: 54px;
    border: 1.5px solid var(--ink);
    font: 500 18px/1 var(--mono);
    background: var(--paper-raised);
  }

  .meta {
    display: flex;
    gap: 14px;
    align-items: center;
    margin: 6px 0 0;
    color: var(--ink-soft);
  }
</style>
