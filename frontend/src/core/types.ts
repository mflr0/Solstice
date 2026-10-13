/**
 * Shapes of the JSON the Go API returns (backend/internal/api). Dates are
 * RFC 3339 strings in UTC; calendar days (due dates) are "YYYY-MM-DD".
 */

export type StatusCategory = 'todo' | 'doing' | 'done';

export interface Me {
  id: number;
  username: string;
  name: string;
  email: string;
  admin: boolean;
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

export type SearchMode = 'simple' | 'jql';

export interface IssuePage {
  total: number;
  page: number;
  size: number;
  issues: IssueSummary[];
  /** How the server read the query: the simple syntax, or JQL. */
  mode?: SearchMode;
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

export interface AdminUser {
  id: number;
  username: string;
  name: string;
  email: string;
  active: boolean;
  admin: boolean;
  hasPassword: boolean;
  created: string;
}

/** A live notification: an issue changed. */
export interface ChangeEvent {
  key: string;
  what: 'issue' | 'comment' | 'attachment' | 'link' | 'worklog' | 'deleted';
  by: number;
}
