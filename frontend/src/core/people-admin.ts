/**
 * People as administrators shape them: groups, project roles and the
 * account list's filters. Pure logic; the pages word the problems and the
 * usage (src/i18n, blocks groupsAdmin, rolesAdmin and usersAdmin).
 */

import { matches } from './admin-forms.js';
import type { Group, GroupUsage, RoleRow } from './admin-types.js';

/* Names --------------------------------------------------------------------- */

export type NameProblem = 'nameRequired' | 'nameTooLong' | 'nameTaken';

/** The longest name of a group or a role, in characters. */
export const NAME_MAX = 255;

/**
 * Groups are stored in lower case: the name as the server will keep it
 * (and as comments restricted to the group name it).
 */
export function groupKey(name: string): string {
  return name.trim().toLowerCase();
}

/** A name: present, 255 characters at most, and not another one's. */
function nameProblem(
  name: string,
  others: readonly { id: number; name: string }[],
  self: number | null,
  fold: (s: string) => string,
): NameProblem | null {
  const n = name.trim();
  if (!n) return 'nameRequired';
  if ([...n].length > NAME_MAX) return 'nameTooLong';
  const key = fold(n);
  return others.some((o) => o.id !== self && fold(o.name) === key) ? 'nameTaken' : null;
}

/** A group's name; the server compares in lower case. */
export function groupNameProblem(
  name: string,
  groups: readonly Pick<Group, 'id' | 'name'>[],
  self: number | null,
): NameProblem | null {
  return nameProblem(name, groups, self, groupKey);
}

/** A role's name; the server compares without regard to case. */
export function roleNameProblem(
  name: string,
  roles: readonly Pick<RoleRow, 'id' | 'name'>[],
  self: number | null,
): NameProblem | null {
  return nameProblem(name, roles, self, (s) => s.toLowerCase());
}

/* Lists --------------------------------------------------------------------- */

/** The groups a search keeps (name or description). */
export function filterGroups<G extends { name: string; description: string | null }>(
  groups: readonly G[],
  q: string,
): G[] {
  return groups.filter((g) => matches(q, g.name, g.description));
}

/** The roles a search keeps (name or description). */
export function filterRoles<R extends { name: string; description: string | null }>(
  roles: readonly R[],
  q: string,
): R[] {
  return roles.filter((r) => matches(q, r.name, r.description));
}

/** The body of a create or update: a trimmed name and description (empty clears it). */
export function namedPayload(
  name: string,
  description: string,
): { name: string; description: string } {
  return { name: name.trim(), description: description.trim() };
}

/* Usage --------------------------------------------------------------------- */

export type UsageKind = keyof GroupUsage;

/** The order in which a group's uses are told. */
export const USAGE_KINDS: readonly UsageKind[] = [
  'grants',
  'roles',
  'securityLevels',
  'comments',
  'worklogs',
];

/** The uses of a group that exist, in the order above. */
export function usageLines(usage: GroupUsage): { kind: UsageKind; count: number }[] {
  return USAGE_KINDS.map((kind) => ({ kind, count: usage[kind] })).filter((l) => l.count > 0);
}

/**
 * Comments and work logs restricted to the group (or role) keep it in place:
 * the server refuses to delete it, since they would vanish for everyone.
 */
export function isBlocked(usage: Pick<GroupUsage, 'comments' | 'worklogs'>): boolean {
  return usage.comments + usage.worklogs > 0;
}

/**
 * Whether deleting a group takes something with it (grants, roles and
 * security levels held by the group go with it): a typed confirmation is
 * asked then.
 */
export function groupInUse(usage: GroupUsage): boolean {
  return usage.grants + usage.roles + usage.securityLevels > 0;
}

/** Whether deleting a role takes something with it: members in projects or grants. */
export function roleInUse(role: Pick<RoleRow, 'projects' | 'grants'>): boolean {
  return role.projects + role.grants > 0;
}

/**
 * What a 409 refusal of a deletion says is still restricted: the answer's
 * `usage` (comments and work logs). Null when the answer says nothing usable.
 */
export function restrictedUsage(data: unknown): { comments: number; worklogs: number } | null {
  if (!data || typeof data !== 'object' || !('usage' in data)) return null;
  const u = (data as { usage: unknown }).usage;
  if (!u || typeof u !== 'object') return null;
  const { comments, worklogs } = u as Record<string, unknown>;
  const c = typeof comments === 'number' ? comments : 0;
  const w = typeof worklogs === 'number' ? worklogs : 0;
  return c + w > 0 ? { comments: c, worklogs: w } : null;
}

/* Accounts ------------------------------------------------------------------ */

export type UserShow = 'all' | 'active' | 'inactive' | 'admins';

export const USER_SHOWS: readonly UserShow[] = ['all', 'active', 'inactive', 'admins'];

/** The group filter: everyone, people in no group, or one group. */
export type GroupFilter = 'all' | 'none' | number;

/** The account list's filters as the address keeps them (?show=, ?group=). */
export function parseUserShow(param: string | null): UserShow {
  return (USER_SHOWS as readonly string[]).includes(param ?? '') ? (param as UserShow) : 'all';
}

export function parseGroupFilter(param: string | null): GroupFilter {
  if (param === 'none') return 'none';
  const n = Number(param);
  return param && Number.isInteger(n) && n > 0 ? n : 'all';
}

/** What the state and the group choose among the accounts (the search is the server's). */
export function filterUsers<
  U extends { active: boolean; admin: boolean; groups?: { id: number }[] | undefined },
>(rows: readonly U[], show: UserShow, group: GroupFilter): U[] {
  return rows.filter((u) => {
    if (show === 'active' && !u.active) return false;
    if (show === 'inactive' && u.active) return false;
    if (show === 'admins' && !u.admin) return false;
    const mine = u.groups ?? [];
    if (group === 'none' && mine.length > 0) return false;
    if (typeof group === 'number' && !mine.some((g) => g.id === group)) return false;
    return true;
  });
}
