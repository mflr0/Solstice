/**
 * Shapes of the boards API (backend/internal/api/boards.go). Dates are RFC
 * 3339 strings in UTC.
 */

import type { IssueBrief, IssueSummary, StatusRef, UserRef } from './types.js';

export type BoardKind = 'kanban' | 'scrum';
export type SwimlaneStrategy = 'none' | 'custom' | 'assignee' | 'epic' | 'parent';
export type SprintState = 'future' | 'active' | 'closed';

export interface ProjectBrief {
  id: number;
  key: string;
  name: string;
}

export interface SprintRef {
  id: number;
  name: string;
  state: SprintState;
  board?: number;
}

export interface Sprint {
  id: number;
  name: string;
  state: SprintState;
  goal: string | null;
  start: string | null;
  end: string | null;
  completed: string | null;
}

/** What changing a sprint sends: what is left out stays, null clears. */
export interface SprintPatch {
  name?: string;
  goal?: string | null;
  start?: string | null;
  end?: string | null;
}

export interface BoardSummary {
  id: number;
  name: string;
  kind: BoardKind;
  projects: ProjectBrief[];
  owner: UserRef | null;
  activeSprint: SprintRef | null;
}

export interface BoardColumn {
  id: number;
  name: string;
  min: number | null;
  max: number | null;
  statuses: StatusRef[];
}

export interface QuickFilter {
  id: number;
  name: string;
  query: string | null;
  description?: string;
}

export interface Swimlane {
  id: number;
  name: string;
  query: string | null;
  default?: boolean;
}

export interface BoardSettings {
  id: number;
  name: string;
  kind: BoardKind;
  filterId: number;
  query: string;
  subQuery: string | null;
  swimlanes: SwimlaneStrategy;
  estimate: '' | 'points' | 'time';
  /** 'timeoriginalestimate' or 'customfield_N'. */
  estimateField: string | null;
  /** '2w', '14d', '1m'. */
  doneCutoff: string | null;
}

export interface BoardDetail {
  board: BoardSettings;
  projects: ProjectBrief[];
  columns: BoardColumn[];
  quickFilters: QuickFilter[];
  swimlanes: Swimlane[];
  /** Open sprints, the active one first. */
  sprints: Sprint[];
  estimateFields: { id: string; name: string }[];
  can: { manage: boolean };
}

export interface BoardCard extends IssueSummary {
  epic: IssueBrief | null;
  parent: IssueBrief | null;
  /** Story points, or hours. */
  estimate: number | null;
  column: number;
  /** Custom swimlanes only. */
  lane?: number;
  /** Backlog only. */
  sprint?: number;
}

export interface BoardCards {
  cards: BoardCard[];
  sprint: Sprint | null;
  truncated: boolean;
}

export interface Backlog {
  sprints: { sprint: Sprint; cards: BoardCard[] }[];
  backlog: BoardCard[];
  truncated: boolean;
}

/** A card dropped somewhere: another column, place in the order, sprint (0: the backlog). */
export interface CardMove {
  issue: string;
  column?: number;
  after?: string;
  before?: string;
  sprint?: number;
}

/** Lists given replace the current ones: items without an id are created. */
export interface BoardPatch {
  name?: string;
  subQuery?: string;
  doneCutoff?: string;
  swimlanes?: SwimlaneStrategy;
  estimate?: string;
  columns?: {
    id?: number;
    name: string;
    min: number | null;
    max: number | null;
    statuses: string[];
  }[];
  quickFilters?: { id?: number; name: string; query: string; description?: string }[];
  lanes?: { id?: number; name: string; query?: string; default: boolean }[];
}
