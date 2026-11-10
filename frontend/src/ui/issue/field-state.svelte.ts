import type { LayoutField } from '../../core/admin-types.js';
import { draftOf, restValue, type CustomDraft } from '../../core/custom-fields.js';
import {
  customFieldId,
  defaultViewLayout,
  requiredFields,
  viewZones,
} from '../../core/issue-view.js';
import type { IssueDetail, OfferedField, ProjectDetail } from '../../core/types.js';
import { api, type IssueFields } from '../../lib/api.js';

// Typed values are saved when the field is left, choices as they are made.
const typed = ['text', 'textarea', 'url', 'number', 'date', 'datetime'];

/**
 * What the issue view's fields share, in the main column and the side card:
 * the view layout, and what editing them needs (the project, the offered
 * custom fields and their drafts). Call destroy when the view goes.
 */
export class FieldState {
  /** The view layout; null until it arrives. */
  view = $state<readonly LayoutField[] | null>(null);
  /** The view layout couldn't be read: the view shows the default one. */
  failed = $state(false);
  /** The fields the create layout requires, only read for editors. */
  required = $state<readonly string[]>([]);
  project = $state<ProjectDetail | null>(null);
  offered = $state<OfferedField[]>([]);
  // Reassigned, not mutated: a derived value is not deeply reactive.
  drafts = $derived.by<Record<number, CustomDraft>>(() => {
    const issue = this.#issue();
    if (!issue) return {};
    return Object.fromEntries(
      this.offered.map((f) => [
        f.id,
        draftOf(
          f,
          issue.customFields.find((v) => v.id === f.id),
        ),
      ]),
    );
  });
  zones = $derived.by(() => {
    const issue = this.#issue();
    // As the server resolves it: the offered fields the layout misses go last in the side card.
    const layout =
      this.view ??
      (this.failed
        ? [
            ...defaultViewLayout,
            ...this.offered.map((f) => ({
              field: customFieldId(f.id),
              zone: 'side' as const,
              hidden: false,
            })),
          ]
        : null);
    if (!layout || !issue) return { main: [], side: [], unplaced: [] };
    return viewZones(layout, issue.customFields);
  });

  readonly onchange: (fields: IssueFields) => Promise<boolean>;
  readonly #issue: () => IssueDetail | null;
  readonly destroy: () => void;

  constructor(
    issue: () => IssueDetail | null,
    onchange: (fields: IssueFields) => Promise<boolean>,
  ) {
    this.#issue = issue;
    this.onchange = onchange;
    this.destroy = $effect.root(() => {
      const key = $derived(issue()?.project.key);
      const type = $derived(issue()?.type.id);
      const edit = $derived(issue()?.can.edit === true);

      // Not `issue` in the callbacks: by then the view may have let go of it.
      $effect(() => {
        const k = key;
        const ty = type;
        if (!k || !ty) return;
        let current = true;
        api.layout(k, ty, 'view').then(
          (l) => {
            if (current) {
              this.view = l.fields;
              this.failed = false;
            }
          },
          () => {
            if (current) {
              this.view = null;
              this.failed = true;
            }
          },
        );
        return () => {
          current = false;
        };
      });

      $effect(() => {
        const k = key;
        const ty = type;
        if (!k || !ty || !edit) return;
        let current = true;
        api.layout(k, ty, 'create').then(
          (l) => {
            if (current) this.required = requiredFields(l.fields);
          },
          () => {},
        );
        return () => {
          current = false;
          this.required = [];
        };
      });

      // Components, versions and types come from the project, only needed to edit.
      $effect(() => {
        const k = key;
        if (!k || !edit) return;
        let current = true;
        void api.project(k).then((p) => {
          if (current) this.project = p;
        });
        return () => {
          current = false;
        };
      });

      // The custom fields the project and type offer, only needed to edit.
      $effect(() => {
        const k = key;
        const ty = type;
        if (!k || !ty || !edit) return;
        let current = true;
        void api.projectFields(k, ty).then((f) => {
          if (current) this.offered = f;
        });
        return () => {
          current = false;
          this.offered = [];
        };
      });
    });
  }

  /** The offered field of a custom field id. */
  offeredField(id: number): OfferedField | undefined {
    return this.offered.find((f) => f.id === id);
  }

  setCustom(f: OfferedField, v: CustomDraft) {
    this.drafts = { ...this.drafts, [f.id]: v };
    if (!typed.includes(f.type)) void this.saveCustom(f);
  }

  async saveCustom(f: OfferedField) {
    const stored = () =>
      draftOf(
        f,
        this.#issue()?.customFields.find((v) => v.id === f.id),
      );
    const next = restValue(f, this.drafts[f.id] ?? stored());
    if (JSON.stringify(next) === JSON.stringify(restValue(f, stored()))) return;
    if (!(await this.onchange({ customFields: { [String(f.id)]: next ?? null } }))) {
      this.drafts = { ...this.drafts, [f.id]: stored() };
    }
  }

  // A row torn down with the focus in it still sends focusout, without its field.
  leaveCustom(e: FocusEvent, f: OfferedField | undefined) {
    if (!f || !typed.includes(f.type)) return;
    const to = e.relatedTarget;
    if (to instanceof Node && (e.currentTarget as HTMLElement).contains(to)) return;
    void this.saveCustom(f);
  }

  customKeys(e: KeyboardEvent, f: OfferedField) {
    if (e.key === 'Enter' && e.target instanceof HTMLInputElement && typed.includes(f.type)) {
      e.preventDefault();
      void this.saveCustom(f);
    }
  }
}
