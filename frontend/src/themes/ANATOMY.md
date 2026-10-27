# Theme anatomy

How a theme ("skin") plugs into Solstice. Carnet is the base look and lives in
`src/app.css`; every other skin is one file, `src/themes/<id>.css`, plus an
optional folder `src/themes/<id>/` for its images.

Ids: `carnet`, `orbite`, `crayon`, `luna`, `retro`, `classique`, `terminal`, `plan`.

## 1. Writing the file

- **Anchor every selector on the skin attribute**: `[data-skin='<id>'] …`, or
  `[data-skin='<id>'][data-scheme='dark'] …` for the dark variant. Never `:root`,
  `html` or `body` as the anchor: the skin root is either `<html>` or a
  `<div class="skin-preview">` in the theme gallery.
- **Fonts**: `@import '@fontsource/<font>/latin-400.css';` at the top of the
  file (packages in `package.json`; ask main to add one). Free licences only.
  System fonts (Tahoma, Verdana, …) only as `font-family` fallbacks, never
  bundled. No Microsoft or Atlassian names, logos, icons, fonts or assets,
  anywhere (code, comments, UI).
- **Images**: `url('./<id>/thing.svg')` — relative to your file; Vite emits them.
- **Motion**: wrap animations in `@media (prefers-reduced-motion: no-preference)`.
- **SVG filters** (roughness…) on frames only, never on text: draw the frame on a
  pseudo-element and filter that.
- No `!important` unless a Svelte inline `style=` forces it.

### How the file is loaded (you do not need to do anything)

`src/lib/skins.ts` imports the file on demand (`?inline`, code-split) and
injects it as `<style data-skin-css="<id>">` after every other stylesheet, so
**your rules win specificity ties**. When the browser supports `@scope`, the
loader wraps the file in `@scope ([data-skin='<id>']) to ([data-skin])` and
rewrites `[data-skin='<id>']` to `:scope`: your rules then stop at any nested
element carrying another `data-skin` (gallery previews), and specificity is
unchanged. `@font-face`, `@keyframes`, `@import`, `@property`,
`@counter-style` stay top-level. While the file loads, `<html>` carries
`data-skin-loading` and the page is hidden (1.2 s at most).

## 2. Tokens

Two token blocks, written **exactly** like this (the contrast test parses them):

```css
[data-skin='<id>'] {
  /* every token below */
  color-scheme: light;
}
[data-skin='<id>'][data-scheme='dark'] {
  /* colours that change */
  color-scheme: dark;
}
```

The first block must declare **every** token Carnet declares (the test checks
this): a preview of your skin sits inside a page of another skin, and anything
you leave out would be inherited from it. No dark block = a light-only skin
(the gallery hides the mode buttons, the dark mode shows your light look).

Colour tokens are `#rrggbb` or `var(--other-token)`; `--primary` and `--bar`
may be gradients (every `#rrggbb` stop is checked).

| Group       | Tokens                                                                                                                                                                                                                                                                                                                                           |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Surfaces    | `--desk` (behind the page), `--paper`, `--paper-raised`, `--paper-sunk`                                                                                                                                                                                                                                                                          |
| Text        | `--ink`, `--ink-soft`, `--muted`                                                                                                                                                                                                                                                                                                                 |
| Lines       | `--rule`, `--rule-strong`, `--rule-stroke` (a `background` for section dividers), `--rule-height`                                                                                                                                                                                                                                                |
| Accent      | `--accent` (fills, borders, marks), `--accent-text` (accent used as text), `--accent-wash` (hover tint), `--sun` (logo), `--margin-line`, `--focus` (focus ring)                                                                                                                                                                                 |
| Main button | `--primary`, `--primary-ink`                                                                                                                                                                                                                                                                                                                     |
| Top bar     | `--bar`, `--bar-ink`, `--bar-ink-soft`                                                                                                                                                                                                                                                                                                           |
| Statuses    | `--stamp-todo`, `--stamp-doing`, `--stamp-done` + shape tokens `--stamp-font`, `--stamp-case`, `--stamp-tracking`, `--stamp-border`, `--stamp-radius`, `--stamp-padding`, `--stamp-fill` (tint %, checked), `--stamp-dot`, `--stamp-large-size`, `--stamp-large-padding`, `--stamp-large-border`, `--stamp-large-tracking`, `--stamp-large-turn` |
| Issue types | `--tone-bug`, `--tone-story`, `--tone-epic`, `--tone-task`, `--tone-subtask`                                                                                                                                                                                                                                                                     |
| Depth       | `--shadow-pop`, `--backdrop`, `--ink-blend`                                                                                                                                                                                                                                                                                                      |
| Type        | `--font-title`, `--font-body`, `--font-mono`, `--font-size`, `--title-weight`, `--h1-weight`, `--title-tracking`, `--title-italic`, `--empty-font`                                                                                                                                                                                               |
| Shapes      | `--frame` (border width), `--radius-xs`, `--radius-s`, `--radius-m`, `--radius-l`, `--avatar-radius`, `--card-top`                                                                                                                                                                                                                               |
| Background  | `--page-texture`, `--page-texture-size`, `--loader` (`none` or `inline-block`: the base orbit loader)                                                                                                                                                                                                                                            |

