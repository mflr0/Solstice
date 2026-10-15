/**
 * Where the search's values come from: the catalogues (statuses, types…),
 * projects, people, labels, sprints, epics, versions and components. Shared
 * by the filter popovers and the JQL editor's suggestions; what is fetched
 * once is kept for the session.
 */

import {
  EMPTY,
  literal,
  ME,
  valueOf,
  type ListField,
  type PeopleRole,
} from '../../core/filters.js';
import { customKind, fieldInfo, type FieldKind } from '../../core/jql.js';
import type {
  CustomFieldRef,
  Project,
  ProjectValue,
  SprintSuggestion,
  StatusCategory,
} from '../../core/types.js';
import { categoryLabels, t } from '../../i18n/fr.js';
import { api } from '../../lib/api.js';
import { refs } from '../../lib/refs.svelte.js';
import { session } from '../../lib/session.svelte.js';

export interface Option {
  /** The JQL value. */
  value: string;
  label: string;
  hint?: string | undefined;
  /** A heading the option sits under (status categories). */
  group?: string | undefined;
  /** Its keys in the server's counts: ids, project keys, label text ("" for none). */
  facets?: string[] | undefined;
}

/** Lower case, accents removed: "Été" finds "ete". */
export function fold(s: string): string {
  // Only Latin accents go: other scripts' marks are letters (Devanagari vowel signs).
  return s
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase();
}

export function matches(q: string, ...texts: (string | undefined)[]): boolean {
  const f = fold(q.trim());
  return !f || texts.some((x) => x !== undefined && fold(x).includes(f));
}

const CATEGORY_ORDER: StatusCategory[] = ['todo', 'doing', 'done'];

class Catalog {
  projects = $state<Project[] | null>(null);
  sprints = $state<Record<string, SprintSuggestion>>({});
  epics = $state<Record<string, string>>({});
  fields = $state<CustomFieldRef[] | null>(null);
  private loadingProjects: Promise<Project[]> | null = null;
  private loadingFields: Promise<CustomFieldRef[]> | null = null;
  private sprintsLoaded = false;

  loadProjects(): Promise<Project[]> {
    this.loadingProjects ??= api
      .projects()
      .then((list) => (this.projects = list))
      .catch(() => {
        this.loadingProjects = null;
        return [];
      });
    return this.loadingProjects;
  }

  /** The custom fields, for the query editor. */
  loadFields(): Promise<CustomFieldRef[]> {
    this.loadingFields ??= api
      .customFields()
      .then((list) => (this.fields = list))
      .catch(() => {
        this.loadingFields = null;
        return [];
      });
    return this.loadingFields;
  }

  async sprintList(q: string): Promise<SprintSuggestion[]> {
    const list = await api.sprintSuggestions(q);
    for (const s of list) this.sprints[String(s.id)] = s;
    if (!q) this.sprintsLoaded = true;
    return list;
  }

  /** Makes sure sprint names are known, for labels. */
  ensureSprints(): void {
    if (this.sprintsLoaded) return;
    this.sprintsLoaded = true;
    void this.sprintList('').catch(() => (this.sprintsLoaded = false));
  }
}

export const catalog = new Catalog();

/** Project keys among the project filter's values. */
function projectKeys(projects: string[]): string[] {
  return projects.map((v) => literal(v)?.toUpperCase()).filter((k): k is string => !!k);
}

/**
 * Versions or components of the chosen projects (all visible ones when none
 * is chosen), one entry per name, with the projects that have it.
 */
async function projectValues(
  load: (q: string, project?: string) => Promise<ProjectValue[]>,
  projects: string[],
  q: string,
): Promise<Option[]> {
  const keys = projectKeys(projects);
  const list = await load(q.trim(), keys.length === 1 ? keys[0] : undefined);
  // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway value, not state
  const byName = new Map<string, Option>();
  for (const v of list) {
    if (v.archived || (keys.length > 1 && !keys.includes(v.project.key))) continue;
    const hint = v.released ? `${v.project.key} ${t.search.released}` : v.project.key;
    const prev = byName.get(v.name.toLowerCase());
    if (prev) prev.hint = `${prev.hint ?? ''}, ${hint}`;
    else byName.set(v.name.toLowerCase(), { value: valueOf(v.name), label: v.name, hint });
  }
  return [...byName.values()];
}

