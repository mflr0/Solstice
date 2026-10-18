package jirafixture

// French corpora for summaries, descriptions and comments. Placeholders in
// braces are filled by the generator: {host}, {person}, {mention}, {key}.

var (
	bugSubjects = []string{
		"Le panier", "La page de connexion", "L'export PDF des factures", "Le formulaire d'inscription",
		"La recherche avancée", "Le tableau de bord", "L'API de facturation", "La synchronisation mobile",
		"Le module de paiement", "Les notifications e-mail", "Le calendrier des livraisons", "L'import CSV",
	}
	bugProblems = []string{
		"se vide au rafraîchissement", "renvoie une erreur 500", "est très lent au-delà de 1 000 lignes",
		"ignore les caractères accentués", "affiche une date décalée d'une heure", "perd les données saisies",
		"ne respecte pas les droits d'accès", "plante sur Safari 18", "s'affiche mal sur mobile",
		"envoie deux fois le même message",
	}
	taskVerbs   = []string{"Ajouter", "Mettre à jour", "Refondre", "Documenter", "Automatiser", "Migrer", "Simplifier", "Tester"}
	taskObjects = []string{
		"l'export CSV des factures", "la page de profil", "le parcours d'inscription", "les tests de bout en bout",
		"la configuration du cache", "le système de notifications", "la gestion des rôles",
		"les traductions anglaises", "le tableau des commandes", "la page d'accueil",
	}
	storyPersonas = []string{"client", "administrateur", "commercial", "agent du support"}
	storyGoals    = []string{
		"suivre ma livraison", "exporter mes factures", "réinitialiser mon mot de passe",
		"filtrer les commandes par statut", "recevoir une alerte de stock", "partager un panier",
		"voir l'historique de mes demandes",
	}
	epicNames = []string{
		"Refonte du tunnel de commande", "Application mobile v2", "Conformité RGPD", "Nouveau back-office",
		"Performance des pages produit", "Internationalisation",
	}
	subtaskNames = []string{
		"Rédiger les tests unitaires", "Revue de code", "Mettre à jour la documentation", "Préparer la recette",
		"Corriger les retours de recette", "Déployer en préproduction",
	}
	opsSummaries = []string{
		"Renouveler le certificat TLS de {host}", "Augmenter l'espace disque sur {host}",
		"Mettre à jour PostgreSQL sur {host}", "Superviser la file de messages sur {host}",
		"Redémarrage inopiné de {host}", "Sauvegarde nocturne en échec sur {host}",
		"Ouvrir le flux réseau entre {host} et le VPN",
	}
	hosts       = []string{"srv-web-01", "srv-web-02", "srv-db-01", "srv-db-02", "srv-batch-01", "proxy-01"}
	rhSummaries = []string{
		"Préparer l'entretien annuel de {person}", "Mettre à jour la grille salariale",
		"Organiser l'accueil de {person}", "Valider les congés d'été", "Renouveler la mutuelle d'entreprise",
		"Publier l'offre de poste « Développeur Go »", "Planifier la formation sécurité",
		"Préparer la visite médicale de {person}",
	}
	rhPersons = []string{"Mme Lambert", "M. Girard", "Mme Petit", "M. Roux", "Mme Benali"}

	contextSentences = []string{
		"Plusieurs clients nous ont remonté le problème depuis la dernière mise en production.",
		"Le besoin vient de l'équipe commerciale, qui doit pouvoir répondre sans passer par le support.",
		"Le comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.",
		"C'est un prérequis pour la prochaine version, à traiter avant la fin du mois.",
		"La supervision a levé l'alerte trois nuits de suite.",
	}
	reproSteps = []string{
		"Se connecter avec un compte client", "Ajouter deux articles au panier", "Rafraîchir la page",
		"Ouvrir le tableau de bord", "Lancer l'export", "Changer la langue en anglais",
	}
	acceptance = []string{
		"Le résultat est identique sur mobile et sur ordinateur",
		"Les droits existants sont respectés",
		"Un message clair s'affiche en cas d'erreur",
		"La page répond en moins d'une seconde",
		"Les tests de bout en bout couvrent le cas nominal",
	}
	codeSnippets = []string{
		"{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}",
		"{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval '7 days';\n{code}",
		"{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq '.[0]'\n{code}",
	}
	logSnippets = []string{
		"{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}",
		"{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}",
	}
	links = []string{
		"[la maquette|https://maquettes.exemple.fr/panier]",
		"[la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes]",
		"[le ticket éditeur|https://support.exemple.com/tickets/88412]",
	}
	environments = []string{
		"Firefox 131 / macOS 15", "Safari 18 / iOS 18", "Chrome 129 / Windows 11", "Production — srv-web-01",
		"Recette — https://recette.exemple.fr",
	}

	commentBodies = []string{
		"Je confirme, reproduit sur l'environnement de recette.",
		"{mention} tu peux regarder quand tu as un moment ?",
		"Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.",
		"Même cause que {key}, je fais le lien.",
		"Pas reproduit chez moi. Tu as une capture ou les logs ?",
		"C'est bon pour moi, on peut fermer.",
		"Attention : *ne pas déployer avant vendredi*, la migration n'est pas prête.",
		"Estimation revue à la hausse : il faut aussi reprendre l'export.",
		"Point fait avec le client ce matin, il valide la solution proposée.",
		"Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}",
	}
	worklogBodies = []string{
		"Analyse", "Développement", "Revue de code", "Tests et recette", "Point avec le client", "Déploiement",
	}

	systemLabels = []string{
		"urgent", "client", "dette-technique", "ux", "sécurité", "perf", "mobile", "à-valider", "régression",
		"backend", "frontend",
	}
	themeLabels = []string{"accessibilité", "rgpd", "onboarding", "facturation"}
)

