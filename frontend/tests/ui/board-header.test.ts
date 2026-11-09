// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import type { BoardDetail } from '../../src/core/board-types.js';
import { en } from '../../src/i18n/en.js';
import BoardHeader from '../../src/ui/board/BoardHeader.svelte';

function detail(kind: 'scrum' | 'kanban', kanbanBacklog: boolean, manage: boolean): BoardDetail {
  return {
    board: {
      id: 7,
      name: 'Team',
      kind,
      filterId: 1,
      filterName: 'Filter',
      query: '',
      subQuery: null,
      sectionBy: 'custom',
      estimate: '',
      estimateField: null,
      doneCutoff: null,
      kanbanBacklog,
    },
    projects: [],
    columns: [],
    quickFilters: [],
    sections: [],
    sprints: [],
    estimateFields: [],
    shares: [],
    can: { manage, share: manage },
  };
}

let mounted: Record<string, unknown> | undefined;

afterEach(() => {
  if (mounted) unmount(mounted);
  mounted = undefined;
  document.body.innerHTML = '';
});

function links(d: BoardDetail, view: 'board' | 'backlog' | 'reports' | 'settings') {
  mounted = mount(BoardHeader, { target: document.body, props: { detail: d, view } });
  flushSync();
  return [...document.querySelectorAll<HTMLAnchorElement>('nav a')];
}

const labels = {
  board: en.boards.viewBoard,
  backlog: en.boards.viewBacklog,
  reports: en.boards.viewReports,
  settings: en.boards.settings,
};

describe('BoardHeader', () => {
  it.each(['board', 'backlog', 'reports', 'settings'] as const)(
    'marks only %s as the current page',
    (view) => {
      const current = links(detail('scrum', false, true), view).filter(
        (a) => a.getAttribute('aria-current') === 'page',
      );
      expect(current.map((a) => a.getAttribute('aria-label'))).toEqual([labels[view]]);
      expect(current[0]?.classList.contains('on')).toBe(true);
    },
  );

  it('names every view without visible text', () => {
    for (const a of links(detail('scrum', false, true), 'board')) {
      expect(a.getAttribute('aria-label')).toBeTruthy();
      expect(a.title).toBe(a.getAttribute('aria-label'));
      expect(a.textContent?.trim()).toBe('');
      expect(a.querySelector('svg path')).not.toBeNull();
    }
  });

  it('offers sharing, outside the views, whoever looks at the board', () => {
    for (const manage of [true, false]) {
      links(detail('kanban', false, manage), 'board');
      const share = document.querySelector<HTMLButtonElement>('nav + button.btn-icon');
      expect(share?.getAttribute('aria-label')).toBe(en.boards.share);
      if (mounted) unmount(mounted);
      mounted = undefined;
    }
  });

  it('shows the backlog for scrum and for kanban with a backlog only', () => {
    const shown = (d: BoardDetail) => {
      const has = links(d, 'board').some((a) => a.getAttribute('aria-label') === labels.backlog);
      if (mounted) unmount(mounted);
      mounted = undefined;
      return has;
    };
    expect(shown(detail('scrum', false, false))).toBe(true);
    expect(shown(detail('kanban', true, false))).toBe(true);
    expect(shown(detail('kanban', false, false))).toBe(false);
  });

  it('offers settings only to those who manage the board', () => {
    const shown = (manage: boolean) => {
      const has = links(detail('kanban', false, manage), 'board').some(
        (a) => a.getAttribute('aria-label') === labels.settings,
      );
      if (mounted) unmount(mounted);
      mounted = undefined;
      return has;
    };
    expect(shown(true)).toBe(true);
    expect(shown(false)).toBe(false);
  });
});