New since the Orbite/Crayonné blocks were copied out of `app.css`: `--desk`,
`--accent-text`, `--focus`, `--bar`, `--bar-ink`, `--bar-ink-soft`,
`--font-size`. `--accent-ink` is gone (use `--primary-ink`).

### Contrast (tests/core/contrast.test.ts, every skin × scheme, no exceptions)

Text, 4.5:1 — `--ink`, `--ink-soft`, `--muted` on `--paper`, `--paper-raised`,
`--paper-sunk`; `--accent-text` on `--paper`, `--paper-raised`; each stamp ink
on `--paper`, `--paper-raised` and on its own tint (`--stamp-fill`); tones on
`--paper-raised`; `--paper` on `--tone-epic`; `--primary-ink` on `--primary`;
`--bar-ink`, `--bar-ink-soft` on `--bar`.
Non-text, 3:1 — `--focus` and `--accent` on `--paper`.

Any other pair your file draws (title bars, inverted rows…): declare it in a
comment anywhere in the file and the test checks it too, in every scheme:

```css
/* contrast: --title-bar-ink on --title-bar */
/* contrast: --rule-strong on --paper 3 */
```

## 3. Hooks

Stable class names (additive; old classes kept). States use ARIA or the
`is-*` classes listed.

**Shell** — `[data-skin]` root (`<html>` or `.skin-preview`): paints `--desk` +
`--page-texture`. `body` is transparent. `.app-shell` (a flex column at least
a screen tall; `main.app-main` grows, so a shown status bar sits at the foot
of short pages) > `.topbar`, `main.app-main`, `footer.app-statusbar` (hidden
by default: `display: none`; cells `.statusbar-cell`, the second is the
connection, `.is-live` while connected). `.app-booting` (first load).

**Top bar** — `header.topbar` > `.topbar-inner` > `a.brand` (`.logo` +
`.wordmark`), `nav.topbar-nav` > `a.nav-link` (`[aria-current='page']`),
`form.topbar-search.quick-search` > `input.input` (see **Search**), `.live-dot` (`.on` when connected),
`a.topbar-themes` (gallery link, icon), `.btn.btn-primary.topbar-create` >
`.plus` (hidden on wide screens) + `.label` + `kbd`, `.account` >
`button.account-button` > `.avatar`, `.popover.account-menu`. The nav has
four links (and may gain more): do not count on `:nth-child`. Short of room,
`.topbar-search` shrinks first, then the nav scrolls sideways inside the bar
(`overflow-x: auto` at every width, links never wrap: a `.nav-link`
decoration drawn outside the nav box is clipped; the current link is
scrolled into view). Narrow screens, in the base: ≤ 860 px the search field
and the `kbd` go and `.live-dot` takes `margin-left: auto`; ≤ 600 px `.wordmark` and `.topbar-create .label` are
visually hidden (still read) and `.plus` shows, sized `1.55em` — a skin that
sets the button's `font-size: 0` to draw its own « + » hides it too; ≤ 420 px
`.topbar-themes` goes (the gallery stays on Account → Appearance).

**Logo** — `svg.logo` > `.logo-sun`, `.logo-line`, `.logo-faint` (Carnet);
Orbite: `.logo-orbit`, `.logo-planet`; Crayonné: `.logo-sketch`, `.logo-ray`.
To use your own mark: `[data-skin='x'] .logo > * { display: none }` and paint
`.logo` with `background: url('./x/logo.svg') center / contain no-repeat`.