// logicielWorkflow is the OSWorkflow descriptor of the "Logiciel" workflow
// stored in jiraworkflows. Statuses: To Do (10000) → In Progress (3) →
// In Review (10002) → Done (10001), plus a global action to Done.
const logicielWorkflow = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE workflow PUBLIC "-//OpenSymphony Group//DTD OSWorkflow 2.8//EN" "http://www.opensymphony.com/osworkflow/workflow_2_8.dtd">
<workflow>
  <meta name="jira.description">Cycle de vie des tickets des projets logiciels</meta>
  <initial-actions>
    <action id="1" name="Create">
      <results>
        <unconditional-result old-status="null" status="open" step="1"/>
      </results>
    </action>
  </initial-actions>
  <global-actions>
    <action id="51" name="Clore directement" view="resolveissue">
      <meta name="jira.description">Ferme le ticket quel que soit son état</meta>
      <results>
        <unconditional-result old-status="null" status="done" step="4"/>
      </results>
    </action>
  </global-actions>
  <steps>
    <step id="1" name="To Do">
      <meta name="jira.status.id">10000</meta>
      <actions>
        <action id="11" name="Démarrer">
          <results>
            <unconditional-result old-status="null" status="underway" step="2"/>
          </results>
        </action>
      </actions>
    </step>
    <step id="2" name="In Progress">
      <meta name="jira.status.id">3</meta>
      <actions>
        <action id="21" name="Demander une revue">
          <results>
            <unconditional-result old-status="null" status="review" step="3"/>
          </results>
        </action>
        <action id="22" name="Remettre à faire">
          <results>
            <unconditional-result old-status="null" status="open" step="1"/>
          </results>
        </action>
      </actions>
    </step>
    <step id="3" name="In Review">
      <meta name="jira.status.id">10002</meta>
      <actions>
        <action id="31" name="Valider" view="resolveissue">
          <results>
            <unconditional-result old-status="null" status="done" step="4"/>
          </results>
        </action>
        <action id="32" name="Renvoyer en cours">
          <results>
            <unconditional-result old-status="null" status="underway" step="2"/>
          </results>
        </action>
      </actions>
    </step>
    <step id="4" name="Done">
      <meta name="jira.status.id">10001</meta>
      <actions>
        <action id="41" name="Rouvrir">
          <results>
            <unconditional-result old-status="null" status="open" step="1"/>
          </results>
        </action>
      </actions>
    </step>
  </steps>
</workflow>
`
