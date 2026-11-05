# Thèmes

Un thème redessine toute l'interface avec du CSS, des polices et des images. Ce
guide explique comment en écrire un hors du dépôt de Solstice. Pages liées :
[administration](administration.md), [sauvegardes](backups.md).

La référence sur ce qu'un thème peut styler est
[ANATOMY.md](../../backend/internal/themes/ANATOMY.md) (en anglais) : les jetons,
tous les noms de classes et le détail des règles. Cette page vous met le pied à
l'étrier et explique comment vérifier et partager un thème.

## Votre premier thème

On part d'un thème existant, on change sa couleur d'accent et on l'essaie dans
son navigateur. Il suffit d'un compte, sans droits d'administrateur.

1. **Exportez Carnet.** Connectez-vous, puis ouvrez `/api/themes/carnet/export`
   sur votre instance. Le navigateur télécharge `carnet-1.0.0.zip`. Les
   administrateurs ont aussi un bouton **Exporter** dans **Administration →
   Thèmes**. Cela marche pour tout thème proposé : remplacez `carnet` par son id.
2. **Décompressez-le** dans un dossier nommé `mon-theme`.
3. **Donnez-lui son propre id.** Dans `theme.json`, mettez `id` à `mon-theme`, et
   changez `name`, `description`, `about` et `author`. Dans `theme.css`, remplacez
   chaque `[data-skin='carnet']` par `[data-skin='mon-theme']`.
4. **Changez trois jetons.** Dans le premier bloc `[data-skin='mon-theme']` de
   `theme.css` (celui sans `data-scheme`), qui contient les jetons :

   ```css
   --accent: #1f5fbf;
   --accent-text: #1a56ad;
   --sun: #1f5fbf;
   ```

   Dans `theme.json`, mettez `preview.light.accent` à `#1f5fbf` : il doit être égal
   à `--accent`.
5. **Compressez le contenu du dossier**, pas le dossier : `theme.json` et
   `theme.css` doivent être à la racine du zip.

   ```sh
   cd mon-theme
   zip -r ../mon-theme.zip . -x '*.DS_Store'
   ```

6. **Importez-le.** Ouvrez le menu du compte, puis **Mes réglages → Mes thèmes**
   (`/settings/themes`). Choisissez **Importer un thème**, prenez le zip et
   confirmez que vous faites confiance au fichier.
7. **Utilisez-le.** Le thème apparaît dans la liste, avec un aperçu clair et un
   aperçu sombre. Choisissez **Utiliser**. Les boutons, les liens et le logo
   passent au bleu.

Pour le modifier encore, changez les fichiers, augmentez `version` dans
`theme.json`, recompressez et réimportez. Le même id demande s'il faut remplacer
la version présente, et le thème reste sélectionné.

## Partir de rien

Exporter un thème donne aussi toutes ses règles à démêler. Pour en écrire un en
partant de rien, choisissez **Télécharger le paquet de départ** dans la galerie
(`/themes`), c'est-à-dire `/api/themes/starter`. Le zip contient :