**Page** — `.page` (the sheet; `[data-title]` holds the page's title, for a
window title bar: `content: attr(data-title)`), `header.page-head` (wraps
under 600 px), `.eyebrow`, `h1.page-title`, `h2.section-title` (`::after` is
the divider, drawn with `--rule-stroke`; `.count`, `.actions`).

Issue page: `.page.issue` > `nav.page-crumbs`, `header.issue-head`
(`h1.page-title`, `.issue-meta`), `.issue-layout` (`.issue-sheet`, then
`aside.card` = the record).

- `.issue-meta` > `.meta-cell` (status, `.resolution`, type, priority,
  `.created`, `.actions`) between `span.meta-sep.dot` (the `·`). Status:
  `.status` > `button.status-trigger` (when the workflow allows a move) >
  `.stamp.large` + `.caret` (▾), then `.popover[role=menu]`.
- Lists: `ul.subtasks > li.list-row`; the activity `.issue-activity` > `.tabs`
  and `[role=tabpanel].panel` > comments `ol.thread > li.list-row.comment`
  (`header`, `.wiki`), history `ol.history > li.list-row` (`.items` > `li` >
  `.field`, `.old` struck out, `.arrow`, `.new`).
- Attachments: `section.attachments` (`.dragging`) > `ul.grid` >
  `li.file` (`.missing`) > `a.thumb` (an `img`, or `.sheet` > `.ext`, the file
  extension) + `.meta` (`.name`, `.sub`) + `button.del`; `p.drop` while a
  file is dragged over.
- The record: `aside.card.fiche` > `.card-head`, `dl` > `.line.card-row`
  (`dt` + `dd`); `.card-head.sub` above custom fields.

Project page: `.page-head` (`.eyebrow`, `h1.page-title` > `.pkey`, `p.lead`),
`dl.facts` (versions and components as `.chip`), then the issue search
(`.issue-search`, see **Search**; the project is fixed, so no project button).
Person page: `.page-head .id` > `.mark.avatar.avatar-large` (the base sets its
size at specificity 0-4-0, so skins' small-avatar rules do not shrink it;
resize it with e.g. `[data-skin='x'] .page-head .avatar.avatar-large`) +
`h1.page-title` and `p.meta.person-meta`. The former search box
(`form.search`, `.mode`, `.popover.help`) is gone: see **Search**. Project
settings: `.intro`, `.savebar` (below).

**Panels / windows** — `.card` with `.card-head` as its title bar (the issue
record). The sign-in sheet `.card.login-card` has no `.card-head`: its title
area is `.login-card > .brand` (logo + `.wordmark`), then `.tagline`, the
form, `.note`. Wiki `{panel}`: `.wiki .panel`, `.wiki .panel-title`.

**Buttons** — `.btn` (secondary), `.btn-primary`, `.btn-quiet` (ghost),
`.btn-danger`, `.btn-small`; pressed: `:active`, `.on` / `[aria-pressed='true']`;
`[disabled]`. `.link-btn` (text button). `kbd` inside buttons.

