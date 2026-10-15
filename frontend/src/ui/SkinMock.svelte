<script lang="ts">
  import type { Scheme, Skin } from '../core/appearance.js';
  import { t } from '../i18n/fr.js';
  import { setPreviewSkin } from '../lib/ui.svelte.js';
  import Logo from './Logo.svelte';
  import TypeMark from './TypeMark.svelte';

  /**
   * A miniature of the interface in a given skin and scheme, built from the
   * same hooks as the real pages, so the skin's own stylesheet draws it.
   * Decorative: hidden from assistive technology and inert.
   */
  interface Props {
    skin: Skin;
    scheme: Scheme;
  }
  const { skin, scheme }: Props = $props();
  setPreviewSkin(() => skin);

  const m = t.themes.mock;
  const first = m.rows[0];
</script>

<div class="skin-preview" data-skin={skin} data-scheme={scheme} aria-hidden="true" inert>
  <div class="mock app-shell">
    <header class="topbar">
      <div class="topbar-inner">
        <span class="brand"><Logo /><span class="wordmark">{t.appName}</span></span>
        <span class="topbar-nav">
          <span class="nav-link" aria-current="page">{m.work}</span>
          <span class="nav-link">{m.projects}</span>
          <span class="nav-link">{m.search}</span>
        </span>
        <span class="topbar-search"><span class="input">{m.searchPlaceholder}</span></span>
        <span class="live-dot on"></span>
        <span class="btn btn-primary topbar-create">{m.create}</span>
        <span class="account"
          ><span class="account-button"><span class="avatar">MD</span></span></span
        >
      </div>
    </header>

    <main class="app-main">
      <div class="page" style:padding-bottom="24px">
        <header class="page-head">
          <div>
            <p class="eyebrow">{m.eyebrow}</p>
            <h1 class="page-title">{m.greeting}</h1>
          </div>
        </header>

        <div class="split">
          <section>
            <h2 class="section-title">{m.section} <span class="count">{m.rows.length}</span></h2>
            <table class="ledger">
              <tbody>
                {#each m.rows as row (row.key)}
                  <tr>
                    <td><TypeMark type={{ id: row.key, name: row.type, subtask: false }} /></td>
                    <td><span class="key">{row.key}</span></td>
                    <td class="wide">{row.summary}</td>
                    <td>
                      <span class="stamp {row.category}" data-category={row.category}
                        >{row.status}</span
                      >
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
            <div class="row form">
              <span class="input grow">{m.comment}</span>
              <span class="btn">{m.cancel}</span>
              <span class="btn btn-primary">{m.save}</span>
            </div>
          </section>

          <aside class="card">
            <div class="card-head">{m.fiche} <span class="key">{first?.key}</span></div>
            <dl>
              <div class="line">
                <dt>{m.assignee}</dt>
                <dd><span class="avatar">MD</span> Marie Durand</dd>
              </div>
              <div class="line">
                <dt>{m.due}</dt>
                <dd>{m.dueValue}</dd>
              </div>
              <div class="line">
                <dd>
                  <span class="stamp large doing" data-category="doing">{first?.status}</span>
                </dd>
              </div>
            </dl>
          </aside>
        </div>

        <div class="board">
          <section class="board-column">
            <header class="board-column-head">
              <span class="board-column-title">{m.todo}</span>
              <span class="board-column-count">1</span>
            </header>
            <div class="board-cards">
              <article class="board-card">
                <span class="key">DEMO-15</span>
                <span>{m.cardSummary}</span>
              </article>
            </div>
          </section>
          <section class="board-column over-limit">
            <header class="board-column-head">
              <span class="board-column-title">{m.doing}</span>
              <span class="board-column-count">2</span>
              <span class="wip-limit">{t.boards.wip(null, 1)}</span>
            </header>
            <div class="board-cards">
              <article class="board-card">
                <span class="key">{first?.key}</span>
                <span>{first?.summary}</span>
              </article>
            </div>
          </section>
        </div>
      </div>
    </main>

    <footer class="app-statusbar">
      <span class="statusbar-cell">{m.ready}</span>
      <span class="statusbar-cell">{m.online}</span>
      <span class="statusbar-cell">Marie Durand</span>
    </footer>
  </div>
</div>

<style>
  /* Real sizes, shrunk: the skin's stylesheet sees an ordinary page. */
  .mock {
    width: 1000px;
    zoom: 0.44;
  }

  .split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 260px;
    gap: 28px;
    align-items: start;
  }

  .split .section-title {
    margin-top: 0;
  }

  .wide {
    width: 100%;
  }

  .form {
    margin-top: 14px;
    flex-wrap: nowrap;
  }

  .form .input {
    display: block;
    color: var(--muted);
  }

  .topbar-search .input {
    display: flex;
    align-items: center;
    color: var(--muted);
  }

  dl {
    margin: 0;
  }

  .line {
    display: flex;
    gap: 10px;
    align-items: center;
    padding: 8px 14px;
  }

  dt {
    width: 80px;
    font-size: 12px;
    color: var(--muted);
  }

  dd {
    display: flex;
    gap: 8px;
    align-items: center;
    margin: 0;
  }

  dd .avatar {
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
    border: var(--frame) solid var(--ink-soft);
    border-radius: var(--avatar-radius);
    font: 500 10px/1 var(--font-mono);
  }

  .board {
    margin-top: 22px;
    grid-auto-columns: 1fr;
    overflow: visible;
  }

  .board-column {
    min-height: 0;
  }
</style>
