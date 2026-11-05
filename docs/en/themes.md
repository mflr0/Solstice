# Themes

A theme redraws the whole interface with CSS, fonts and images. This guide is
for writing one outside the Solstice repository. Related pages:
[administration](administration.md), [backups](backups.md).

The reference for what a theme can style is
[ANATOMY.md](../../backend/internal/themes/ANATOMY.md): the tokens, every class
name, and the rules in detail. This page gets you started and tells you how to
check and share a theme.

## Your first theme

This takes an existing theme, changes its accent colour and tries it in your
browser. You only need an account, not administrator rights.

1. **Export Carnet.** Sign in, then open `/api/themes/carnet/export` on your
   instance. Your browser downloads `carnet-1.0.0.zip`. Administrators also have
   an **Export** button on **Administration → Themes**. Any theme on offer works
   the same way: replace `carnet` with its id.
2. **Unzip it** into a folder named `my-theme`.
3. **Give it its own id.** In `theme.json`, set `id` to `my-theme`, and change
   `name`, `description`, `about` and `author`. In `theme.css`, replace every
   `[data-skin='carnet']` with `[data-skin='my-theme']`.
4. **Change three tokens.** In the first `[data-skin='my-theme']` block of
   `theme.css` (the one with no `data-scheme`), which holds the tokens:

   ```css
   --accent: #1f5fbf;
   --accent-text: #1a56ad;
   --sun: #1f5fbf;
   ```

   In `theme.json`, set `preview.light.accent` to `#1f5fbf`: it must equal
   `--accent`.
5. **Zip the contents of the folder**, not the folder: `theme.json` and
   `theme.css` must be at the root of the zip.

   ```sh
   cd my-theme
   zip -r ../my-theme.zip . -x '*.DS_Store'
   ```

6. **Import it.** Open the account menu, then **My settings → My themes**
   (`/settings/themes`). Choose **Import a theme**, pick the zip, and confirm
   that you trust the file.
7. **Use it.** The theme appears in the list with a light and a dark preview.
   Choose **Use**. Buttons, links and the logo turn blue.

To change it again, edit the files, raise `version` in `theme.json`, zip and
import again. The same id asks whether to replace the version you have, and the
theme stays selected.

## Starting from nothing

Exporting a theme also gives you all its rules to unpick. To write one from
nothing, choose **Download the starter package** on the gallery (`/themes`), which
is `/api/themes/starter`. The zip holds:

