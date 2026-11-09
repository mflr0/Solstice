import type { BoardCard } from '../../core/board-types.js';
import { paths } from '../../core/route.js';
import { t } from '../../i18n/t.js';
import { api, errorMessage } from '../../lib/api.js';
import { copyText } from '../../lib/clipboard.js';
import { notify } from '../../lib/ui.svelte.js';

/** The "Issue" group of a card's menu (a CardMenu group): duplicate, watch, copy the link. */
export function issueGroup(
  card: BoardCard,
  duplicate: (key: string) => void,
  watched: (key: string, on: boolean) => void,
): { label: string; items: { label: string; run: () => void }[] } {
  return {
    label: t.boards.issueGroup,
    items: [
      ...(card.can.create ? [{ label: t.boards.duplicate, run: () => duplicate(card.key) }] : []),
      {
        label: card.watching ? t.boards.stopWatching : t.issue.watch,
        run: () => void watch(card, watched),
      },
      { label: t.boards.copyLink, run: () => void copyLink(card.key) },
    ],
  };
}

async function watch(card: BoardCard, watched: (key: string, on: boolean) => void) {
  const on = !card.watching;
  try {
    const counter = await api.watch(card.key, on);
    watched(card.key, counter.me);
    notify(counter.me ? t.boards.watched(card.key) : t.boards.unwatched(card.key));
  } catch (e) {
    notify(errorMessage(e), 'error');
  }
}

async function copyLink(key: string) {
  const url = `${location.origin}${paths.issue(key)}`;
  if (await copyText(url)) notify(t.projects.linkCopied);
  else notify(t.projects.copyLinkFailed(url), 'error');
}
