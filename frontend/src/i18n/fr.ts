/**
 * French, the source language. Every string the user sees is written here
 * first; every other catalogue (en.ts, de.ts…) has exactly these keys, as the
 * Messages type derived from this object requires (src/i18n/types.ts).
 * Code and comments are in English. src/i18n/README.md tells how to translate.
 *
 * This module also serves the language in use, so components import from
 * here whatever the language: `t` is a live binding that boot points at the
 * visitor's catalogue (src/i18n/index.ts), and the helpers below read it.
 */

import type { Ago, Day } from '../core/dates.js';
import { ago, parseDay, splitDuration } from '../core/dates.js';
import type { Grant, Holder, Permission, StatusCategory } from '../core/types.js';
import { dateTime, dayMonth, formatNumber, longDay, plural, shortDay } from './format.js';
import type { Messages } from './types.js';

/** "12 mars 2026", "1er avril 2026", "March 12, 2026". */
export function renderDay(day: Day): string {
  return longDay(day);
}

export function renderAgo(a: Ago): string {
  switch (a.kind) {
    case 'now':
      return t.time.justNow;
    case 'minutes':
      return t.time.minutesAgo(a.n);
    case 'hours':
      return t.time.hoursAgo(a.n);
    case 'yesterday':
      return t.time.yesterday;
    case 'days':
      return t.time.daysAgo(a.n);
    case 'date':
      return longDay(a.day);
  }
}

/** Relative wording for an instant, e.g. "il y a 3 h". */
export function relative(iso: string, now?: Date): string {
  return renderAgo(ago(iso, now));
}

/** "12 mars 2026 à 14:05". */
export function formatDateTime(iso: string): string {
  return dateTime(iso);
}

/** A due date, "YYYY-MM-DD" → "12 mars 2026". */
export function formatDay(s: string): string {
  const d = parseDay(s);
  return d ? longDay(d) : s;
}

/** "2 h 30", "2 h", "45 min". */
export function formatDuration(seconds: number): string {
  const { h, m } = splitDuration(seconds);
  return t.time.duration(h, m);
}