- `theme.json`, filled with placeholders.
- `theme.css`, with every token in commented groups and Carnet's values, a dark
  block, an example `@font-face` and asset, and an empty commented heading per
  section of the [hooks](../../backend/internal/themes/ANATOMY.md#3-hooks).
- `fonts/`, with that font and its licence, and `assets/`, with that image.
- `ANATOMY.md`, the reference the comments of `theme.css` send you to. It can stay
  in the zip: an instance keeps it but never serves it.

It passes [the checks](#the-checks) as it is. Replace `my-theme` in both files,
change the tokens, and keep `preview` in `theme.json` equal to them. Then zip and
import it as in [your first theme](#your-first-theme).

## The package

A package is a zip with these files at its root:

| File | Holds |
|---|---|
| `theme.json` | The manifest, below. Required. |
| `theme.css` | The stylesheet: fonts, then the tokens, then your rules. Required. |
| `assets/` | Images: `.svg`, `.png`, `.webp`, `.jpg`, and their licence as `.txt`. |
| `fonts/` | Fonts as `.woff2`, and their licence as `.txt`. |
| `LICENSE`, `LICENCE` or `COPYING` | The licence of the theme, optional, with `.txt`, `.md` or no extension. |
| `ANATOMY.md` | The reference that comes with the starter package, optional. |

A package holds at most 100 files and 10 MiB once unpacked. An instance refuses
any other file, so Finder's `__MACOSX` folder and `.DS_Store` files count.

The manifest:

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

| Field | Rule |
|---|---|
| `id` | 1 to 32 of `a-z`, `0-9` and `-`. The selectors of `theme.css` use it. |
| `contract` | The version of the contract the theme was written for. It is 1 today. |
| `version` | A [semantic version](https://semver.org), such as `1.0.0`. |
| `name`, `description` | Locale → text, `en` required. A missing locale shows the English text. |
| `about` | Optional, like `name`. One sentence for the theme gallery. |
| `author` | Not empty. |
| `schemes` | `["light"]`, or `["light", "dark"]` when `theme.css` has a dark block. |
| `preview` | `bar`, `paper`, `ink` and `accent` of each scheme as `#rrggbb`, equal to the tokens of those names. |

No other field is allowed.

## The contract

`theme.css` builds on the same structure as every other theme:

- **Tokens.** One block with every token Carnet declares, and an optional dark
  block with the colours that change. See
  [Tokens](../../backend/internal/themes/ANATOMY.md#2-tokens).
- **Hooks.** The class names you can style, page by page. See
  [Hooks](../../backend/internal/themes/ANATOMY.md#3-hooks).
- **Anchor.** Every selector starts with `[data-skin='<id>']`, or
  `[data-skin='<id>'][data-scheme='dark']`. Never `:root`, `html` or `body`.

The `contract` field says which version of these tokens and hooks the theme was
written for. An instance refuses a package with a higher `contract` than its
own. It still takes a lower one, and the interface says the theme may look
broken. The
[contract changes](../../backend/internal/themes/ANATOMY.md#6-contract-changes)
list what each version changed.

## The rules

The checks enforce most of these. Follow them even if you only use the theme
yourself: My themes checks nothing.

- **No JavaScript.** A theme is CSS, images and fonts. A `.js` file is refused,
  and an SVG must not hold a script.
- **No `@import`.** Declare fonts with `@font-face` at the top of `theme.css`.
- **`url()` points into the package**: `assets/…` or `fonts/…`, written exactly
  so, with no `./` or `..`, or a `data:` URL. No web addresses.
- **Free font licences only.** Each font needs a licence file next to it:
  `<family>-OFL.txt` for `<family>-….woff2`, or a `.txt` with no hyphen in its
  name for the whole folder. The checks only see that the file exists: choosing a
  licence that allows this is up to you.
- **No third-party brands or logos**: no other company's names, logos, icons,
  fonts or assets. No check can see this either: it is up to you.
- **Readable colours.** Text needs a contrast of 4.5:1, focus rings and accents
  3:1, in every scheme.
- **Font sizes in `rem`**, not `px`.
- **No CSS escapes** (`\`) outside strings.

## The checks

Importing as an administrator runs the checks. There is no other way to run them
on your own package, and My themes doesn't run them: a theme that works there may
still be refused. Without administrator rights, ask an administrator, or use an
instance of your own ([installation](install.md)).

A refused package stores nothing, and the page lists what is wrong. The API
answers the same list. Each finding has three parts:

```text
theme.css:12   url        url(./assets/moon.svg): want a file under assets/ or fonts/, or data:
```

- **Where**: the file, and the line when there is one.
- **The rule**: the table says what to do about each.
- **The message**: what was found.

| Rule | Means | Fix |
|---|---|---|
| `zip` | The zip itself: a file that isn't allowed, a folder other than `assets` or `fonts`, too many files, too big. | Zip the contents of the folder, with only [the files above](#the-package). |
| `manifest` | `theme.json` is missing, isn't valid or has an unknown field. | See [the fields](#the-package). |
| `css` | `theme.css` is missing or can't be read. | Put it at the root of the zip. |
| `size` | Over 100 files or 10 MiB. | Remove files. |
| `tokens` | A token is missing, or no block matches the `id`. | Declare every token. Check `[data-skin='<id>']` against the `id`. |
| `contrast` | Two colours are too close, in the scheme the message names. | Change one of them. |
| `schemes` | `schemes` doesn't match the presence of a dark block. | List `dark` exactly when there is a dark block. |
| `preview` | A preview colour differs from its token. | Copy the token's value to `theme.json`. |
| `font-size` | A font size in `px`. | Use `rem`, at 16 px to the rem. |
| `import` | An `@import`. | Remove it. Declare fonts with `@font-face`. |
| `escape` | A `\` outside a string. | Write the character, or move it into a string. |
| `url` | A `url()` leaves the package, or its file isn't there. | Point to an existing file under `assets/` or `fonts/`. |
| `load` | `var()` or another call inside `image-set()`, `image()` or `src()`. | Use strings or `url()` there. |
| `licence` | A font has no licence file next to it. | Add one, as in [the rules](#the-rules). |

## Trying a theme

- **My themes** (`/settings/themes`) keeps a theme for you, with previews.
- **The gallery** (`/themes`) shows it next to the others, under **My themes**.
- **`?skin=<id>`** in the address forces a theme for this page load, and doesn't
  change your choice. A theme of My themes has the id `local-<id>`:
  `?skin=local-my-theme`.

**Safe mode.** If a theme breaks the interface, add `?skin=carnet` to the address:
the page loads with Carnet. To delete the theme, open
`/settings/themes?skin=carnet`.

When a theme is missing, fails to load or takes more than 5 seconds, Solstice
shows Carnet for this page load, with a message. Your choice stays, so the theme
comes back once it works.

## Keep your zip

A theme imported on My themes stays in that browser, and goes to no server:

- Other people don't see it, and neither do your other browsers and devices.
- Anyone using the same browser profile has it, whichever account they sign in
  with.
- Clearing the site's data deletes it.

So keep the zip. **Export** on My themes gives back the zip you imported, as long
as the theme is still there.

## Importing a theme for everyone

An administrator can offer a theme to everyone on the instance:

1. Open **Administration → Themes** (`/admin/themes`).
2. Choose **Import a theme** and pick the zip. If the checks find anything, the
   page lists the findings and nothing is stored.
3. The theme is enabled at once, and appears in every user's theme menu.

If the id is taken, the theme is served under a suffixed one (`retro-2`). The
files don't change. To update a theme, choose **Replace** on its row and give a
new version with the same `id`.

For the limits, turning themes off and deleting them, see
[administration](administration.md#themes). The API is described in
`backend/internal/api/ADMIN_API.md`, under "Themes".
