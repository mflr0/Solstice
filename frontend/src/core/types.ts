/**
 * Shapes of the JSON the Go API returns (backend/internal/api). Dates are
 * RFC 3339 strings in UTC; calendar days (due dates) are "YYYY-MM-DD".
 */

/** The status categories, in the order of an issue's life. */
export const STATUS_CATEGORIES = ['todo', 'doing', 'done'] as const;

export type StatusCategory = (typeof STATUS_CATEGORIES)[number];

export interface Me {
  id: number;
  username: string;
  name: string;
  email: string;
  admin: boolean;
  /** The interface language chosen on the account (src/i18n/locales.ts); absent on older servers. */
  locale?: string | null;
  /** "ldap" for an account whose password is the directory's; absent on older servers. */
  directory?: string;
}

export interface UserRef {
  id: number;
  username: string;
  name: string;
  active: boolean;
}

export interface StatusRef {
  id: string;
  name: string;
  category: StatusCategory;
}

export interface PriorityRef {
  id: string;
  name: string;
  color?: string;
}

export interface NamedRef {
  id: string;
  name: string;
}

export interface TypeRef {
  id: string;
  name: string;
  subtask: boolean;
}

export interface LinkTypeRef {
  id: number;
  name: string;
  inward: string;
  outward: string;
}

export interface Meta {
  statuses: StatusRef[];
  priorities: PriorityRef[];
  resolutions: NamedRef[];
  /** The resolution of issues reaching a done status without one; '' leaves them unresolved. */
  defaultResolution: string;
  issueTypes: TypeRef[];
  linkTypes: LinkTypeRef[];
}

export interface IdName {
  id: number;
  name: string;
}

export interface IssueSummary {
  key: string;
  summary: string;
  type: TypeRef;
  status: StatusRef;
  priority: PriorityRef | null;
  resolution: NamedRef | null;
  assignee: UserRef | null;
  reporter: UserRef | null;
  created: string;
  updated: string;
  due: string | null;
  confidential: boolean;
}

export interface IssuePage {
  total: number;
  page: number;
  size: number;
  issues: IssueSummary[];
}

export interface IssueBrief {
  key: string;
  summary: string;
  type: TypeRef;
  status: StatusRef;
  assignee: UserRef | null;
}

export interface IssueLink {
  id: number;
  label: string;
  direction: 'outward' | 'inward';
  type: LinkTypeRef;
  issue: IssueBrief;
}

export interface RemoteLink {
  id: number;
  url: string;
  title: string;
  relationship?: string;
}

export interface Attachment {
  id: number;
  filename: string;
  mimeType: string;
  size: number;
  created: string;
  author: UserRef | null;
  url: string;
  missing: boolean;
  canDelete: boolean;
}

export type CustomKind =
  'text' | 'wiki' | 'number' | 'date' | 'datetime' | 'url' | 'option' | 'user' | 'issue' | 'label';

export interface CustomValue {
  id: number;
  name: string;
  kind: CustomKind;
  values: string[];
  users?: UserRef[];
}

export interface Counter {
  count: number;
  me: boolean;
}

export interface TimeTracking {
  original: number | null;
  remaining: number | null;
  spent: number | null;
}

/** Permission names, see backend/internal/perm. */
export type Permission =
  | 'browse'
  | 'create'
  | 'edit'
  | 'comment'
  | 'transition'
  | 'assign'
  | 'assignable'
  | 'attach'
  | 'link'
  | 'worklog'
  | 'delete'
  | 'moderate'
  | 'watchers'
  | 'security'
  | 'admin';

export type Can = Partial<Record<Permission | 'configure', boolean>>;

/** A move the project's workflow allows from the current status. */
export interface Transition {
  id: number;
  name: string;
  to: StatusRef;
}

export interface IssueDetail extends IssueSummary {
  id: number;
  project: { id: number; key: string; name: string };
  description: string | null;
  environment: string | null;
  creator: UserRef | null;
  resolved: string | null;
  time: TimeTracking;
  securityLevel: IdName | null;
  parent: IssueBrief | null;
  subtasks: IssueBrief[];
  labels: string[];
  components: IdName[];
  affectsVersions: IdName[];
  fixVersions: IdName[];
  links: IssueLink[];
  remoteLinks: RemoteLink[];
  attachments: Attachment[];
  customFields: CustomValue[];
  watchers: Counter;
  votes: Counter;
  can: Can;
  /** The current status, then the ones reachable from it. */
  statuses: StatusRef[];
  transitions: Transition[];
  securityLevels: IdName[];
  commentCount: number;
  /** The epic it belongs to (boards). */
  epic?: IssueBrief | null;
  /** The sprints it has been in, closed ones included. */
  sprints?: { id: number; name: string; state: 'active' | 'future' | 'closed'; board?: number }[];
}

/** What PATCH returns when the change hid the issue from its author. */
export interface HiddenIssue {
  key: string;
  hidden: true;
}

export interface Visibility {
  group?: string;
  role?: string;
}

export interface Comment {
  id: number;
  author: UserRef | null;
  body: string;
  created: string;
  updated: string;
  updatedBy: UserRef | null;
  visibility: Visibility | null;
  canEdit: boolean;
}

export interface HistoryItem {
  field: string;
  fieldType: string;
  from: string | null;
  to: string | null;
}

