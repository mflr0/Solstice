# Theme anatomy

How a theme ("skin") plugs into Solstice. `frontend/src/app.css` is the
structure every skin builds on, drawn from the tokens. Each skin, Carnet the
default included, is a package (section 5): a folder
`backend/internal/themes/builtin/<id>/` holding `theme.json`, `theme.css` and
its fonts and images.

This page is the reference, and section 4 is for this repo's own themes. To write
a theme outside the repo, start from the guide: [docs/en/themes.md](../../../docs/en/themes.md).

Ids, in the order the menus list them (`builtin/order.json`): `carnet`,
`orbite`, `crayon`, `luna`, `retro`, `classique`, `terminal`, `plan`.

## 1. Writing the file

- **Anchor every selector on the skin attribute**: `[data-skin='<id>'] …`, or
  `[data-skin='<id>'][data-scheme='dark'] …` for the dark variant. Never `:root`,
  `html` or `body` as the anchor: the skin root is either `<html>` or a
  `<div class="skin-preview">` in the theme gallery.
- **Fonts**: woff2 files in `fonts/` with the font's licence file next
  to them (`<family>-OFL.txt` for `<family>-….woff2`, or a `.txt` with no
  hyphen in its name for the whole folder), declared by `@font-face` rules at the top of the file with
  `src: url('fonts/x.woff2') format('woff2')`. No `@import`: the
  validator refuses one. Free licences only.
  System fonts (Tahoma, Verdana, …) only as `font-family` fallbacks, never
  bundled. No Microsoft or Atlassian names, logos, icons, fonts or assets,
  anywhere (code, comments, UI).