/**
 * Values to pick for a filter, matching q. projects is the project filter
 * (JQL values), which narrows labels, epics, versions and components.
 */
export async function loadOptions(
  field: ListField,
  q: string,
  projects: string[],
): Promise<Option[]> {
  const meta = session.meta;
  switch (field) {
    case 'project':
      return (await catalog.loadProjects())
        .filter((p) => matches(q, p.key, p.name))
        .map((p) => ({ value: valueOf(p.key), label: p.name, hint: p.key, facets: [p.key] }));
    case 'type': {
      // One entry per name; its counts add up every type of that name.
      // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway value, not state
      const byName = new Map<string, Option>();
      for (const ty of meta?.issueTypes ?? []) {
        if (!matches(q, ty.name)) continue;
        const prev = byName.get(ty.name.toLowerCase());
        if (prev) prev.facets?.push(ty.id);
        else
          byName.set(ty.name.toLowerCase(), {
            value: valueOf(ty.name),
            label: ty.name,
            facets: [ty.id],
          });
      }
      return [...byName.values()];
    }
    case 'status':
      return CATEGORY_ORDER.flatMap((cat) =>
        (meta?.statuses ?? [])
          .filter((s) => s.category === cat && matches(q, s.name))
          .map((s) => ({
            value: valueOf(s.name),
            label: s.name,
            group: categoryLabels[cat],
            facets: [s.id],
          })),
      );
    case 'priority':
      return (meta?.priorities ?? [])
        .filter((p) => matches(q, p.name))
        .map((p) => ({ value: valueOf(p.name), label: p.name, facets: [p.id] }));
    case 'resolution':
      return (meta?.resolutions ?? [])
        .filter((r) => matches(q, r.name))
        .map((r) => ({ value: valueOf(r.name), label: r.name, facets: [r.id] }));
    case 'people':
    case 'assignee':
    case 'reporter': {
      const users = await api.users(q.trim());
      for (const u of users) refs.users[u.username.toLowerCase()] = u.name;
      return users.map((u) => ({
        value: valueOf(u.username),
        label: u.name,
        hint: u.username,
        facets: [String(u.id)],
      }));
    }
    case 'labels': {
      const keys = projectKeys(projects);
      const labels = await api.labels(q.trim(), keys.length === 1 ? keys[0] : undefined);
      return labels.map((l) => ({ value: valueOf(l), label: l, facets: [l] }));
    }
    case 'sprint':
      return (await catalog.sprintList(q.trim())).map((s) => ({
        value: String(s.id),
        label: s.name,
        hint: `${s.boardName} · ${t.boards.sprintStates[s.state]}`,
      }));
    case 'epic': {
      const keys = projectKeys(projects);
      const epics = await api.epics(q.trim(), keys.length === 1 ? keys[0] : undefined);
      for (const e of epics) catalog.epics[e.key] = e.summary;
      return epics
        .filter((e) => keys.length < 2 || keys.includes(e.project))
        .map((e) => ({ value: e.key, label: e.summary, hint: e.key }));
    }
    case 'fixVersion':
      return projectValues(api.versions, projects, q);
    case 'component':
      return projectValues(api.components, projects, q);
  }
}

