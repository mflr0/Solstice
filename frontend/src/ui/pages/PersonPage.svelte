<script lang="ts">
  import { searchHref } from '../../core/filters-url.js';
  import { quote } from '../../core/jql.js';
  import type { Person } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import IssueTable from '../IssueTable.svelte';
  import Picture from '../Picture.svelte';

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

<div class="page" data-title={person?.user.name}>
  {#if error}
    <p class="error-text" role="alert">{error}</p>
  {:else if !person}
    <p class="muted loading">{t.common.loading}</p>
  {:else}
    <header class="page-head">
      <div class="id">
        <span class="mark avatar avatar-large" aria-hidden="true"
          ><Picture user={person.user} /></span
        >
        <div>
          <h1 class="page-title">{person.user.name}</h1>
          <p class="meta person-meta">
            <span class="key">{person.user.username}</span>
            {#if person.email}<a href={`mailto:${person.email}`}>{person.email}</a>{/if}
            {#if !person.user.active}<span class="stamp">{t.person.inactive}</span>{/if}
          </p>
        </div>
      </div>
      <a class="btn" href={searchHref(`participant = ${quote(person.user.username)}`)}
        >{t.search.allTickets}</a
      >
    </header>
    <h2 class="section-title">
      {t.person.assigned} <span class="count">{person.assigned.length}</span>
      <span class="actions"
        ><a
          class="link-btn"
          href={searchHref(`assignee = ${quote(person.user.username)} AND statusCategory != Done`)}
          >{t.common.seeAll}</a
        ></span
      >
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
    border: 1.5px solid var(--ink);
    font: 500 18px/1 var(--font-mono);
    background: var(--paper-raised);
  }

  /* Its size outweighs a skin's small-avatar rules (0-4-0); its look does not. */
  .id .mark.avatar-large {
    width: 54px;
    height: 54px;
    font-size: 18px;
  }

  .meta {
    display: flex;
    gap: 14px;
    align-items: center;
    margin: 6px 0 0;
    color: var(--ink-soft);
  }
</style>