- **Images**: `url('assets/thing.svg')` — relative to `theme.css`; the loader points them at `/api/themes/<id>/`.
- **CSS escapes** (`\`) only inside strings: the validator refuses them anywhere else, selectors included.
- **`image-set()`, `image()`, `src()`**: the validator holds their strings to the `url()` rule, and refuses `var()` or any other call in them but `url()` and `type()`.
- **Motion**: wrap animations in `@media (prefers-reduced-motion: no-preference)`.
- **SVG filters** (roughness…) on frames only, never on text: draw the frame on a
  pseudo-element and filter that.
- **Sizes in rem**: `--font-size`, every font size and the layout sizes
  (`--bar-height`, page and column widths, control heights) in rem, at 16 px
  to the rem (14px → `0.875rem`), so they follow the browser's font size
  and the wide-screen steps of app.css (1rem grows by 1/16 at 1600, 1920 and
  2560 px). No px font size but `0`: the validator refuses one. Paddings, gaps, borders, radii, shadows and `@media` widths stay px.
- No `!important` unless a Svelte inline `style=` forces it.

### How the file is loaded (you do not need to do anything)

`src/lib/skins.svelte.ts` fetches the file from `GET /api/themes/<id>/theme.css`
(`public/theme-boot.js` starts this for the stored skin, before the application's
scripts). It rewrites the anchor to the id the server serves the package under and
package-relative `url()`s to `/api/themes/<id>/…`, then injects the file as
`<style data-skin-css="<id>">` after every other stylesheet, so
**your rules win specificity ties**. When the browser supports `@scope`, the
loader wraps the file in `@scope ([data-skin='<id>']) to ([data-skin])` and
rewrites `[data-skin='<id>']` to `:scope`: your rules then stop at any nested
element carrying another `data-skin` (gallery previews), and specificity is
unchanged. `@font-face`, `@keyframes`, `@import`, `@property`,
`@counter-style` stay top-level. While the file loads, `<html>` carries
`data-skin-loading` and the page is hidden.

Carnet loads the same way. It is the default, and the fallback for this page
load when the theme isn't in `GET /api/themes`, when its file fails to load, or
when it isn't in within 5 s: a toast says so, and the stored choice stays, so
the theme comes back once it works.

**Escape hatch**: `?skin=<id>` in the address forces that theme for this page
load and doesn't change the stored choice. If a theme breaks the interface,
`?skin=carnet` gets the page back.

## 2. Tokens

Two token blocks, written **exactly** like this (the validator parses them):

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

The first block must declare **every** token Carnet declares (the validator checks
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
| Statuses    | `--stamp-todo`, `--stamp-doing`, `--stamp-done` + shape tokens `--stamp-font`, `--stamp-case`, `--stamp-tracking`, `--stamp-dot`, `--stamp-large-size`, `--stamp-large-tracking`, `--stamp-large-turn`                                                                                                                                           |
| Issue types | `--tone-bug`, `--tone-story`, `--tone-epic`, `--tone-task`, `--tone-subtask`                                                                                                                                                                                                                                                                     |
| Depth       | `--shadow-pop`, `--backdrop`, `--ink-blend`                                                                                                                                                                                                                                                                                                      |
| Type        | `--font-title`, `--font-body`, `--font-mono`, `--font-size`, `--title-weight`, `--h1-weight`, `--title-tracking`, `--title-italic`, `--empty-font`                                                                                                                                                                                               |
| Shapes      | `--frame` (border width), `--radius-xs`, `--radius-s`, `--radius-m`, `--radius-l`, `--avatar-radius`, `--card-top`                                                                                                                                                                                                                               |
| Background  | `--page-texture`, `--page-texture-size`, `--loader` (`none` or `inline-block`: the base orbit loader)                                                                                                                                                                                                                                            |

New since the Orbite/Crayonné blocks were copied out of `app.css`: `--desk`,
`--accent-text`, `--focus`, `--bar`, `--bar-ink`, `--bar-ink-soft`,
`--font-size`. `--accent-ink` is gone (use `--primary-ink`).

### Contrast (`validate.go`, every skin × scheme, no exceptions)

Text, 4.5:1 — `--ink`, `--ink-soft`, `--muted` on `--paper`, `--paper-raised`,
`--paper-sunk`; `--accent-text` on `--paper`, `--paper-raised`; each stamp ink
on `--paper`, `--paper-raised`; tones on
`--paper-raised`; `--paper` on `--tone-epic`; `--primary-ink` on `--primary`;
`--bar-ink`, `--bar-ink-soft` on `--bar`.
Non-text, 3:1 — `--focus` and `--accent` on `--paper`.

The focus ring is at least 2px thick, in `--focus`, in any style (solid, dotted,
dashed). `outline: none` in a `:focus` or `:focus-visible` rule needs a 2px
`box-shadow: 0 0 0 2px …` in the same rule. `frontend/tests/core/focus-ring.test.ts`
checks it.

Any other pair your file draws (title bars, inverted rows…): declare it in a
comment anywhere in the file and the validator checks it too, in every scheme:

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
`.theme-picker` > `button.topbar-themes` (a palette: stroked `path`, filled
`circle` dots; `[aria-current='page']` on
the gallery, `[aria-expanded='true']` while open: give both the same look),
`.popover.theme-menu` (`button.item[role=menuitemcheckbox]`, the dark-mode
switch, and a `.sep`, then `.theme-head` > `.scheme-name`s, then a `.theme-row`
per skin > `.skin-name` + two `button.scheme[role=menuitemradio]`, light and
dark, each a thumbnail drawn from `--sw-*`, not an `.item`; then `a.item` to
the gallery), `.btn.btn-primary.topbar-create` >
`.plus` (hidden on wide screens) + `.label` + `kbd`, `.account` >
`button.account-button` > `.avatar`, `.popover.account-menu`. The nav has
four links (and may gain more): do not count on `:nth-child`. Short of room,
`.topbar-search` shrinks first; when the nav still does not fit beside the
tools, TopBar sets `.is-stacked` on `.topbar` and the nav takes a full-width
second row (`order: 1`, links grow to share it, centred), at any width. The
first row keeps `--bar-height`: set it, not `height`, on `.topbar-inner`, and
space its items with `column-gap`, not `gap`. Style the stacked row under
`.topbar.is-stacked`. Last resort, the nav scrolls sideways inside the bar
(`overflow-x: auto` at every width, links never wrap: a `.nav-link`
decoration drawn outside the nav box is clipped; the current link is
scrolled into view). Narrow screens, in the base: ≤ 860 px the search field
and the `kbd` go and `.live-dot` takes `margin-left: auto`; ≤ 600 px `.wordmark` and `.topbar-create .label` are
visually hidden (still read) and `.plus` shows, sized `1.55em` — a skin that
sets the button's `font-size: 0` to draw its own « + » hides it too; ≤ 420 px
`.theme-picker` goes (the gallery stays on Account → Appearance).

**Logo** — `svg.logo` > `.logo-sun`, `.logo-line`, `.logo-faint` (Carnet).
To use your own mark: `[data-skin='x'] .logo > * { display: none }` and paint
`.logo` with `background: url('assets/logo.svg') center / contain no-repeat`.
For a mark that follows `--sun` or `currentColor` in light and dark, mask it
instead: `mask: url('assets/logo.svg') center / contain no-repeat` and give the
colours as `background` layers (Plan, Orbite, Crayonné). A mask clips
`filter`, so draw a glow as a gradient.

**Page** — `.page` (the sheet; `[data-title]` holds the page's title, for a
window title bar: `content: attr(data-title)`), `header.page-head` (wraps
under 600 px), `.eyebrow`, `h1.page-title`, `h2.section-title` (`::after` is
the divider, drawn with `--rule-stroke`; `.count`, `.actions`).

Issue page: `.page.issue` > `nav.page-crumbs`, `header.issue-head`
(`h1.page-title`, `.issue-meta`), `.issue-layout` (`.issue-sheet`, then
`aside.card` = the record).
`.page.issue` is a container named `issue` (it can sit in a modal), so restyle
the issue view by its width with `@container issue (max-width: …)`, not
`@media`. With the default page padding its content is 916 px wide at a
980 px screen, 728 px at 760 px, 568 px at 600 px; measure it in a skin that
changes the page's padding.

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
`h1.page-title` and `p.meta.person-meta`. Project settings: `.intro`,
`.savebar` (below).

**Panels / windows** — `.card` with `.card-head` as its title bar (the issue
record). The sign-in sheet `.card.login-card` has no `.card-head`: its title
area is `.login-card > .brand` (logo + `.wordmark`), then `.tagline`, the
form, `.note`. Wiki `{panel}`: `.wiki .panel`, `.wiki .panel-title`.

**Buttons** — `.btn` (secondary), `.btn-primary`, `.btn-quiet` (ghost),
`.btn-danger`, `.btn-small`; pressed: `:active`, `.on` / `[aria-pressed='true']`;
`[disabled]`. `.btn.btn-icon` (a square on the button's height, an `svg` in
`currentColor` and no text; a `::before` or `::after` in the flow crowds it).
`.link-btn` (text button). `kbd` inside buttons.

**Fields** — `.field` > `.field-label` (or first `span`), `.input`, `.select` (a dropdown's button),
`.textarea`, `input.checkbox`, `.hint`; `.boxed` (`label.field-box` in the
create dialog) frames the label editor like an input in forms. A person picker
is `.picker` with `button.current` and `.popover` (`input.filter`, then
`ul[role=listbox]` > `li` > `button.item`); in a form it is `.framed` and its
`button.current` is also a `.select`. A multi-value one is `.multi` (`.framed`
in a form, the same way) with `button.current` and
`.popover` (`input.filter`, then `[role=menu]` >
`button.item[role=menuitemcheckbox]`). Their `.popover` opens in the top
layer, like a dropdown's list. Every list that offers people (these
pickers, the search filter's people, the editor's « @ » list) draws a person
as these children of its `.item` (`UserOption`): `.avatar`, `span.name`
(`.inactive` once deactivated: struck out), `span.user.muted` (the username,
then « deactivated » when so). `.chip` (labels, values). `.key` (issue keys,
mono).

**Tables** — `table.ledger` > `thead th`, `tbody tr`, `td`; `.num` (right-aligned
numbers). Rows are clickable (`cursor: pointer`).
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
(`[aria-selected='true']`).

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
A searchable dropdown's list is `div.popover.dropdown-list.searchable` >
`input.input.filter[role=combobox]` (it keeps the focus; the options scroll
under it), `ul[role=listbox]` (the options as above, the matched letters in
`mark.match`), then `p.no-match` (« No option matches. », empty while some
do).
A list in the top layer is fixed to the window and placed by script (under
its button, or above it when there is more room there): don't give it a
`position`, `inset` or `margin`; its `max-height` still holds.

**Dialogs** — `dialog.modal` (`::backdrop`) > `header.modal-head` (title bar) >
`h2.modal-title` + `button.modal-close` (label « Fermer Échap »; restyle it,
e.g. as a ✕ box with `font-size: 0` and a `::before`), `.modal-body`,
`footer.modal-foot`. Some dialogs put buttons in the title bar, in
`.modal-actions` before the Close button.
`dialog.modal.issue-modal` shows an issue over a board or a backlog: its title
is the issue key as a link (`h2.modal-title > a.key`), its `.modal-body` has
no padding and holds the issue view (`.page.issue`, without the page's
margin, border and shadow, with the skin's page padding). It is about as wide
as the issue page and fills the screen under 760 px.

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
`title` only, nothing to style, but for relative dates.

**Relative dates** — `time.reltime` ("3 hours ago", a dotted underline). On
hover, `.reltime-tip` (a `popover` in the top layer, `position: fixed`,
placed by script above the date: style it, don't position it) >
`.reltime-day` (the full day) + `.reltime-time` (the time and the time zone).
Base: `--ink` ground, `--paper` text.

**Marks** — `span.stamp` (`.todo` / `.doing` / `.done`, `[data-category]`,
`.large` on the issue page; `::before` is the optional dot, `--stamp-dot`). A
status is its name in its ink: draw no border, background, shadow or frame
around it, and no horizontal padding, so it lines up with the text around it.
`.type-mark[data-type='bug|story|epic|task|subtask']` (`data-type` is the
family guessed from the name). An admin may style a type: its colour comes in
an inline `--type-color` (unset without one), its fill in
`data-fill='filled|subtle|outline'`, and its icon as an `svg` child in place
of the letter. The mark is a bare glyph in a 1.125rem square: draw no border,
background, tile or `::before` shape behind it. Fills apply to the glyph (an
icon's `path.detail` is drawn in `--paper` when filled, a `path.line` is never
filled, a filled letter is weight 800), so change only its font, and its colour
through `--mark` (`--mark-solid` when filled, `--mark-on-tint` when subtle). To
keep the type's colour, clamp its lightness into a variable that is unset
without one, as the bundled skins do with `--type-ink`, and keep 3:1 against
`--paper` and `--paper-raised`. `.priority-mark` >
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
  criterion in use (a chip): `button.btn.btn-small.filter-button`
  (`[aria-expanded]`; `.is-active` when it holds values; inside: `.filter-name`,
  `.filter-value` = the chosen values, `.caret`), then
  `button.btn.btn-small.filter-clear` (the ×, only when active), then
  `.popover.filter-popover[role=dialog]` (`.wide` for dates). Then « Filtre »:
  `.add-filter` > `.filter-wrap.pinned` with the same button (`.lead` = the +)
  and a `.popover.filter-popover.wide` > `.add-panel` > `.add-fields`
  (`input.input.field-search`, `ul.field-list[role=listbox]` > `li.group-label`
  and `li.item.field-option[role=option]` (`[aria-selected='true']` for the
  field shown, `.active` under the keyboard, `.in-use` when it holds values),
  `li.none`; then `ul.more-list > li > label.item`: « Tickets que j’observe »,
  a checkbox, `.busy` when disabled, with `p.busy-note`) + `.add-values` (the
  field's picker, as in a chip, or `p.pick-field`). `.link-btn.clear-all`.
  « Personnes » opens with `.role` > `.role-field` (`button.dropdown.select`:
  the role) + `p.role-hint` above its list.
  The base sets only `background-color` on these buttons, so a skin can paint
  `background-image` fills (Plan does).
- Inside a popover: `.value-picker` > `input.input.filter-search` +
  `ul.filter-options[role=listbox]` > `li.group-label` (« Raccourcis », status
  categories) and `li.item.filter-option[role=option]` (`[aria-selected='true']`
  when ticked, `.active` under the keyboard; `.box` = the tick box, `.mark` (Type and people pickers: a `.type-mark` or an
  `.avatar`, empty on « Non assigné »), `.label`,
  `.hint`; a person instead has the person parts of Fields, then `.count` = issues it would show, `.zero` when none; counts come
  in a moment after the list), then `.picker-foot` (« Effacer »). Dates: `.date-picker` >
  `fieldset.date-presets` > `label.item.date-preset` (`.active`), `.custom` >
  two `input.input[type=date]`.
- ≤ 600 px a popover is a sheet along the bottom of the screen
  (`position: fixed`), over `.filter-veil` (painted with `--backdrop`), with
  `.sheet-head` (name + close button) shown; the bar wraps.
- At the end of the bar: `.filter-tools` > `.column-picker` (« Colonnes »),
  `button.btn.btn-small.btn-quiet.mode-toggle` (« JQL », or « Filtres » in the
  editor), then the ⋯ menu as on a card: `.card-menu-toggle` and its
  `.popover.card-menu` (« Exporter en CSV »). In JQL mode the same buttons sit
  in `.jql-tools`, above the editor.
- Under the bar: `p.search-summary` (`strong` = the count, then the order in
  words); `p.refused` (« Requête trop complexe pour les filtres »); `.results`
  (the `.ledger`).
- JQL mode: `.jql-editor` (`.has-error`) > `.jql-field` > `pre.jql-highlight`
  (the coloured copy, `aria-hidden`) under a transparent
  `textarea.jql-input`: both draw the same box, so give them the same font,
  padding, line-height and border width (style them together, or leave
  them). Tokens: `span.jql-token-field`, `.jql-token-operator`,
  `.jql-token-keyword`, `.jql-token-value`, `.jql-token-string`,
  `.jql-token-function`, `.jql-token-punct`, `.jql-token-error` (base inks:
  `--stamp-doing`,
  `--muted`, `--tone-epic`, `--ink`, `--stamp-done`, `--tone-task`,
  `--accent-text` with a wavy underline). Suggestions:
  `ul.popover.jql-suggestions[role=listbox]` > `li.item` (`.active`, `.label`,
  `.detail`). `.jql-foot` (`.hint`, the run button), `p.error-text.jql-error`.
- View page (`/views/:id`, `/views/new`): `.view-page` > `.view-head` >
  `a.back`, `h1.view-title` (holding `input.name-input` when the view can be
  renamed), `.unsaved` once something differs from the saved view, and
  `.actions` (`.link-btn.discard`, `.btn`s); then `.issue-search`, or
  `p.empty-state` when the view doesn't exist or isn't shared.
- Quick search (top bar): `form.topbar-search.quick-search` >
  `input.input#quick-search[role=combobox]`, `kbd`,
  `ul.popover.quick-search-results[role=listbox]` > `li.group-label`, `li.sep`,
  `li[role=option]` > `a.item` (`.quick-issue`, `.quick-project`,
  `.quick-person`, `.quick-all`; `.active` under the keyboard) holding
  `.type-mark`, `.key`, `.summary`, `.stamp`, `.who`, `.hint`. Each person is
  followed by three small options on one line, `li.sub` (`.first` on the
  first) > `a.item.quick-assigned`, `.quick-reported`, `.quick-profile`.
- Matched letters: `mark.match`, inside the quick search's project keys and
  names, people's names and `.who`. The base draws it bold in `--accent-text` (in
  the row's own ink on a hovered or `.active` popover `.item`, bold only), so
  it reads on any row a skin paints. Override
  it only if that looks wrong in your skin; a new colour pair needs its
  `/* contrast: … */` line. Don't change its padding or line-height: the row
  must keep its height.

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
(people for "@", `.item` with the person parts of Fields), `.popover.editor-link` (link form, `.field`s),
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
`[aria-expanded]`, `.board-lane-title`, `.board-lane-count`); the bands come
before the columns in the markup. A card dragged by touch is copied into a
`.board-card.touch-ghost` on `body` that follows the finger. Wider than the
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