/** Values with a meaning of their own, offered first: Moi, Non assigné… */
export function specialValues(field: ListField, role: PeopleRole = 'participant'): Option[] {
  const s = t.search.special;
  const me = session.me ? [String(session.me.id)] : [];
  switch (field) {
    case 'people':
      // Only watchers and commenters can be nobody.
      return [
        { value: ME, label: s.me },
        ...(role === 'watcher' ? [{ value: EMPTY, label: s.noWatcher }] : []),
        ...(role === 'commenter' ? [{ value: EMPTY, label: s.noComment }] : []),
      ];
    case 'assignee':
      return [
        { value: ME, label: s.me, facets: me },
        { value: EMPTY, label: s.unassigned, facets: [''] },
      ];
    case 'reporter':
      return [{ value: ME, label: s.me, facets: me }];
    case 'resolution':
      return [{ value: EMPTY, label: s.unresolved, facets: [''] }];
    case 'sprint':
      return [
        { value: 'openSprints()', label: s.openSprints },
        { value: 'futureSprints()', label: s.futureSprints },
        { value: 'closedSprints()', label: s.closedSprints },
        { value: EMPTY, label: s.noSprint },
      ];
    case 'epic':
      return [{ value: EMPTY, label: s.noEpic }];
    case 'fixVersion':
      return [
        { value: 'unreleasedVersions()', label: s.unreleased },
        { value: 'releasedVersions()', label: s.released },
        { value: EMPTY, label: s.noVersion },
      ];
    case 'component':
      return [{ value: EMPTY, label: s.noComponent }];
    case 'labels':
      return [{ value: EMPTY, label: s.noLabel, facets: [''] }];
    default:
      return [];
  }
}

const FUNCTION_LABELS: Record<string, () => string> = {
  currentuser: () => t.search.special.me,
  opensprints: () => t.search.special.openSprints,
  futuresprints: () => t.search.special.futureSprints,
  closedsprints: () => t.search.special.closedSprints,
  unreleasedversions: () => t.search.special.unreleased,
  releasedversions: () => t.search.special.released,
  earliestunreleasedversion: () => t.search.special.earliestUnreleased,
  latestreleasedversion: () => t.search.special.latestReleased,
  standardissuetypes: () => t.search.special.standardTypes,
  subtaskissuetypes: () => t.search.special.subtaskTypes,
};

/** How a chosen value reads on its button: a name rather than an id or username. */
export function labelOf(field: ListField, value: string, role?: PeopleRole): string {
  const special = specialValues(field, role).find((o) => o.value === value);
  if (special) return special.label;
  const fn = /^([A-Za-z]+)\((.*)\)$/.exec(value);
  if (fn) {
    const name = fn[1]?.toLowerCase() ?? '';
    if (name === 'membersof') return t.search.special.membersOf(literal(fn[2] ?? '') ?? '');
    return FUNCTION_LABELS[name]?.() ?? value;
  }
  if (value === EMPTY) return t.common.none;
  const text = literal(value) ?? value;
  switch (field) {
    case 'people':
    case 'assignee':
    case 'reporter':
      refs.ensure({ users: [text], attachments: [], issueKeys: [] });
      return refs.users[text.toLowerCase()] ?? text;
    case 'sprint':
      catalog.ensureSprints();
      return catalog.sprints[text]?.name ?? text;
    case 'epic':
      return catalog.epics[text.toUpperCase()] ?? text;
    default:
      return text;
  }
}

/** A field's kind for the editor: a system field, else a custom field by name or cf[id]. */
export function kindOf(jqlField: string): FieldKind | undefined {
  const info = fieldInfo(jqlField);
  if (info) return info.kind;
  const id = /^cf\[(\d+)\]$/i.exec(jqlField)?.[1];
  const f = (catalog.fields ?? []).find((c) =>
    id ? String(c.id) === id : fold(c.name) === fold(jqlField),
  );
  return f ? customKind(f.kind) : undefined;
}

/** The list a field's values come from, for value suggestions in the editor. */
export function listFieldOf(jqlField: string): ListField | null {
  const kind = kindOf(jqlField);
  switch (kind) {
    case 'project':
    case 'type':
    case 'status':
    case 'priority':
    case 'resolution':
    case 'sprint':
    case 'epic':
    case 'component':
      return kind;
    case 'user':
      return 'assignee';
    case 'label':
      // Custom label fields hold labels of their own: no list for them.
      return fieldInfo(jqlField) ? 'labels' : null;
    case 'version':
      return 'fixVersion';
    default:
      return null;
  }
}
