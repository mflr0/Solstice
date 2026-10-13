/**
 * Every string the user sees lives here. Code and comments are in English;
 * the interface is in French. Adding a language means writing a sibling file.
 */

import type { Ago, Day } from '../core/dates.js';
import { ago, localParts, parseDay, splitDuration } from '../core/dates.js';
import type { Grant, Holder, Permission, StatusCategory } from '../core/types.js';

const MONTHS = [
  'janvier',
  'février',
  'mars',
  'avril',
  'mai',
  'juin',
  'juillet',
  'août',
  'septembre',
  'octobre',
  'novembre',
  'décembre',
];

/** "12 mars 2026", "1er avril 2026". */
export function renderDay(day: Day): string {
  const d = day.d === 1 ? '1er' : String(day.d);
  return `${d} ${MONTHS[day.m - 1] ?? ''} ${day.y}`;
}

export function renderAgo(a: Ago): string {
  switch (a.kind) {
    case 'now':
      return 'à l’instant';
    case 'minutes':
      return `il y a ${a.n} min`;
    case 'hours':
      return `il y a ${a.n} h`;
    case 'yesterday':
      return 'hier';
    case 'days':
      return `il y a ${a.n} j`;
    case 'date':
      return renderDay(a.day);
  }
}

/** Relative wording for an instant, e.g. "il y a 3 h". */
export function relative(iso: string, now?: Date): string {
  return renderAgo(ago(iso, now));
}

/** "12 mars 2026 à 14:05". */
export function formatDateTime(iso: string): string {
  const p = localParts(iso);
  return `${renderDay(p)} à ${String(p.hh).padStart(2, '0')}:${String(p.mm).padStart(2, '0')}`;
}

/** A due date, "YYYY-MM-DD" → "12 mars 2026". */
export function formatDay(s: string): string {
  const d = parseDay(s);
  return d ? renderDay(d) : s;
}

/** "2 h 30", "2 h", "45 min". */
export function formatDuration(seconds: number): string {
  const { h, m } = splitDuration(seconds);
  if (h === 0) return `${m} min`;
  if (m === 0) return `${h} h`;
  return `${h} h ${String(m).padStart(2, '0')}`;
}

const sizeFormat = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 1 });