**Administration overview** (`/admin`; base styles in
`AdminOverview.svelte`) — `.groups` > `section.group` (`h2.section-title`, then
`ul` > `li`) per group of sections. Each section is one link,
`a.admin-card` (the whole card; `--paper-raised`, `--paper-sunk` on hover)

> `svg.admin-icon` (16 × 16 strokes in `currentColor`, glyphs from
> `src/ui/admin/admin-icons.ts`) + `.admin-card-title` + `.admin-card-text`.
> Its hover moves it up 1 px, except under `prefers-reduced-motion: reduce`;
> a skin that draws its own card sets `transform: none` there (Rétro 98 bevels it,
> Terminal turns it inverse, Plan fills it with ink).
> The « to do » cards above are `ul.todo-list` > `li.card`.

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
- `go test ./internal/themes -run TestValidateBuiltin` (from `backend/`) — runs
  the validator on every built-in package: manifest, token completeness, AA,
  font sizes, escapes, file targets, licences, `schemes` and `preview`. Each finding
  gives the rule, the file and the line.
- `pnpm exec prettier --write ../backend/internal/themes/builtin/<id>/theme.css` —
  format your own file only.
- The gallery shows « Ce thème est encore en cours de dessin » while your
  file is missing, and only a light preview if it has no dark block.

