/** What the share dialog's search offers: people, groups and projects, by name. */

import type { FilterShare, IdName, Project, UserRef } from '../../core/types.js';

/** The choices shown at most. */
export const MAX_CHOICES = 8;

/** A share the search offers, with a username for people. */
export interface ShareChoice {
  share: FilterShare & { id: number; name: string };
  username?: string;
}

const key = (s: FilterShare) => `${s.kind}-${s.id ?? 0}-${s.group ?? 0}`;

/**
 * The people the server found, then the groups and projects whose name (or
 * key) holds the search, leaving out what the view is already shared with.
 */
export function shareChoices(
  q: string,
  people: UserRef[],
  groups: IdName[],
  projects: Project[],
  current: FilterShare[],
): ShareChoice[] {
  const needle = q.trim().toLowerCase();
  if (!needle) return [];
  const has = (s: string) => s.toLowerCase().includes(needle);
  const taken = new Set(current.map(key));
  const all: ShareChoice[] = [
    ...people.map((u) => ({
      share: { kind: 'user' as const, id: u.id, name: u.name },
      username: u.username,
    })),
    ...groups
      .filter((g) => has(g.name))
      .map((g) => ({ share: { kind: 'group' as const, id: g.id, name: g.name } })),
    ...projects
      .filter((p) => has(p.key) || has(p.name))
      .map((p) => ({ share: { kind: 'project' as const, id: p.id, name: p.key } })),
  ];
  return all.filter((c) => !taken.has(key(c.share))).slice(0, MAX_CHOICES);
}