**Fields** — `.field` > `.field-label` (or first `span`), `.input`, `.select` (a dropdown's button),
`.textarea`, `input.checkbox`, `.hint`; `.field-box` (also `.boxed`) frames a
picker like an input in the create dialog (assignee, components, versions,
labels). `.chip` (labels, values). `.key` (issue keys, mono).

**Tables** — `table.ledger` > `thead th`, `tbody tr`, `td`; `.num` (right-aligned
numbers); `tr.is-selected` (reserved). Rows are clickable (`cursor: pointer`).
Issue lists: every `th` and `td` carries its column — `.c-type`, `.c-key`,
`.c-summary` (`.summary` inside), `.c-status`, `.c-priority`, `.c-assignee`
(also `.c-who`), `.c-due` (`.is-overdue`, also `.late`, when past and not
done), `.c-updated` (also `.num`). Base on narrow screens: ≤ 760 px priority
and assignee are hidden, ≤ 600 px due date and last change too, and the
summary wraps on two lines; a skin that lays rows out itself (grid, flex)
sets `display` on the cells it keeps. `.table-scroll` wraps a table that may
be wider than a phone (users, project permissions): it scrolls inside, not
the page. Users list: `td.tags > .tag-list` (the flex row of marks).

**Tabs** — `.tabs[role=tablist]` > `button.tab[role=tab]`
(`[aria-selected='true']`). `.tabs.tabs-folder` = folder tabs over an editor.

**Menus** — `.popover` (`[role=menu]` or `[role=listbox]`) > `.item`
(`.active`, `[aria-checked='true']`, `[aria-selected='true']`, `.danger`),
`.sep`, `.group-label`.

**Dropdowns** — one value from a list; the app has no native select:
`button.dropdown[role=combobox]` (`.select` in a form, `.quiet` in the issue
sidebar; `[aria-expanded='true']` while open) > `.icon` + `.name` (`.name.none`
= the placeholder). The list is `ul.popover.dropdown-list[role=listbox]` in the
top layer > `li.item[role=option]` (`.active` = highlighted,
`[aria-selected='true']` = the value, ✓ after it) > `.icon` + `.name`. Grouped
options sit in `li[role=group]` > `.group-label` + `ul.group` > `li.item`.

**Dialogs** — `dialog.modal` (`::backdrop`) > `header.modal-head` (title bar) >
`h2.modal-title` + `button.modal-close` (label « Fermer Échap »; restyle it,
e.g. as a ✕ box with `font-size: 0` and a `::before`), `.modal-body`,
`footer.modal-foot`.

**Save bar** — `.savebar` (unsaved changes, sticky at the bottom of the page):
its first child is the message (or a `.error-text`), then `.btn.btn-quiet`
and `.btn.btn-primary`.

**Workflows** (project settings, « Workflows » tab) — the list is a
`table.ledger`; the editor has a `table.ledger.matrix` of transitions (rows
`th[scope=row]`, checked cells `td.on`, the diagonal `td.self`) and
`figure.diagram` > `.lane` > `.group` (`.caption`, `ol.nodes` > `li` >
`.stamp`) + `svg.wires`: `g.wire` (`.forward` above the row, `.backward`
below, `.any` from the dotted `.bus`; `.to-todo|.to-doing|.to-done` set
`--c` from the stamp inks) > `.line` + `.head`, and `g.start` (start
marker). The arrows draw with `currentColor`; restyle strokes, not colours.

**Toasts** — `.toasts` > `p.toast` (`.error`, `.info`). Tooltips: native
`title` only, nothing to style.

**Marks** — `span.stamp` (`.todo` / `.doing` / `.done`, `[data-category]`,
`.large` on the issue page; `::before` is the optional dot, `--stamp-dot`).
`.type-mark[data-type='bug|story|epic|task|subtask']`. `.priority-mark` >
`.glyph[data-level='highest|high|medium|low|lowest|other']` (its colour comes
from the server in an inline `--priority-color`, drawn with
`color: var(--priority-color, var(--ink-soft))`: override `color`, or mix the
variable). `.avatar` (initials; `.avatar-large` on the person page).

**Empty and waiting** — `p.empty`; `.illustration[data-kind='lost|empty|search']`
before it (an empty box, `display: none` by default — show it and give it a
`background-image` and a size, or masks on `::before` / `::after` as Orbite
does). `.loading` (its `::before` is the loader). `.error-text`.

**Pager** — `nav.pager` > `.range`, `.btn.btn-small`.

**Search** (`/issues`, and the issue list of a project page) — `.issue-search`.
The search lives in the address (`?jql=…`), so every hook below is redrawn on
back / forward.

- Filter bar: `.filter-bar[role=group]` > `.filter-text` (`.glass` = the
  magnifier, `input.input`) + `.filter-buttons` > one `.filter-wrap` per
  criterion: `button.btn.btn-small.filter-button` (`[aria-expanded]`;
  `.is-active` when it holds values; inside: `.filter-name`, `.filter-value` =
  the chosen values, `.caret`), then `button.btn.btn-small.filter-clear` (the
  ×, only when active), then `.popover.filter-popover[role=dialog]` (`.wide`
  for dates and « Plus »). « Plus » lists further criteria in
  `ul.more-list > li > label.item` (checkboxes; `.busy` when disabled, with
  `p.busy-note`). `.link-btn.clear-all`. The first button, « Personnes », opens
  with `.role` > `.role-field` (`button.dropdown.select`: the role) + `p.role-hint`
  above its list.
  The base sets only `background-color` on these buttons, so a skin can paint
  `background-image` fills (Plan does).
- Inside a popover: `.value-picker` > `input.input.filter-search` +
  `ul.filter-options[role=listbox]` > `li.group-label` (« Raccourcis », status
  categories) and `li.item.filter-option[role=option]` (`[aria-selected='true']`
  when ticked, `.active` under the keyboard; `.box` = the tick box, `.label`,
  `.hint`, `.count` = issues it would show, `.zero` when none; counts come
  in a moment after the list), then `.picker-foot` (« Effacer »). Dates: `.date-picker` >
  `fieldset.date-presets` > `label.item.date-preset` (`.active`), `.custom` >
  two `input.input[type=date]`.
- ≤ 600 px a popover is a sheet along the bottom of the screen
  (`position: fixed`), over `.filter-veil` (painted with `--backdrop`), with
  `.sheet-head` (name + close button) shown; the bar wraps.
- Under the bar: `.search-tools` > `.count`, `.sort` (`button.dropdown.select`,
  `button.sort-dir`), `.mode-switch[role=group]` (two `.btn.btn-small`,
  `[aria-pressed='true']` on the current mode); `p.refused` (« Requête trop
  complexe pour les filtres »); `.results` (the `.ledger`).
- JQL mode: `.jql-editor` (`.has-error`) > `.jql-field` > `pre.jql-highlight`
  (the coloured copy, `aria-hidden`) under a transparent
  `textarea.jql-input`: both draw the same box, so give them the same font,
  padding, line-height and border width (style them together, or leave
  them). Tokens: `span.jql-token-field`, `-operator`, `-keyword`, `-value`,
  `-string`, `-function`, `-punct`, `-error` (base inks: `--stamp-doing`,
  `--muted`, `--tone-epic`, `--ink`, `--stamp-done`, `--tone-task`,
  `--accent-text` with a wavy underline). Suggestions:
  `ul.popover.jql-suggestions[role=listbox]` > `li.item` (`.active`, `.label`,
  `.detail`). `.jql-foot` (`.hint`, the run button), `p.error-text.jql-error`.
- Saved filters (search page): `.search-page` > `.search-layout` >
  `aside.saved-filters` + `.search-main`. In the aside: `button.saved-toggle`
  (shown ≤ 1000 px, unfolds `.saved-body.open`), `h2.saved-title`,
  `h3.group-label` (Favoris, Mes filtres, Partagés avec moi),
  `li.saved-item` (`.is-current`) > `button.star` (`[aria-pressed='true']`
  for a favourite) + `a` (`[aria-current='page']`; `.name`, `.owner`). The
  page head shows `.modified` (« modifié ») when the open filter was changed.
- Quick search (top bar): `form.topbar-search.quick-search` >
  `input.input#quick-search[role=combobox]`, `kbd`,
  `ul.popover.quick-search-results[role=listbox]` > `li.group-label`, `li.sep`,
  `li[role=option]` > `a.item` (`.quick-issue`, `.quick-project`,
  `.quick-person`, `.quick-all`; `.active` under the keyboard) holding
  `.type-mark`, `.key`, `.summary`, `.stamp`, `.who`, `.hint`. Each person is
  followed by three small options on one line, `li.sub` (`.first` on the
  first) > `a.item.quick-assigned`, `.quick-reported`, `.quick-profile`.

**Wiki text** — `.wiki` (p, headings, lists, `pre`, `code`, `blockquote`,
`.panel`, `table.wiki-table`, `a.issue-key`, `a.mention`).

**Editor** (descriptions, comments, new issue; base styles in `app.css`) —
`.editor` (the frame; `:focus-within` while typing, `.is-source` in source
mode, `.is-loading` for the instant before it opens) > `.editor-toolbar`
(`[role=toolbar]`) > `.editor-group` (separated by a left rule; `.editor-end`
pushed right; `.editor-context` = a full-width second row for table or code
tools, with `.editor-context-label`) > `button.editor-button`
(`[aria-pressed='true']` when the format applies, `[disabled]`;
`.editor-mark[data-mark='strong|em|underline|strike']` hold a letter, the
others an SVG drawn with `stroke: currentColor`; `.editor-source-toggle`),
`button.dropdown.select.editor-style`, `input.input.editor-language`. Then the
editable area `.editor-content`, **which is also a `.wiki`**: your `.wiki`
rules style it, so text looks the same while edited. Inside it:
`a.editor-mention` (also `.mention`), `a.issue-key` (no `href`),
`.editor-raw` (markup kept as written — `span` inline, `div` for blocks,
`[data-kind]`; a dashed outline by default), `p.is-empty::before`
(placeholder), `.selectedCell`, `.ProseMirror-selectednode`. Around it:
`textarea.textarea.editor-source` (source mode), `.popover.editor-menu`
(people for "@", `.item`), `.popover.editor-link` (link form, `.field`s),
`.editor-status` (upload in progress), `.editor-hint` (shortcut hints).

**Boards** (`/boards/<id>`, `/boards/<id>/backlog`; base styles in `app.css`) —
`.sprint-head` (`.sprint-name`, `.sprint-dates`, `.sprint-meta` = issue
count and estimate total, `.sprint-actions`, `.sprint-goal` on its own line)
heads the board's active sprint and each backlog section.
`.board` > `section.board-column` (`.is-over` while a card is dragged over it,
`.over-limit` past its maximum, `.under-limit` under its minimum) >
`header.board-column-head` (`h2.board-column-title`, `.board-column-count`,
`.wip-limit`) + `.board-cards` (one per swimlane) > `article.board-card`
(`.is-dragging`) with `.type-mark`, `.key`, `.chip` (epic, estimate),
`.stamp` (when a column holds several statuses), `.priority-mark`, `.avatar`
and the menu button `.card-menu-toggle` (`.btn.btn-quiet.btn-small`; its
`.popover.card-menu` opens in the top layer). `.drop-marker` is the line
showing where a dragged card lands (`.before` / `.after` a card, or alone at
the top of an empty list). The board is a grid: columns span every row
(`grid-template-rows: subgrid`), and with swimlanes a `.board-lane` band
crosses them above each lane's cells (`button.board-lane-toggle` with
`[aria-expanded]`, `.board-lane-title`, `.board-lane-count`). Wider than the
page, the board scrolls sideways: `.more-left` / `.more-right` while columns
are out of sight on that side, each shown by a shade on that edge, the
board's `::before` / `::after` (drawn with `--ink`). The backlog
reuses the hooks: each section is a `.sprint-head` then a
`.board-column.backlog-list` > `.board-cards` of `article.board-card.backlog-row`
(one line each).