/** "820 o", "12 Ko", "1,4 Mo". */
export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} ${t.units.bytes}`;
  if (bytes < 1024 * 1024) return `${formatNumber(bytes / 1024, 1)} ${t.units.kilobytes}`;
  return `${formatNumber(bytes / (1024 * 1024), 1)} ${t.units.megabytes}`;
}

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

/** A history entry's field (Jira's names, lower case), as Solstice shows it. */
export function fieldLabel(field: string): string {
  return (t.historyFields as Record<string, string>)[field.toLowerCase()] ?? field;
}

/** History fields whose values are durations in seconds. */
export const DURATION_FIELDS = new Set(['timeoriginalestimate', 'timeestimate', 'timespent']);

export const fr = {
  appName: 'Solstice',
  tagline: 'Suivi de tickets',

  /** Words of the date, duration and size helpers; src/i18n/format.ts does the rest. */
  time: {
    justNow: 'à l’instant',
    minutesAgo: (n: number) => `il y a ${n} min`,
    hoursAgo: (n: number) => `il y a ${n} h`,
    yesterday: 'hier',
    daysAgo: (n: number) => `il y a ${n} j`,
    /** An absolute date inside a sentence: "created on …". */
    onDate: (date: string) => `le ${date}`,
    /** "2 h 30", "2 h", "45 min". */
    duration: (h: number, m: number) =>
      h === 0 ? `${m} min` : m === 0 ? `${h} h` : `${h} h ${String(m).padStart(2, '0')}`,
  },
  units: { bytes: 'o', kilobytes: 'Ko', megabytes: 'Mo' },

  categories: {
    todo: 'À faire',
    doing: 'En cours',
    done: 'Terminé',
  },

  /** What each permission allows, for the project settings page. */
  permissions: {
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
    watchers: {
      label: 'Gérer les observateurs',
      help: 'Ajouter ou retirer d’autres observateurs.',
    },
    security: { label: 'Confidentialité', help: 'Choisir qui peut voir un ticket.' },
    admin: { label: 'Administrer', help: 'Régler le projet, dont cette page.' },
  },

  holders: {
    everyone: 'Tout le monde',
    user: 'Personne',
    group: 'Groupe',
    role: 'Rôle',
    lead: 'Responsable du projet',
    reporter: 'Rapporteur du ticket',
    assignee: 'Assigné du ticket',
  },

  /** Jira's history field names (lower case), as Solstice shows them. */
  historyFields: {
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
  },

  nav: {
    work: 'Mon travail',
    projects: 'Projets',
    boards: 'Tableaux',
    search: 'Recherche',
    create: 'Nouveau ticket',
    account: 'Compte',
    myPage: 'Ma page',
    admin: 'Utilisateurs',
    skin: 'Thème',
    skins: {
      carnet: 'Carnet',
      orbite: 'Orbite',
      crayon: 'Crayonné',
      luna: 'Luna',
      retro: 'Rétro 98',
      classique: 'Classique',
      terminal: 'Terminal',
      plan: 'Plan',
    },
    skinHints: {
      carnet: 'papier et encre',
      orbite: 'nuit et orbites',
      crayon: 'mine de crayon',
      luna: 'barres bleues, tons beiges',
      retro: 'gris en relief',
      classique: 'blanc et bleu',
      terminal: 'phosphore vert',
      plan: 'calque et cartouches',
    },
    themes: 'Thèmes',
    allThemes: 'Tous les thèmes…',
    mode: 'Mode',
    modes: { system: 'système', light: 'clair', dark: 'sombre' },
    language: 'Langue',
    logout: 'Se déconnecter',
    live: 'Mises à jour en direct',
    offline: 'Hors ligne : les pages ne se mettent plus à jour seules',
  },

  statusbar: {
    ready: 'Prêt',
  },

  themes: {
    title: 'Thèmes',
    eyebrow: 'Apparence',
    intro:
      'Chaque thème redessine toute l’interface, pour vous seul. Cliquez sur un aperçu pour l’appliquer ; le mode clair ou sombre suit votre choix.',
    descriptions: {
      carnet: 'Le carnet de chantier : papier crème, encre noire, un trait vermillon.',
      orbite: 'Observatoire de nuit : bleu profond, étoiles discrètes, le soleil en accent.',
      crayon: 'Esquisse au crayon : graphite, papier grainé ; en sombre, un tableau noir.',
      luna: 'Bureau des années 2000 : barres de titre bleues, bouton vert, tons beiges.',
      retro: 'Bureau de la fin des années 90 : gris en relief, fenêtres marine, fond sarcelle.',
      classique: 'Familier pour qui vient de l’outil d’origine : blanc, bleu, sobre.',
      terminal: 'Écran phosphore vert ; en clair, un listing sur papier zébré.',
      plan: 'Plan d’architecte : bleu, quadrillage blanc, cartouches ; en clair, un tirage.',
    },
    current: 'Thème actuel',
    use: 'Utiliser ce thème',
    apply: (name: string, scheme: string) => `Appliquer ${name}, ${scheme}`,
    schemes: { light: 'clair', dark: 'sombre' },
    lightOnly: 'Existe en clair seulement.',
    preparing: 'Aperçu en préparation…',
    unfinished: 'Ce thème est encore en cours de dessin.',
    mock: {
      work: 'Mon travail',
      projects: 'Projets',
      search: 'Recherche',
      create: 'Nouveau ticket',
      searchPlaceholder: 'Rechercher…',
      eyebrow: 'Mardi 13 octobre',
      greeting: 'Bonjour Marie',
      section: 'Assignés à moi',
      rows: [
        {
          key: 'DEMO-12',
          type: 'Tâche',
          summary: 'Exporter le planning en PDF',
          status: 'En cours',
          category: 'doing',
        },
        {
          key: 'DEMO-9',
          type: 'Récit',
          summary: 'Relire les droits des invités',
          status: 'À faire',
          category: 'todo',
        },
        {
          key: 'DEMO-4',
          type: 'Bogue',
          summary: 'Corriger le tri des dates',
          status: 'Terminé',
          category: 'done',
        },
      ],
      comment: 'Ajouter un commentaire',
      cancel: 'Annuler',
      save: 'Enregistrer',
      fiche: 'Fiche',
      assignee: 'Assigné',
      due: 'Échéance',
      dueValue: '20 oct.',
      todo: 'À faire',
      doing: 'En cours',
      cardSummary: 'Revoir la page d’accueil',
      ready: 'Prêt',
      online: 'En ligne',
    },
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
    /** Between a label and its value (French puts a space before the colon). */
    labelSep: '\u00a0: ',
    range: (first: string, last: string, total: string) => `${first}–${last} sur ${total}`,
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
    results: (n: number) => plural(n, 'ticket', 'tickets'),
    empty: 'Aucun ticket ne correspond.',
    filters: 'Filtres',
    text: 'Texte recherché',
    textPlaceholder: 'Rechercher dans les tickets…',
    fields: {
      project: 'Projet',
      people: 'Personnes',
      type: 'Type',
      status: 'Statut',
      assignee: 'Responsable',
      priority: 'Priorité',
      labels: 'Étiquettes',
      reporter: 'Rapporteur',
      resolution: 'Résolution',
      sprint: 'Sprint',
      epic: 'Épopée',
      fixVersion: 'Version corrigée',
      component: 'Composant',
      created: 'Créé',
      updated: 'Mis à jour',
      due: 'Échéance',
      resolved: 'Résolu',
    },
    more: 'Plus',
    moreLabel: 'Autres critères',
    watching: 'Tickets que j’observe',
    watchingBusy: 'Le filtre Personnes sert déjà à un autre rôle.',
    role: 'Rôle',
    roles: {
      participant: 'Toute implication',
      assignee: 'Responsable',
      reporter: 'Rapporteur',
      creator: 'Créateur',
      watcher: 'Observateur',
      commenter: 'Commentateur',
    } as Record<string, string>,
    roleHint: 'Responsable et Rapporteur ont leur propre filtre : le choix y passe.',
    allTickets: 'Tous ses tickets',
    clearAll: 'Tout effacer',
    clear: (field: string) => `Retirer le filtre ${field}`,
    filterValues: (field: string) => `Chercher : ${field}`,
    filterPlaceholder: 'Filtrer…',
    noValue: 'Aucune valeur ne correspond.',
    loadingValues: 'Chargement…',
    addValue: (v: string) => `Ajouter « ${v} »`,
    close: 'Fermer',
    reset: 'Effacer',
    andMore: (n: number) => `+${n}`,
    special: {
      me: 'Moi',
      unassigned: 'Non assigné',
      open: 'Ouverts',
      done: 'Terminés',
      unresolved: 'Non résolu',
      openSprints: 'Sprints en cours ou à venir',
      futureSprints: 'Sprints à venir',
      closedSprints: 'Sprints terminés',
      noSprint: 'Sans sprint',
      noEpic: 'Sans épopée',
      unreleased: 'Versions non livrées',
      released: 'Versions livrées',
      earliestUnreleased: 'Prochaine version',
      latestReleased: 'Dernière version livrée',
      noVersion: 'Sans version',
      noComponent: 'Sans composant',
      noLabel: 'Sans étiquette',
      noWatcher: 'Sans observateur',
      noComment: 'Sans commentaire',
      standardTypes: 'Types hors sous-tâches',
      subtaskTypes: 'Sous-tâches',
      membersOf: (group: string) => `Membres de ${group}`,
    },
    shortcuts: 'Raccourcis',
    categoryOnly: (name: string) => `catégorie ${name}`,
    released: 'livrée',
    noProject: 'Choisissez un projet pour voir ses valeurs.',
    datePresets: {
      today: 'Aujourd’hui',
      week: 'Cette semaine',
      last7: '7 derniers jours',
      last30: '30 derniers jours',
      month: 'Ce mois-ci',
      older30: 'Il y a plus de 30 jours',
      overdue: 'En retard',
      next7: 'Dans les 7 jours',
      none: 'Sans échéance',
    } as Record<string, string>,
    customRange: 'Dates précises',
    from: 'Du',
    to: 'Au',
    since: (d: string) => `depuis le ${d}`,
    until: (d: string) => `jusqu’au ${d}`,
    between: (a: string, b: string) => `${a} → ${b}`,
    noDate: 'vide',
    /** "2026-10-01" → "1 oct. 2026". */
    day: (ymd: string) => {
      const d = parseDay(ymd);
      return d ? shortDay(d) : ymd;
    },
    sortBy: 'Trier par',
    sorts: {
      updated: 'Mis à jour',
      created: 'Créé',
      priority: 'Priorité',
      key: 'Clé',
      due: 'Échéance',
      status: 'Statut',
      resolved: 'Résolu',
    },
    customSort: 'Tri de la requête',
    ascending: 'Ordre croissant',
    descending: 'Ordre décroissant',
    mode: 'Affichage de la recherche',
    modeFilters: 'Filtres',
    modeJql: 'JQL',
    tooComplex: 'Requête trop complexe pour les filtres',
    tooComplexHint:
      'Elle emploie OU, NON, ou des critères que les filtres ne montrent pas : elle reste ici, en JQL.',
    jqlLabel: 'Requête JQL',
    run: 'Rechercher',
    jqlHint: 'Entrée lance la recherche · Maj + Entrée va à la ligne · Ctrl + Espace propose',
    suggestions: 'Suggestions',
    function: 'fonction',
    customField: 'champ personnalisé',
    keyword: 'mot-clé',
    operator: 'opérateur',
    jqlFields: {
      project: 'projet',
      type: 'type de ticket',
      status: 'statut',
      statusCategory: 'catégorie de statut',
      assignee: 'responsable',
      reporter: 'rapporteur',
      creator: 'créateur',
      priority: 'priorité',
      resolution: 'résolution',
      labels: 'étiquettes',
      component: 'composant',
      fixVersion: 'version corrigée',
      affectedVersion: 'version affectée',
      sprint: 'sprint',
      'Epic Link': 'épopée',
      parent: 'ticket parent',
      key: 'clé',
      text: 'résumé et description',
      summary: 'résumé',
      description: 'description',
      comment: 'commentaires',
      environment: 'environnement',
      created: 'date de création',
      updated: 'dernière modification',
      resolved: 'date de résolution',
      due: 'échéance',
      watcher: 'observateur',
      voter: 'votant',
      level: 'confidentialité',
      timespent: 'temps passé',
      originalEstimate: 'estimation initiale',
      remainingEstimate: 'temps restant',
      participant: 'personne impliquée',
      commenter: 'auteur d’un commentaire',
    } as Record<string, string>,
  },

  quick: {
    label: 'Recherche rapide',
    placeholder: 'Rechercher des tickets, une clé, une personne…',
    issues: 'Tickets',
    projects: 'Projets',
    people: 'Personnes',
    searchAll: (q: string) => `Rechercher « ${q} » dans tous les tickets`,
    ticketsOf: (name: string) => `Tickets de ${name}`,
    assigned: 'Assignés',
    assignedTo: (name: string) => `Assignés à ${name}`,
    reported: 'Signalés',
    reportedBy: (name: string) => `Signalés par ${name}`,
    profile: 'Profil',
    profileOf: (name: string) => `Voir le profil de ${name}`,
  },

  saved: {
    title: 'Filtres enregistrés',
    toggle: (n: number) => `Filtres enregistrés (${n})`,
    favourites: 'Favoris',
    mine: 'Mes filtres',
    shared: 'Partagés avec moi',
    none: 'Aucun filtre pour l’instant. Enregistrez une recherche pour la retrouver ici.',
    save: 'Enregistrer',
    saveAs: 'Enregistrer sous…',
    saveSearch: 'Enregistrer la recherche',
    modified: 'modifié',
    rename: 'Renommer',
    delete: 'Supprimer',
    star: (name: string) => `Ajouter « ${name} » aux favoris`,
    unstar: (name: string) => `Retirer « ${name} » des favoris`,
    name: 'Nom',
    description: 'Description',
    newTitle: 'Enregistrer la recherche',
    renameTitle: 'Renommer le filtre',
    deleteTitle: 'Supprimer le filtre',
    created: (name: string) => `Filtre « ${name} » enregistré.`,
    updated: (name: string) => `Filtre « ${name} » mis à jour.`,
    deleted: (name: string) => `Filtre « ${name} » supprimé.`,
    confirmDelete: (name: string) =>
      `Supprimer le filtre « ${name} » ? Les personnes avec qui il est partagé ne le verront plus.`,
    by: (who: string) => `de ${who}`,
    current: 'Filtre',
    gone: 'Ce filtre n’existe plus, ou n’est pas partagé avec vous.',
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
    linkPlaceholder: 'Clé du ticket',
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
    createdOn: (when: string) => `créé ${when}`,
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
  },

  editor: {
    loading: 'Ouverture de l’éditeur…',
    toolbar: 'Mise en forme',
    style: 'Style du paragraphe',
    paragraph: 'Texte',
    heading: (level: number) => `Titre ${level}`,
    bold: 'Gras',
    italic: 'Italique',
    underline: 'Souligné',
    strike: 'Barré',
    code: 'Code dans le texte',
    link: 'Lien',
    bulletList: 'Liste à puces',
    orderedList: 'Liste numérotée',
    quote: 'Citation',
    codeBlock: 'Bloc de code',
    table: 'Tableau',
    rule: 'Trait de séparation',
    mention: 'Mentionner quelqu’un',
    undo: 'Annuler la frappe',
    redo: 'Rétablir',
    source: 'Source',
    sourceLabel: 'Balisage wiki',
    tableTools: 'Tableau',
    rowAfter: 'Ligne en dessous',
    columnAfter: 'Colonne à droite',
    deleteRow: 'Supprimer la ligne',
    deleteColumn: 'Supprimer la colonne',
    headerRow: 'Ligne d’en-tête',
    deleteTable: 'Supprimer le tableau',
    language: 'Langage',
    noLanguage: 'aucun',
    linkText: 'Texte affiché',
    linkUrl: 'Adresse',
    linkApply: 'Appliquer',
    linkRemove: 'Retirer le lien',
    badUrl: 'Adresse non reconnue : http(s)://, mailto:, /chemin ou clé de ticket.',
    mentionEmpty: 'Personne ne correspond.',
    mentionSearching: 'Recherche…',
    people: 'Personnes',
    raw: 'Balisage conservé tel quel ; double-cliquez pour l’ouvrir en mode Source.',
    uploading: (names: string) => `Envoi de ${names}…`,
    escHint: 'Échap pour annuler',
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

  boards: {
    title: 'Tableaux',
    kinds: { kanban: 'Kanban', scrum: 'Scrum' },
    kindHints: {
      kanban: 'Un flux continu : les tickets avancent de colonne en colonne.',
      scrum: 'Par sprints : on choisit un lot de tickets, puis on le mène à terme.',
    },
    name: 'Nom',
    kind: 'Type',
    projects: 'Projets',
    owner: 'Propriétaire',
    activeSprint: 'Sprint en cours',
    project: 'Projet',
    allProjects: 'Tous les projets',
    empty: 'Aucun tableau ne vous est accessible.',
    emptyProject: (key: string) => `Aucun tableau pour le projet ${key}.`,
    count: (n: number) => plural(n, 'tableau', 'tableaux'),
    create: 'Créer un tableau',
    createTitle: 'Nouveau tableau',
    createNameHint: (example: string) => `Facultatif ; par défaut « ${example} ».`,
    createHelp:
      'Le tableau reprend les tickets du projet, rangés en trois colonnes : À faire, En cours, Terminé. Tout se règle ensuite.',
    createSubmit: 'Créer le tableau',
    creating: 'Création…',
    created: 'Tableau créé.',
    notAdmin: 'Seuls les administrateurs de ce projet peuvent lui créer un tableau.',
    noProject: 'Aucun projet ne vous est accessible.',

    views: 'Vue',
    viewBoard: 'Tableau',
    viewBacklog: 'Backlog',
    settings: 'Réglages',
    quickFilters: 'Filtres rapides',
    showAll: 'Tout afficher',
    cards: (n: number) => plural(n, 'ticket', 'tickets'),
    truncated:
      'Seuls les 1 000 premiers tickets sont affichés : resserrez le filtre du tableau ou utilisez un filtre rapide.',
    emptyBoard: 'Aucun ticket sur ce tableau.',
    emptyFiltered: 'Aucun ticket ne correspond à ces filtres rapides.',
    noActiveSprint: 'Aucun sprint en cours.',
    noActiveSprintHint: 'Les sprints se préparent et se démarrent depuis le backlog.',
    openBacklog: 'Ouvrir le backlog',
    wip: (min: number | null, max: number | null) =>
      [min !== null ? `min ${min}` : '', max !== null ? `max ${max}` : '']
        .filter(Boolean)
        .join(' · '),
    wipOver: 'limite dépassée',
    wipUnder: 'sous le minimum',
    unassigned: 'Non assigné',
    noEpic: 'Sans épopée',
    otherIssues: 'Autres tickets',

    cardActions: (key: string) => `Actions sur ${key}`,
    moveTo: 'Déplacer vers',
    order: 'Ordre',
    moveUp: 'Monter',
    moveDown: 'Descendre',
    toTop: 'Tout en haut',
    toBottom: 'Tout en bas',
    subtaskOf: (key: string) => `Sous-tâche de ${key}`,
    epic: 'Épopée',
    sprint: 'Sprint',
    estimate: 'Estimation',
    points: (n: number) => plural(n, 'point', 'points'),
    /** A card's estimate: story points as a bare number, time in hours. */
    estimateValue: (n: number, kind: '' | 'points' | 'time') =>
      kind === 'time' ? `${formatNumber(n, 1)} h` : formatNumber(n, 1),
    moved: (key: string, where: string) => `${key} déplacé dans « ${where} ».`,
    movedUp: (key: string) => `${key} monté d’une place.`,
    movedDown: (key: string) => `${key} descendu d’une place.`,
    ranked: (key: string) => `${key} a changé de place.`,
    dropHere: 'Déposez un ticket ici, ou passez par le menu d’un ticket.',

    day: (iso: string) => dayMonth(iso),
    dates: (start: string, end: string) => `${start} → ${end}`,
    daysLeft: (n: number) =>
      n > 0
        ? plural(n, 'jour restant', 'jours restants')
        : n === 0
          ? 'se termine aujourd’hui'
          : `dépassé de ${plural(-n, 'jour', 'jours')}`,
    sprintStates: { active: 'en cours', future: 'à venir', closed: 'terminé' },
    goal: 'Objectif',
    startSprint: 'Démarrer le sprint',
    completeSprint: 'Terminer le sprint',
    createSprint: 'Créer un sprint',
    sprintCreated: 'Sprint créé.',
    anotherActive: 'Un sprint est déjà en cours : terminez-le d’abord.',
    startTitle: (name: string) => `Démarrer « ${name} »`,
    startName: 'Nom',
    startGoal: 'Objectif',
    startEnd: 'Dernier jour',
    startEndHint: 'Deux semaines par défaut.',
    starting: 'Démarrage…',
    started: (name: string) => `« ${name} » a commencé.`,
    completeTitle: (name: string) => `Terminer « ${name} »`,
    completeSummary: (done: number, open: number) =>
      `${plural(done, 'ticket terminé', 'tickets terminés')}, ${plural(open, 'ticket non terminé', 'tickets non terminés')}.`,
    completeMoveTo: 'Les tickets non terminés vont dans',
    toBacklog: 'Le backlog',
    completing: 'Clôture…',
    completed: (name: string) => `« ${name} » est terminé.`,
    backlog: 'Backlog',
    backlogEmpty: 'Le backlog est vide.',
    sectionCount: (n: number) => plural(n, 'ticket', 'tickets'),
  },

  boardSettings: {
    title: 'Réglages du tableau',
    back: 'Retour au tableau',
    forbidden:
      'Seuls le propriétaire du tableau et les administrateurs de ses projets peuvent le régler.',
    general: 'Général',
    name: 'Nom',
    filter: 'Filtre du tableau',
    filterHint: 'Le tableau montre les tickets de ce filtre, dans l’ordre de leur rang.',
    seeIssues: 'Voir ces tickets',
    subQuery: 'Sous-filtre',
    subQueryHint: 'Requête JQL facultative, appliquée en plus du filtre.',
    doneCutoff: 'Retirer les tickets terminés après',
    doneCutoffHint:
      'Un nombre suivi de d (jours), w (semaines) ou m (mois), par exemple 2w. Vide : jamais.',
    columns: 'Colonnes',
    columnsHint:
      'Glisser un ticket dans une colonne lui donne le premier statut de la colonne que son workflow permet d’atteindre.',
    columnName: 'Nom de la colonne',
    min: 'Minimum',
    max: 'Maximum',
    statuses: 'Statuts',
    noStatus: 'Aucun statut : cette colonne restera vide.',
    addColumn: 'Ajouter une colonne',
    newColumn: 'Nouvelle colonne',
    moveUp: (name: string) => `Monter « ${name} »`,
    moveDown: (name: string) => `Descendre « ${name} »`,
    moveLeft: (name: string) => `Déplacer « ${name} » vers la gauche`,
    moveRight: (name: string) => `Déplacer « ${name} » vers la droite`,
    remove: (name: string) => `Retirer « ${name} »`,
    unmapped: 'Statuts sans colonne',
    unmappedHint: 'Les tickets dans ces statuts n’apparaissent pas sur le tableau.',
    allMapped: 'Chaque statut des projets du tableau a sa colonne.',
    putIn: (name: string) => `Placer « ${name} » dans`,
    chooseColumn: 'Choisir une colonne…',
    quickFilters: 'Filtres rapides',
    quickFiltersHint: 'Des boutons au-dessus du tableau ; plusieurs filtres actifs se combinent.',
    noQuickFilters: 'Aucun filtre rapide.',
    addQuickFilter: 'Ajouter un filtre rapide',
    newQuickFilter: 'Nouveau filtre',
    query: 'Requête (JQL)',
    description: 'Description',
    swimlanes: 'Couloirs',
    strategies: {
      none: 'Aucun',
      custom: 'Par requête',
      assignee: 'Par assigné',
      epic: 'Par épopée',
      parent: 'Par ticket parent',
    },
    strategyHints: {
      none: 'Une seule bande.',
      custom: 'Une bande par requête, dans l’ordre ; un ticket va dans la première qui le prend.',
      assignee: 'Une bande par personne.',
      epic: 'Une bande par épopée.',
      parent: 'Une bande par ticket ayant des sous-tâches.',
    },
    lanes: 'Couloirs par requête',
    noLanes: 'Aucun couloir.',
    addLane: 'Ajouter un couloir',
    newLane: 'Nouveau couloir',
    defaultLane: 'Par défaut',
    defaultLaneHint: 'Le couloir par défaut reçoit les tickets qu’aucune requête ne prend.',
    estimation: 'Estimation',
    estimateNone: 'Aucune',
    estimateTime: 'Estimation initiale (temps)',
    estimateField: 'Champ numérique (points…)',
    chooseField: 'Choisir un champ…',
    noFields: 'Aucun champ numérique n’existe.',
    save: 'Enregistrer',
    saved: 'Réglages enregistrés.',
    unsaved: 'Modifications non enregistrées',
    discard: 'Annuler les modifications',
    danger: 'Supprimer le tableau',
    dangerHint: 'Les tickets ne sont pas touchés et le filtre du tableau reste disponible.',
    deleteTitle: 'Supprimer le tableau',
    deleteConfirm: (name: string) =>
      `Supprimer « ${name} », ses colonnes, filtres rapides et sprints ? Les tickets ne sont pas touchés.`,
    deleted: (name: string) => `Tableau « ${name} » supprimé.`,
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

  workflows: {
    tab: 'Workflows',
    rightsTab: 'Droits et rôles',
    tabsLabel: 'Parties des réglages',
    intro:
      'Un workflow fixe les statuts d’un ticket et les passages permis entre eux. Chaque type de ticket suit un workflow de ce projet ; celui par défaut prend tous les types qui n’en suivent pas un autre.',
    readOnly: 'Lecture seule : seuls les administrateurs du projet modifient les workflows.',
    name: 'Nom',
    types: 'Types de ticket',
    statuses: 'Statuts',
    transitions: 'Transitions',
    defaultBadge: 'par défaut',
    noTypes: 'aucun type pour l’instant',
    edit: 'Modifier',
    view: 'Voir',
    delete: 'Supprimer',
    newWorkflow: 'Nouveau workflow',
    copyFrom: 'Partir d’une copie de',
    newName: 'Nom du workflow',
    create: 'Créer',
    created: (name: string) => `Workflow « ${name} » créé.`,
    deleteTitle: (name: string) => `Supprimer « ${name} » ?`,
    deleteTypes: (types: string[], fallback: string) =>
      `${types.join(', ')} ${types.length > 1 ? 'reviendront' : 'reviendra'} au workflow par défaut « ${fallback} ».`,
    deleteKeep: 'Les tickets gardent leur statut actuel ; rien n’est effacé de leur historique.',
    deleted: (name: string) => `Workflow « ${name} » supprimé.`,
    back: 'Tous les workflows',
    general: 'Général',
    initial: 'Statut de départ',
    initialHint: 'Les nouveaux tickets commencent ici.',
    makeDefault: 'Workflow par défaut',
    makeDefaultHint: 'Il prendra tous les types de ticket qui ne suivent pas un autre workflow.',
    isDefault:
      'Workflow par défaut du projet : il sert tous les types de ticket qui ne suivent pas un autre workflow.',
    servedTypes: 'Types de ticket qui le suivent',
    follows: (name: string) => `suit « ${name} »`,
    start: 'départ',
    issues: (n: number) => plural(n, 'ticket', 'tickets'),
    moveUp: (name: string) => `Monter ${name}`,
    moveDown: (name: string) => `Descendre ${name}`,
    removeStatus: (name: string) => `Retirer ${name}`,
    lastStatus: 'Un workflow garde au moins un statut.',
    reorderHint: 'Glissez les statuts, ou utilisez les flèches, pour changer leur ordre.',
    addStatus: 'Ajouter un statut',
    searchStatus: 'Chercher ou créer un statut',
    noMatch: 'Aucun autre statut ne correspond.',
    createStatus: (name: string) => `Créer « ${name} » :`,
    createAs: 'Catégorie du nouveau statut',
    sharedStatuses: 'Les statuts sont communs à tous les projets ; leur nom se partage.',
    diagram: 'Schéma',
    diagramLabel: (statuses: number, transitions: number) =>
      `Schéma du workflow : ${plural(statuses, 'statut', 'statuts')}, ${plural(transitions, 'transition', 'transitions')}.`,
    anyStatus: 'depuis tout statut',
    anyStatusRow: 'N’importe quel statut',
    count: (n: number) => plural(n, 'transition', 'transitions'),
    matrixHint:
      'Cochez une case pour permettre de passer du statut de la ligne à celui de la colonne. Le nom de la transition est celui du bouton proposé sur le ticket.',
    fromTo: 'depuis ↓ vers →',
    cell: (from: string, to: string) => `De ${from} vers ${to}`,
    cellAny: (to: string) => `De n’importe quel statut vers ${to}`,
    renameLabel: (name: string) => `Renommer la transition ${name}`,
    transitionName: 'Nom de la transition',
    noStatuses: 'Ajoutez des statuts pour tracer des transitions.',
    saved: 'Workflow enregistré.',
    leaveTitle: 'Modifications non enregistrées',
    leaveBody: 'Ce workflow a des modifications non enregistrées. Les abandonner ?',
    stay: 'Continuer la modification',
    leave: 'Abandonner les modifications',
    remapTitle: 'Où vont ces tickets ?',
    moveTo: 'Déplacer vers',
    issuesCol: 'Tickets',
    remapNote: 'Chaque déplacement est inscrit dans l’historique du ticket.',
    remapConfirm: 'Déplacer et enregistrer',
    remapDefault: (name: string) =>
      `Types de ticket rendus au workflow par défaut « ${name} » : leurs statuts absents de celui-ci`,
    remapDeleteConfirm: 'Déplacer et supprimer',
  },
} as const;

/** The catalogue in use: French until boot points it elsewhere (src/i18n/index.ts). */
export let t: Messages = fr;

export let categoryLabels: Record<StatusCategory, string> = fr.categories;
export let permissionLabels: Record<Permission, { label: string; help: string }> = fr.permissions;
export let holderLabels: Record<Holder, string> = fr.holders;

/** Makes a catalogue the one every component reads. Boot only: pages do not re-render. */
export function useMessages(m: Messages): void {
  t = m;
  categoryLabels = m.categories;
  permissionLabels = m.permissions;
  holderLabels = m.holders;
}