export interface HistoryEntry {
  id: number;
  author: UserRef | null;
  created: string;
  items: HistoryItem[];
}

export interface Worklog {
  id: number;
  author: UserRef | null;
  started: string;
  seconds: number;
  body: string | null;
  visibility: Visibility | null;
  /** Whether the user may edit or delete it: its author, or a moderator. */
  canEdit: boolean;
}

/** What changing a work log sends; what is left out stays as it is. */
export interface WorklogPatch {
  started?: string;
  seconds?: number;
  body?: string;
}

export interface Project {
  id: number;
  key: string;
  name: string;
  description?: string | null;
  kind: string;
  lead: UserRef | null;
  category: string | null;
  open: number;
}

export interface Version {
  id: number;
  name: string;
  released: boolean;
  archived: boolean;
  releaseDate: string | null;
}

export interface Component {
  id: number;
  name: string;
  archived: boolean;
}

export interface ProjectDetail {
  project: Project;
  issueTypes: TypeRef[];
  components: Component[];
  versions: Version[];
  securityLevels: IdName[];
  defaultAssignee: 'unassigned' | 'lead';
  can: Can;
}

export type Holder = 'everyone' | 'user' | 'group' | 'role' | 'lead' | 'reporter' | 'assignee';

export interface Grant {
  permission: Permission;
  holder: Holder;
  id?: number;
  name?: string;
}

export interface Role {
  id: number;
  name: string;
  users: UserRef[];
  groups: IdName[];
}

export interface ProjectPermissions {
  permissions: Permission[];
  grants: Grant[];
  roles: Role[];
  groups: IdName[];
}

export interface Person {
  user: UserRef;
  email: string;
  assigned: IssueSummary[];
}

export interface Refs {
  users: Record<string, string>;
  issues: Record<string, boolean>;
}

/** A live notification: an issue changed. */
export interface ChangeEvent {
  key: string;
  what: 'issue' | 'comment' | 'attachment' | 'link' | 'worklog' | 'deleted';
  by: number;
}

/* Workflows ---------------------------------------------------------------- */

/** A move between two statuses of a workflow; from null: from any status. */
export interface WorkflowTransition {
  id: number;
  name: string;
  from: StatusRef | null;
  to: StatusRef;
}

export interface Workflow {
  id: number;
  name: string;
  default: boolean;
  initial: StatusRef;
  /** In the workflow's order. */
  statuses: StatusRef[];
  transitions: WorkflowTransition[];
  /** The issue types following it; the default takes every type not given another. */
  issueTypes: TypeRef[];
  /** Issues per status id. */
  usage: Record<string, number>;
}

export interface ProjectWorkflows {
  workflows: Workflow[];
  /** The project's issue types. */
  issueTypes: TypeRef[];
  /** The whole status catalogue, shared by every project. */
  statuses: StatusRef[];
  can: { edit: boolean };
}

/** What saving a workflow sends. */
export interface WorkflowInput {
  name: string;
  initial: string;
  statuses: string[];
  transitions: { id?: number; name: string; from: string | null; to: string }[];
  /** Ignored for the default workflow. */
  issueTypes?: string[];
  /** Makes it the project's default. */
  default?: true;
  /** Removed status id → status its issues move to. */
  remap?: Record<string, string>;
  /** For issue types going back to the default workflow: status it lacks → one of its statuses. */
  remapDefault?: Record<string, string>;
}

/** Issues sitting in a status a workflow is about to lose. */
export interface StrandedStatus {
  status: StatusRef;
  count: number;
}

/** Issues of types going back to the default workflow, in statuses it lacks. */
export interface StrandedDefault {
  stranded: StrandedStatus[];
  workflow: { id: number; name: string; statuses: StatusRef[] };
}

/* Search -------------------------------------------------------------------- */

export interface FilterShare {
  kind: 'everyone' | 'group' | 'project' | 'user';
  id?: number;
  /** With kind project: only this role. */
  role?: number;
  name?: string;
}

/** A saved search (GET /api/filters). */
export interface SavedFilter {
  id: number;
  name: string;
  description: string | null;
  /** JQL, or the former search syntax for old filters. */
  query: string;
  owner: UserRef | null;
  mine: boolean;
  favourite: boolean;
  canEdit: boolean;
  shares: FilterShare[];
}

/** A sprint offered as a search value (GET /api/sprints). */
export interface SprintSuggestion {
  id: number;
  name: string;
  state: 'active' | 'future' | 'closed';
  board: number;
  boardName: string;
}

/** Issue counts per value of a few fields (GET /api/issues/facets). */
export interface Facets {
  total: number;
  /** field → value → count; values are ids (users, statuses…), project keys, label text; "" counts the empty ones. */
  facets: Record<string, Record<string, number>>;
}

/** An epic offered as a search value (GET /api/epics). */
export interface EpicRef {
  key: string;
  summary: string;
  status: StatusRef;
  project: string;
}

/** A custom field, for the query editor (GET /api/fields). */
export interface CustomFieldRef {
  id: number;
  name: string;
  /** The field type: float, select, multiselect, datepicker, userpicker, labels… */
  kind: string;
  /** How a query names it, quoted when needed. */
  jql: string;
}

/** A version or component across projects (GET /api/versions, /api/components). */
export interface ProjectValue {
  id: number;
  name: string;
  project: { id: number; key: string; name: string };
  released?: boolean;
  archived?: boolean;
}