**Setup wizard** (`/setup`, before any account exists; base styles in
`src/ui/setup/setup.css`) — `.setup` > `main.card.setup-sheet` >
`.setup-inner` (the padding) holding `header.setup-head` (`.brand` with
`.logo` + `.wordmark`, `.eyebrow`), `nav.setup-steps` > `ol` > `li`
(`.is-done`, `.is-current` with `[aria-current='step']`; `.num`, `.label`),
`p.counter`, the step (`h1.page-title`, `p.lead`, `.field`s, `.setup-row`,
`details` with `pre.setup-code`, `p.setup-ok` / `.setup-warn` results,
`.setup-actions` with `.btn`s), and `footer.setup-foot` (the version).
Language and account choices are `label.language` / `label.account` (`.on`
when chosen), the start choices `button.choice`, the import progress a
native `progress` and `ol.phases` > `li.phase` (`.done`, `.current`), its
report a `table.ledger.report`, the password meter `.meter[data-score]` > four
`span` (`.on`). Everything sits on the sheet: nothing is drawn on `--desk`.

**Gallery** (`/themes`) — each preview is
`.skin-preview[data-skin][data-scheme]` (`inert`, `contain: paint`, so a
`position: fixed` overlay on your root stays inside it) holding `.app-shell` >
`.topbar` (brand, three `.nav-link`, search input, create button, avatar),
`.app-main > .page` (`.page-head` with `.eyebrow` + `.page-title`,
`.section-title` + `.count`, a `.ledger` of three rows with every stamp
category, a row of `.input` + `.btn` + `.btn-primary`, a `.card` with
`.card-head`, a two-column `.board`) and `.app-statusbar`. Sticky elements are
static there.

## 4. Checking your theme

- Dev server against the synthetic data (never port 8080, it holds real data):
  `SOLSTICE_BACKEND=http://localhost:8081 pnpm exec vite --host 127.0.0.1 --port <free port> --strictPort`,
  sign in as `marie.durand` / `motdepasse`, choose your theme in the account
  menu, or open `/themes` to see it next to the others (and Carnet inside it).
  Editing your file reloads the page.
- `pnpm exec vitest run tests/core/contrast.test.ts` — token completeness and
  AA for your skin (skipped while your file does not exist).
- `pnpm exec prettier --write src/themes/<id>.css` — format your own file only.
- The gallery shows « Ce thème est encore en cours de dessin » while your
  file is missing, and only a light preview if it has no dark block.