- `theme.json`, rempli de valeurs à remplacer.
- `theme.css`, avec tous les jetons en groupes commentés et les valeurs de Carnet,
  un bloc sombre, un exemple de `@font-face` et d'image, et un titre commenté vide
  par section des
  [points d'accroche](../../backend/internal/themes/ANATOMY.md#3-hooks).
- `fonts/`, avec cette police et sa licence, et `assets/`, avec cette image.
- `ANATOMY.md`, la référence (en anglais) vers laquelle renvoient les commentaires
  de `theme.css`. Il peut rester dans le zip : une instance le garde mais ne le sert
  jamais.

Il passe [les vérifications](#les-vérifications) tel quel. Remplacez `my-theme`
dans les deux fichiers, changez les jetons, et gardez `preview` de `theme.json`
égal à ces jetons. Puis zippez et importez-le comme dans
[votre premier thème](#votre-premier-thème).

## Le paquet

Un paquet est un zip avec ces fichiers à sa racine :

| Fichier | Contient |
|---|---|
| `theme.json` | Le manifeste, ci-dessous. Obligatoire. |
| `theme.css` | La feuille de style : les polices, puis les jetons, puis vos règles. Obligatoire. |
| `assets/` | Les images : `.svg`, `.png`, `.webp`, `.jpg`, et leur licence en `.txt`. |
| `fonts/` | Les polices en `.woff2`, et leur licence en `.txt`. |
| `LICENSE`, `LICENCE` ou `COPYING` | La licence du thème, en option, en `.txt`, en `.md` ou sans extension. |
| `ANATOMY.md` | La référence livrée avec le paquet de départ, en option. |

Un paquet contient au plus 100 fichiers et 10 Mio une fois décompressé. Une
instance refuse tout autre fichier, y compris le dossier `__MACOSX` et les fichiers
`.DS_Store` du Finder.

Le manifeste :

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

| Champ | Règle |
|---|---|
| `id` | De 1 à 32 caractères parmi `a-z`, `0-9` et `-`. Les sélecteurs de `theme.css` l'utilisent. |
| `contract` | La version du contrat pour laquelle le thème a été écrit. Elle vaut 1 aujourd'hui. |
| `version` | Une [version sémantique](https://semver.org/lang/fr/), comme `1.0.0`. |
| `name`, `description` | Langue → texte, `en` obligatoire. Une langue absente affiche le texte anglais. |
| `about` | En option, comme `name`. Une phrase pour la galerie de thèmes. |
| `author` | Non vide. |
| `schemes` | `["light"]`, ou `["light", "dark"]` quand `theme.css` a un bloc sombre. |
| `preview` | `bar`, `paper`, `ink` et `accent` de chaque mode en `#rrggbb`, égaux aux jetons de ces noms. |

Aucun autre champ n'est admis.

## Le contrat

`theme.css` s'appuie sur la même structure que tous les autres thèmes :

- **Les jetons.** Un bloc avec tous les jetons que déclare Carnet, et un bloc sombre
  en option avec les couleurs qui changent. Voir
  [Tokens](../../backend/internal/themes/ANATOMY.md#2-tokens).
- **Les points d'accroche.** Les noms de classes que vous pouvez styler, page par
  page. Voir [Hooks](../../backend/internal/themes/ANATOMY.md#3-hooks).
- **L'ancre.** Chaque sélecteur commence par `[data-skin='<id>']`, ou
  `[data-skin='<id>'][data-scheme='dark']`. Jamais `:root`, `html` ou `body`.

Le champ `contract` dit pour quelle version de ces jetons et points d'accroche le
thème a été écrit. Une instance refuse un paquet dont le `contract` dépasse le
sien. Elle en accepte un plus bas, et l'interface signale que le thème peut
s'afficher mal. La liste des
[changements de contrat](../../backend/internal/themes/ANATOMY.md#6-contract-changes)
dit ce que chaque version a changé.

## Les règles

Les vérifications imposent la plupart d'entre elles. Suivez-les même si le thème
n'est que pour vous : Mes thèmes ne vérifie rien.

- **Pas de JavaScript.** Un thème, c'est du CSS, des images et des polices. Un
  fichier `.js` est refusé, et un SVG ne doit pas contenir de script.
- **Pas de `@import`.** Déclarez les polices avec `@font-face` en haut de
  `theme.css`.
- **`url()` pointe dans le paquet** : `assets/…` ou `fonts/…`, écrit exactement
  ainsi, sans `./` ni `..`, ou une URL `data:`. Pas d'adresse web.
- **Polices sous licence libre uniquement.** Chaque police a besoin d'un fichier de
  licence à côté d'elle : `<famille>-OFL.txt` pour `<famille>-….woff2`, ou un
  `.txt` sans tiret dans son nom pour tout le dossier. Les vérifications voient
  seulement que le fichier existe : c'est à vous de choisir une licence qui le
  permet.
- **Pas de marques ni de logos de tiers** : ni noms, ni logos, ni icônes, ni polices,
  ni fichiers d'une autre entreprise. Aucune vérification ne le voit non plus :
  cela dépend de vous.
- **Des couleurs lisibles.** Le texte demande un contraste de 4,5:1, les contours de
  focus et les accents 3:1, dans chaque mode.
- **Tailles de police en `rem`**, pas en `px`.
- **Pas d'échappements CSS** (`\`) hors des chaînes.

## Les vérifications

L'import en tant qu'administrateur lance les vérifications. Il n'y a pas d'autre
moyen de les lancer sur votre paquet, et Mes thèmes ne les lance pas : un thème qui
y marche peut encore être refusé. Sans droits d'administrateur, demandez à un
administrateur, ou utilisez votre propre instance
([installation](install.md)).

Un paquet refusé n'enregistre rien, et la page liste ce qui ne va pas. L'API répond
la même liste. Chaque résultat a trois parties :

```text
theme.css:12   url        url(./assets/moon.svg): want a file under assets/ or fonts/, or data:
```

- **Où** : le fichier, et la ligne quand il y en a une.
- **La règle** : le tableau dit quoi faire pour chacune.
- **Le message**, en anglais : ce qui a été trouvé.

| Règle | Signifie | Correction |
|---|---|---|
| `zip` | Le zip lui-même : un fichier non admis, un dossier autre que `assets` ou `fonts`, trop de fichiers, trop gros. | Compressez le contenu du dossier, avec seulement [les fichiers ci-dessus](#le-paquet). |
| `manifest` | `theme.json` manque, n'est pas valide ou a un champ inconnu. | Voir [les champs](#le-paquet). |
| `css` | `theme.css` manque ou ne peut pas être lu. | Mettez-le à la racine du zip. |
| `size` | Plus de 100 fichiers ou de 10 Mio. | Retirez des fichiers. |
| `tokens` | Un jeton manque, ou aucun bloc ne correspond à `id`. | Déclarez tous les jetons. Comparez `[data-skin='<id>']` à `id`. |
| `contrast` | Deux couleurs sont trop proches, dans le mode que nomme le message. | Changez l'une des deux. |
| `schemes` | `schemes` ne correspond pas à la présence d'un bloc sombre. | Indiquez `dark` exactement quand il y a un bloc sombre. |
| `preview` | Une couleur d'aperçu diffère de son jeton. | Recopiez la valeur du jeton dans `theme.json`. |
| `font-size` | Une taille de police en `px`. | Utilisez `rem`, à 16 px pour 1 rem. |
| `import` | Un `@import`. | Retirez-le. Déclarez les polices avec `@font-face`. |
| `escape` | Un `\` hors d'une chaîne. | Écrivez le caractère, ou placez-le dans une chaîne. |
| `url` | Un `url()` sort du paquet, ou son fichier n'y est pas. | Pointez vers un fichier existant sous `assets/` ou `fonts/`. |
| `load` | `var()` ou un autre appel dans `image-set()`, `image()` ou `src()`. | Mettez-y des chaînes ou `url()`. |
| `licence` | Une police n'a pas de fichier de licence à côté d'elle. | Ajoutez-en un, comme dans [les règles](#les-règles). |

## Essayer un thème

- **Mes thèmes** (`/settings/themes`) garde un thème pour vous, avec des aperçus.
- **La galerie** (`/themes`) le montre à côté des autres, sous **Mes thèmes**.
- **`?skin=<id>`** dans l'adresse force un thème pour ce chargement de page, sans
  changer votre choix. Un thème de Mes thèmes a l'id `local-<id>` :
  `?skin=local-mon-theme`.

**Mode sans échec.** Si un thème casse l'interface, ajoutez `?skin=carnet` à
l'adresse : la page se charge avec Carnet. Pour supprimer le thème, ouvrez
`/settings/themes?skin=carnet`.

Quand un thème manque, ne se charge pas ou met plus de 5 secondes, Solstice affiche
Carnet pour ce chargement de page, avec un message. Votre choix reste, et le thème
revient dès qu'il fonctionne.

## Gardez votre zip

Un thème importé dans Mes thèmes reste dans ce navigateur et ne va sur aucun
serveur :

- Personne d'autre ne le voit, et vos autres navigateurs et appareils non plus.
- Toute personne qui utilise le même profil de navigateur l'a, quel que soit le
  compte avec lequel elle se connecte.
- Effacer les données du site le supprime.

Gardez donc le zip. **Exporter** dans Mes thèmes redonne le zip que vous avez
importé, tant que le thème est encore là.

## Importer un thème pour tout le monde

Un administrateur peut proposer un thème à toute l'instance :

1. Ouvrez **Administration → Thèmes** (`/admin/themes`).
2. Choisissez **Importer un thème** et prenez le zip. Si les vérifications trouvent
   quelque chose, la page liste les résultats et rien n'est enregistré.
3. Le thème est activé aussitôt et apparaît dans le menu des thèmes de chacun.

Si l'id est pris, le thème est servi sous un id suffixé (`retro-2`). Les fichiers
ne changent pas. Pour mettre un thème à jour, choisissez **Remplacer** sur sa ligne
et donnez une nouvelle version avec le même `id`.

Pour les limites, la désactivation et la suppression, voir
[l'administration](administration.md#thèmes). L'API est décrite dans
`backend/internal/api/ADMIN_API.md`, section « Themes ».