## 5. Package

The server serves theme packages built into its binary, from
`backend/internal/themes/builtin/<id>/`; `builtin/order.json` lists their ids in
the order the menus show them. **Contract version: 1.** A package
declaring a higher `contract` is refused. One declaring a lower `contract` can
still be used; the interface says it may look broken.

A package is a folder named after its id:

- `theme.json`, the manifest (below);
- `theme.css`, the file of section 1;
- `assets/…` (images) and `fonts/…` (fonts and their licence), optional.

`GET /api/themes/starter` serves a valid package to start from, generated from
Carnet by `starter.go`, with a copy of this page: a token Carnet adds or a section
added under Hooks above fails its tests until it joins the lists there.

Allowed file types: `.css`, `.svg`, `.png`, `.jpg`, `.jpeg`, `.webp`,
`.gif`, `.woff2`, `.woff`, `.ttf`, `.otf`, `.txt`. Any other file under
`assets/` or `fonts/` makes the package invalid, and the server does not start.
A package holds at most 100 files and 10 MiB (`MaxPackageFiles`,
`MaxPackageBytes`).

```json
{
  "id": "night-owl",
  "contract": 1,
  "version": "1.0.0",
  "name": { "en": "Night owl", "fr": "Chouette" },
  "description": { "en": "Dark and calm." },
  "about": { "en": "A night sky: deep blue, a pale moon, an orange accent." },
  "author": "Someone",
  "schemes": ["light", "dark"],
  "preview": {
    "light": { "bar": "#112233", "paper": "#ffffff", "ink": "#000000", "accent": "#abcdef" },
    "dark": { "bar": "#000000", "paper": "#111111", "ink": "#eeeeee", "accent": "#ff8800" }
  }
}
```

