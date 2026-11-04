/** Editing who sees a board: the list of its filter's shares. */

import { t } from '../i18n/t.js';
import type { FilterShare } from './types.js';

function sameShare(a: FilterShare, b: FilterShare): boolean {
  return a.kind === b.kind && (a.id ?? 0) === (b.id ?? 0) && (a.group ?? 0) === (b.group ?? 0);
}

/** The list with this share, once. */
export function addShare(list: FilterShare[], share: FilterShare): FilterShare[] {
  return list.some((s) => sameShare(s, share)) ? list : [...list, share];
}

export function removeShare(list: FilterShare[], share: FilterShare): FilterShare[] {
  return list.filter((s) => !sameShare(s, share));
}

/** Whether a share of before is gone from after. */
export function sharesLost(before: FilterShare[], after: FilterShare[]): boolean {
  return before.some((s) => !after.some((a) => sameShare(s, a)));
}

/** A share in words: its kind, then whom it names. */
export function shareLabel(share: FilterShare): string {
  const kind = t.boards.shareKinds[share.kind];
  return share.kind === 'everyone' ? kind : `${kind}${t.common.labelSep}${share.name ?? ''}`;
}