/** "820 o", "12 Ko", "1,4 Mo". */
export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`;
  if (bytes < 1024 * 1024) return `${sizeFormat.format(bytes / 1024)} Ko`;
  return `${sizeFormat.format(bytes / (1024 * 1024))} Mo`;
}

const numberFormat = new Intl.NumberFormat('fr-FR');

/** "1 ticket", "1 204 tickets". */
export function plural(n: number, one: string, many: string): string {
  return `${numberFormat.format(n)} ${n > 1 ? many : one}`;
}

export const categoryLabels: Record<StatusCategory, string> = {
  todo: 'À faire',
  doing: 'En cours',
  done: 'Terminé',
};

/** What each permission allows, for the project settings page. */
export const permissionLabels: Record<Permission, { label: string; help: string }> = {
  browse: { label: 'Voir', help: 'Voir le projet et ses tickets.' },
  create: { label: 'Créer', help: 'Ouvrir de nouveaux tickets.' },
  edit: { label: 'Modifier', help: 'Changer le résumé, la description et les champs.' },
  comment: { label: 'Commenter', help: 'Ajouter des commentaires, modifier les siens.' },
  transition: {
    label: 'Changer le statut',
    help: 'Faire avancer un ticket, le résoudre, le rouvrir.',
  },
  assign: { label: 'Assigner', help: 'Attribuer un ticket à quelqu’un.' },
  assignable: { label: 'Être assigné', help: 'Figurer parmi les personnes assignables.' },
  attach: {
    label: 'Joindre des fichiers',
    help: 'Déposer des pièces jointes, retirer les siennes.',
  },
  link: { label: 'Lier', help: 'Relier des tickets entre eux.' },
  worklog: { label: 'Saisir du temps', help: 'Déclarer le temps passé.' },
  delete: { label: 'Supprimer', help: 'Supprimer définitivement un ticket.' },
  moderate: {
    label: 'Modérer',
    help: 'Modifier ou retirer les commentaires et fichiers des autres.',
  },
  watchers: { label: 'Gérer les observateurs', help: 'Ajouter ou retirer d’autres observateurs.' },
  security: { label: 'Confidentialité', help: 'Choisir qui peut voir un ticket.' },
  admin: { label: 'Administrer', help: 'Régler le projet, dont cette page.' },
};

export const holderLabels: Record<Holder, string> = {
  everyone: 'Tout le monde',
  user: 'Personne',
  group: 'Groupe',
  role: 'Rôle',
  lead: 'Responsable du projet',
  reporter: 'Rapporteur du ticket',
  assignee: 'Assigné du ticket',
};

export function grantLabel(g: Grant): string {
  switch (g.holder) {
    case 'user':
    case 'group':
    case 'role':
      return g.name ? `${g.name}` : `${holderLabels[g.holder]} ${g.id ?? ''}`;
    default:
      return holderLabels[g.holder];
  }
}

/** Jira's history field names, as Solstice shows them. */
const FIELD_LABELS: Record<string, string> = {
  status: 'Statut',
  assignee: 'Assigné',
  reporter: 'Rapporteur',
  resolution: 'Résolution',
  priority: 'Priorité',
  summary: 'Résumé',
  description: 'Description',
  environment: 'Environnement',
  labels: 'Étiquettes',
  component: 'Composant',
  'fix version': 'Version corrigée',
  version: 'Version affectée',
  duedate: 'Échéance',
  issuetype: 'Type',
  attachment: 'Pièce jointe',
  link: 'Lien',
  security: 'Confidentialité',
  timeoriginalestimate: 'Estimation initiale',
  timeestimate: 'Temps restant',
  timespent: 'Temps passé',
  worklogid: 'Temps saisi',
  key: 'Clé',
  project: 'Projet',
  workflow: 'Workflow',
  comment: 'Commentaire',
  parent: 'Parent',
  'epic link': 'Épopée',
  'epic child': 'Ticket de l’épopée',
  sprint: 'Sprint',
  rank: 'Rang',
};

export function fieldLabel(field: string): string {
  return FIELD_LABELS[field.toLowerCase()] ?? field;
}

/** History fields whose values are durations in seconds. */
export const DURATION_FIELDS = new Set(['timeoriginalestimate', 'timeestimate', 'timespent']);

/** Examples shown in the search help; clicking one inserts it. */
export const searchExamples: { q: string; label: string }[] = [
  { q: '@moi', label: 'assignés à moi' },
  { q: '@personne', label: 'sans assigné' },
  { q: 'par:@moi', label: 'que j’ai signalés' },
  { q: 'est:ouvert', label: 'pas encore terminés' },
  { q: 'est:fini', label: 'terminés' },
  { q: 'statut:"En cours"', label: 'un statut précis' },
  { q: 'type:Bug', label: 'un type de ticket' },
  { q: 'priorité:Highest', label: 'une priorité' },
  { q: 'projet:DEMO', label: 'un projet' },
  { q: '#urgent', label: 'une étiquette' },
  { q: 'composant:api', label: 'un composant' },
  { q: 'version:2.4', label: 'une version corrigée' },
  { q: '-est:fini', label: 'le moins exclut' },
  { q: 'tri:priorité', label: 'trier (création, priorité, clé, échéance)' },
];

/** JQL examples: Jira filters work as they are. Clicking one replaces the query. */
export const jqlExamples: { q: string; label: string }[] = [
  {
    q: 'project = DEMO AND status != Done ORDER BY priority DESC',
    label: 'ouverts de DEMO, urgents d’abord',
  },
  {
    q: 'assignee in membersOf("developpeurs") AND updated >= -7d',
    label: 'l’équipe, cette semaine',
  },
  { q: 'assignee = currentUser() AND resolution is EMPTY', label: 'mes tickets non résolus' },
];

export const t = {
  appName: 'Solstice',
  tagline: 'Suivi de tickets',

  nav: {
    work: 'Mon travail',
    projects: 'Projets',
    search: 'Recherche',
    create: 'Nouveau ticket',
    quickSearch: 'Rechercher un ticket',
    quickSearchPlaceholder: 'Rechercher ou DEMO-12',
    account: 'Compte',
    myPage: 'Ma page',
    admin: 'Utilisateurs',
    theme: 'Thème',
    themes: { system: 'système', light: 'clair', dark: 'sombre' },
    logout: 'Se déconnecter',
    live: 'Mises à jour en direct',
    offline: 'Hors ligne : les pages ne se mettent plus à jour seules',
  },

  common: {
    loading: 'Chargement…',
    save: 'Enregistrer',
    saving: 'Enregistrement…',
    cancel: 'Annuler',
    edit: 'Modifier',
    delete: 'Supprimer',
    close: 'Fermer',
    add: 'Ajouter',
    remove: 'Retirer',
    none: 'Aucun',
    noneF: 'Aucune',
    unassigned: 'Non assigné',
    me: 'Moi',
    seeAll: 'Tout voir',
    retry: 'Réessayer',
    write: 'Écrire',
    preview: 'Aperçu',
    nothingToPreview: 'Rien à prévisualiser.',
    inactive: 'compte désactivé',
    confidential: 'Confidentiel',
    previous: 'Précédents',
    next: 'Suivants',
    yes: 'oui',
    no: 'non',
    search: 'Chercher',
    filter: 'Filtrer…',
    ctrlEnter: 'Ctrl + Entrée pour envoyer',
    of: 'sur',
    pagination: 'Pagination',
    escKey: 'Échap',
    confirmDelete: 'Supprimer ?',
    actions: 'Actions',
    breadcrumbs: 'Fil d’Ariane',
    mainNav: 'Navigation principale',
  },

  errors: {
    network: 'Le serveur ne répond pas. Vérifiez votre connexion.',
    generic: 'Une erreur est survenue.',
    notFound: 'Cette page n’existe pas, ou vous n’y avez pas accès.',
  },

  login: {
    title: 'Connexion',
    username: 'Identifiant',
    password: 'Mot de passe',
    submit: 'Se connecter',
    pending: 'Connexion…',
    note: 'Les comptes repris de Jira gardent leur mot de passe.',
  },

  home: {
    title: 'Mon travail',
    assigned: 'Assignés à moi',
    reported: 'Signalés par moi, en cours',
    recent: 'Modifiés récemment',
    emptyAssigned: 'Rien ne vous est assigné en ce moment.',
    emptyReported: 'Aucun ticket ouvert à votre nom.',
    emptyRecent: 'Aucune activité récente.',
    greeting: (name: string) => `Bonjour ${name}.`,
  },

  projects: {
    title: 'Projets',
    key: 'Clé',
    name: 'Nom',
    lead: 'Responsable',
    open: 'Ouverts',
    uncategorised: 'Sans catégorie',
    empty: 'Aucun projet ne vous est accessible.',
    settings: 'Réglages',
    kinds: { software: 'logiciel', business: 'métier', service_desk: 'support' } as Record<
      string,
      string
    >,
    filters: 'Raccourcis',
    quick: [
      { q: 'est:ouvert', label: 'Ouverts' },
      { q: '@moi est:ouvert', label: 'Les miens' },
      { q: '@personne est:ouvert', label: 'Sans assigné' },
      { q: 'est:fini tri:maj', label: 'Terminés' },
    ],
    components: 'Composants',
    versions: 'Versions',
    released: 'livrée',
    archived: 'archivée',
  },

  settings: {
    title: 'Réglages du projet',
    intro:
      'Ces règles ne valent que pour ce projet : il n’y a pas de schéma partagé, modifier ici ne touche aucun autre projet.',
    permissions: 'Qui peut faire quoi',
    holders: 'Bénéficiaires',
    addHolder: 'Ajouter',
    nobody: 'Personne',
    chooseKind: 'Type de bénéficiaire',
    chooseGroup: 'Groupe',
    chooseRole: 'Rôle',
    chooseUser: 'Personne',
    roles: 'Rôles dans ce projet',
    roleMembers: 'Membres',
    noMembers: 'Aucun membre',
    saved: 'Réglages enregistrés.',
    unsaved: 'Modifications non enregistrées',
    discard: 'Annuler les modifications',
    forbidden: 'Seuls les administrateurs du projet peuvent voir cette page.',
    backToProject: 'Retour au projet',
  },

  search: {
    title: 'Recherche',
    placeholder: 'Texte libre, @personne, statut:"En cours", #étiquette…',
    help: 'Syntaxe',
    helpTitle: 'Recherche simple',
    helpIntro:
      'Les mots cherchent dans le résumé et la description. Ces filtres se combinent ; deux valeurs du même filtre sont des alternatives.',
    jqlTitle: 'JQL',
    jqlIntro:
      'Les requêtes JQL de vos filtres Jira fonctionnent telles quelles : Solstice les reconnaît seul.',
    jqlMode: 'JQL',
    jqlModeTitle: 'Requête lue comme du JQL',
    results: (n: number) => plural(n, 'ticket', 'tickets'),
    empty: 'Aucun ticket ne correspond.',
    start: 'Tapez une recherche, ou partez d’un exemple ci-dessous.',
  },

  table: {
    type: 'Type',
    key: 'Clé',
    summary: 'Résumé',
    status: 'Statut',
    priority: 'Priorité',
    assignee: 'Assigné',
    updated: 'Modifié',
    due: 'Échéance',
    overdue: 'en retard',
  },

  issue: {
    description: 'Description',
    noDescription: 'Pas de description.',
    addDescription: 'Ajouter une description',
    environment: 'Environnement',
    subtasks: 'Sous-tâches',
    addSubtask: 'Ajouter une sous-tâche',
    links: 'Liens',
    noLinks: 'Aucun lien.',
    addLink: 'Lier un ticket',
    linkTarget: 'Clé du ticket',
    linkKind: 'Relation',
    linkPlaceholder: 'DEMO-12',
    remoteLinks: 'Liens externes',
    attachments: 'Pièces jointes',
    attach: 'Joindre des fichiers',
    dropHere: 'Déposez les fichiers ici',
    uploading: 'Envoi…',
    missingFile: 'fichier non repris de Jira',
    download: 'Télécharger',
    activity: 'Activité',
    tabs: { comments: 'Commentaires', history: 'Historique', worklogs: 'Temps' },
    fiche: 'Fiche',
    assignee: 'Assigné',
    assignToMe: 'M’assigner',
    reporter: 'Rapporteur',
    priority: 'Priorité',
    type: 'Type',
    components: 'Composants',
    fixVersions: 'Versions corrigées',
    affectsVersions: 'Versions affectées',
    labels: 'Étiquettes',
    addLabel: 'Ajouter une étiquette',
    due: 'Échéance',
    security: 'Confidentialité',
    public: 'Visible par le projet',
    created: 'Créé',
    updated: 'Modifié',
    resolved: 'Résolu',
    resolution: 'Résolution',
    time: 'Temps',
    estimate: 'Estimé',
    remaining: 'Restant',
    spent: 'Passé',
    customFields: 'Autres champs',
    watch: 'Observer',
    watching: 'Observé',
    watchers: (n: number) => plural(n, 'observateur', 'observateurs'),
    vote: 'Voter',
    voted: 'Voté',
    votes: (n: number) => plural(n, 'vote', 'votes'),
    by: 'par',
    createdBy: (when: string, who: string) => `créé ${when} par ${who}`,
    parent: 'Sous-tâche de',
    changeStatus: 'Changer le statut',
    chooseResolution: 'Résolution',
    confirmTransition: 'Valider',
    delete: 'Supprimer le ticket',
    confirmDelete: (key: string) =>
      `Supprimer ${key} définitivement, avec ses sous-tâches, commentaires et fichiers ?`,
    deleted: (key: string) => `${key} a été supprimé.`,
    hidden: (key: string) =>
      `${key} a été enregistré, mais vous ne pouvez plus le voir avec ce niveau de confidentialité.`,
    updatedBy: (who: string) => `Mis à jour par ${who} à l’instant.`,
    someone: 'un collègue',
    moved: (oldKey: string, key: string) => `${oldKey} s’appelle désormais ${key}.`,
    editSummary: 'Modifier le résumé',
    menu: 'Autres actions',
  },

  comments: {
    empty: 'Pas encore de commentaire.',
    placeholder: 'Votre commentaire…',
    send: 'Commenter',
    edited: 'modifié',
    visibleTo: (who: string) => `visible : ${who}`,
    confirmDelete: 'Supprimer ce commentaire ?',
    hint: '*gras*  _italique_  {{code}}  [texte|https://…]  [~identifiant]',
  },

  history: {
    empty: 'Aucune modification enregistrée.',
    changed: 'modifié',
    added: 'ajouté',
    removed: 'retiré',
    showText: 'voir le texte',
  },

  worklogs: {
    empty: 'Aucun temps saisi.',
    total: (d: string) => `${d} au total`,
    duration: 'Durée',
    durationHint: '2 h 30, 45 min, 1,5 h',
    started: 'Date',
    body: 'Travail effectué',
    submit: 'Saisir',
    invalid: 'Durée non comprise : essayez « 2 h 30 » ou « 45 min ».',
  },

  create: {
    title: 'Nouveau ticket',
    subtaskTitle: (parent: string) => `Nouvelle sous-tâche de ${parent}`,
    project: 'Projet',
    type: 'Type',
    summary: 'Résumé',
    description: 'Description',
    priority: 'Priorité',
    assignee: 'Assigné',
    labels: 'Étiquettes',
    components: 'Composants',
    fixVersions: 'Version corrigée',
    due: 'Échéance',
    security: 'Confidentialité',
    submit: 'Créer',
    pending: 'Création…',
    cannotCreate: 'Vous ne pouvez pas créer de ticket dans ce projet.',
    created: (key: string) => `${key} créé.`,
    defaultAssignee: 'Par défaut',
  },

  person: {
    assigned: 'En cours à son nom',
    empty: 'Aucun ticket ouvert à son nom.',
    inactive: 'Compte désactivé',
  },

  admin: {
    title: 'Utilisateurs',
    newUser: 'Nouvel utilisateur',
    username: 'Identifiant',
    name: 'Nom affiché',
    email: 'Adresse e-mail',
    password: 'Mot de passe',
    passwordHint: '10 caractères au moins',
    admin: 'Administrateur',
    active: 'Actif',
    state: 'État',
    noPassword: 'sans mot de passe',
    resetPassword: 'Nouveau mot de passe',
    deactivate: 'Désactiver',
    activate: 'Réactiver',
    makeAdmin: 'Rendre administrateur',
    removeAdmin: 'Retirer l’administration',
    created: 'Compte créé.',
    updated: 'Compte mis à jour.',
    forbidden: 'Cette page est réservée aux administrateurs.',
    since: 'Depuis',
  },

  notFound: {
    title: 'Introuvable',
    back: 'Revenir à mon travail',
  },
} as const;