- `id`: 1 to 32 of `a-z`, `0-9` and `-`, the folder's name.
- `contract`: an integer, required, 0 up to the server's contract version.
- `version`: a [semantic version](https://semver.org) (`1.0.0`, `2.1.0-beta.1`).
- `name`, `description`: locale → text, `en` required, no empty text. The menus
  show the name and, under it, the description: a few words on the look.
  A missing locale shows the English text.
- `about`: optional, locale → text like `name`, `en` required when present.
  The theme gallery (`/themes`) shows it under the name: a sentence on the
  theme. Without it, the gallery shows the description.
- `author`: not empty.
- `schemes`: `["light"]` or `["light", "dark"]`, the latter exactly when
  `theme.css` has a dark block.
- `preview`: for each scheme, and only those, `bar`, `paper`, `ink` and
  `accent` as `#rrggbb`, equal to the tokens of that name in that scheme (for
  a gradient, one of its stops). The theme menu draws its swatches with them,
  and Account → Appearance with the light ones.
- No other field.

`GET /api/themes` (no sign-in needed) answers `{"contract": 1, "themes": [manifest…]}`:
the built-in themes in the order of `order.json`, then the enabled themes stored on the
instance (`<data>/themes/<id>`) by English name, each with the id it is served under. `GET /api/themes/<id>/<path>` serves `theme.css`, `assets/…` and
`fonts/…`, with an ETag, and under `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'`
so an SVG opened on its own runs no script. `theme.json` and other files are not served.

`GET /api/themes/<id>/export`, for a signed-in user, answers the package as a zip an
import takes, under its manifest's id. Only an administrator can export a disabled theme.

An administrator imports a package as a zip (`POST /api/admin/themes`, see
`backend/internal/api/ADMIN_API.md`). `theme.json` and `theme.css` sit at the
root of the zip, not in a folder. An import takes less than a built-in package
may hold: `assets/*.svg|png|webp|jpg`, `fonts/*.woff2`, and licence files (a
`.txt` in `assets/` or `fonts/`, or `LICENSE`, `LICENCE` or `COPYING` at the root,
with `.txt` or `.md` or no extension). The validator must find nothing. If its id
is taken the theme is served under a suffixed one (`retro-2`), and the loader
rewrites the anchor: the files don't change.

## 6. Contract changes

A change that removes or renames a hook bumps `Contract` (`themes.go`) and adds
an entry here. Adding a hook does neither.

- **1**: the hooks as of #526. `tr.is-selected` and `.tabs.tabs-folder` were
  removed without a bump: nothing ever set them, so no theme could rely on them.
  `.search-tools` (with its `.count`, `.sort`, `.sort-dir`) and `.mode-switch`
  went the same way in #812: no release had shipped them yet.
  The search page's saved filters (`.search-page`, `.search-layout`,
  `.saved-filters`, `.search-main`, `.saved-toggle`, `.saved-body`,
  `.saved-head`, `.saved-title`, `.manage`, `.manage-bar`, `.pick-all`,
  `.manage-actions`, `.saved-item`, `.owner`, `.star`, `.modified`) went with
  it in #749, for the same reason.
