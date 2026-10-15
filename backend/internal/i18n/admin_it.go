package i18n

// Messages of the administration, the profile and the import, in Italian.
func init() {
	for k, v := range adminIT {
		if _, ok := catalogues["it"][k]; !ok {
			catalogues["it"][k] = v
		}
	}
}

var adminIT = map[string]string{
	// Projects and catalogues
	"Clé invalide : de 2 à 10 lettres majuscules ou chiffres, en commençant par une lettre.": "Chiave non valida: da 2 a 10 lettere maiuscole o cifre, iniziando con una lettera.",
	"Un projet s'appelle déjà « %s ».":                                                       "Un progetto si chiama già «%s».",
	"Le responsable doit être un compte actif.":                                              "Il responsabile deve essere un account attivo.",
	"Catégorie introuvable.":                                                                 "Categoria non trovata.",
	"Choisissez au moins un type de ticket qui ne soit pas une sous-tâche.":                  "Scegli almeno un tipo di ticket che non sia un sotto-task.",
	"Modèle inconnu : software ou simple.":                                                   "Modello sconosciuto: software o simple.",
	"La clé %s est déjà prise (par un projet actuel ou ancien).":                             "La chiave %s è già occupata (da un progetto attuale o passato).",
	"Type de ticket inconnu : %s.":                                                           "Tipo di ticket sconosciuto: %s.",
	"Assignation par défaut : unassigned ou lead.":                                           "Assegnazione predefinita: unassigned o lead.",
	"Des tickets utilisent encore les types retirés : changez leur type d'abord.":            "Alcuni ticket usano ancora i tipi rimossi: cambia prima il loro tipo.",
	"Ce projet contient des tickets : tapez sa clé pour confirmer la suppression.":           "Questo progetto contiene dei ticket: digita la sua chiave per confermare l'eliminazione.",
	"Le nom est obligatoire (%d caractères au plus).":                                        "Il nome è obbligatorio (al massimo %d caratteri).",
	"« %s » existe déjà.":                                                                    "«%s» esiste già.",

	// Default catalogue names
	"Tâche":                   "Task",
	"Bug":                     "Bug",
	"Story":                   "Story",
	"Sous-tâche":              "Sotto-task",
	"Très haute":              "Molto alta",
	"Haute":                   "Alta",
	"Moyenne":                 "Media",
	"Basse":                   "Bassa",
	"Très basse":              "Molto bassa",
	"Ne sera pas fait":        "Non sarà fatto",
	"Doublon":                 "Duplicato",
	"Impossible à reproduire": "Impossibile riprodurre",
	"Blocage":                 "Blocco",
	"est bloqué par":          "è bloccato da",
	"bloque":                  "blocca",
	"est lié à":               "è collegato a",
	"est dupliqué par":        "è duplicato da",
	"duplique":                "duplica",
	"Clone":                   "Clone",
	"est cloné par":           "è clonato da",
	"clone":                   "clona",
	"Administrateurs":         "Amministratori",
	"Développeurs":            "Sviluppatori",
	"Utilisateurs":            "Utenti",

	// Parts (versions, components)
	"Version de destination introuvable dans ce projet.":               "Versione di destinazione non trovata in questo progetto.",
	"Les tickets non résolus vont dans une version pas encore sortie.": "I ticket non risolti vanno in una versione non ancora rilasciata.",
	"La date de début doit précéder la date de sortie.":                "La data di inizio deve precedere la data di rilascio.",
	"Composant de remplacement introuvable dans ce projet.":            "Componente di sostituzione non trovato in questo progetto.",
	"L'ordre doit citer chaque élément une fois et une seule.":         "L'ordine deve elencare ogni elemento una volta e una sola.",
	"Assignation par défaut inconnue (au choix : %s).":                 "Assegnazione predefinita sconosciuta (scegli tra: %s).",

	// Fields
	"Projet inconnu : %s.": "Progetto sconosciuto: %s.",
	"Des tickets ont une valeur dans ce champ : tapez son nom pour confirmer la suppression.": "Alcuni ticket hanno un valore in questo campo: digita il suo nome per confermare l'eliminazione.",
	"Type de champ inconnu (au choix : %s).":                                                  "Tipo di campo sconosciuto (scegli tra: %s).",
	"Une option a un texte de 255 caractères au plus.":                                        "Un'opzione ha un testo di al massimo 255 caratteri.",
	"Ce type de champ n'a pas d'options.":                                                     "Questo tipo di campo non ha opzioni.",
	"Le champ « %s » n'est pas proposé pour ce projet ou ce type de ticket.":                  "Il campo «%s» non è disponibile per questo progetto o questo tipo di ticket.",
	"Ce champ sert au fonctionnement des tableaux : il ne peut pas être supprimé.":            "Questo campo serve al funzionamento delle board: non può essere eliminato.",

	// Catalogs (issue types, link types)
	"Il faut garder au moins un type de ticket.":                                               "Deve rimanere almeno un tipo di ticket.",
	"Des tickets utilisent ce type : il ne peut plus devenir ou cesser d'être une sous-tâche.": "Alcuni ticket usano questo tipo: non può più diventare o cessare di essere un sotto-task.",
	"Le type de remplacement doit être de même nature (sous-tâche ou non).":                    "Il tipo di sostituzione deve essere della stessa natura (sotto-task o no).",
	"Élément de remplacement introuvable.":                                                     "Elemento di sostituzione non trovato.",
	"Cet élément sert encore : choisissez vers quel autre déplacer ce qui l'utilise.":          "Questo elemento è ancora in uso: scegli verso quale altro spostare ciò che lo usa.",
	"Un type de lien a un nom, un sens direct et un sens inverse.":                             "Un tipo di collegamento ha un nome, un senso diretto e un senso inverso.",
	"Couleur invalide : #rrggbb attendu.":                                                      "Colore non valido: atteso #rrggbb.",

	// People, roles, groups
	"Le rôle « %s » existe déjà.":   "Il ruolo «%s» esiste già.",
	"Le groupe « %s » existe déjà.": "Il gruppo «%s» esiste già.",
	"Utilisateur introuvable.":      "Utente non trovato.",
	"Des commentaires ou des temps saisis sont réservés à ce rôle : il ne peut pas être supprimé.":   "Alcuni commenti o tempi registrati sono riservati a questo ruolo: non può essere eliminato.",
	"Des commentaires ou des temps saisis sont réservés à ce groupe : il ne peut pas être supprimé.": "Alcuni commenti o tempi registrati sono riservati a questo gruppo: non può essere eliminato.",

	// Settings
	"Le nom de l'instance est obligatoire (%d caractères au plus).":         "Il nome dell'istanza è obbligatorio (al massimo %d caratteri).",
	"Le texte du bandeau fait %d caractères au plus.":                       "Il testo del banner può avere al massimo %d caratteri.",
	"Niveau de bandeau inconnu : info ou warning.":                          "Livello del banner sconosciuto: info o warning.",
	"Durée de connexion invalide : de 1 à 365 jours.":                       "Durata di connessione non valida: da 1 a 365 giorni.",
	"Longueur minimale des mots de passe invalide : de 8 à 128 caractères.": "Lunghezza minima delle password non valida: da 8 a 128 caratteri.",

	// Import
	"Un import depuis Jira remplace les données : Solstice revient dans quelques minutes.": "Un'importazione da Jira sostituisce i dati: Solstice torna tra qualche minuto.",
	"Tapez le nom de la base de données (%s) pour confirmer le remplacement.":              "Digita il nome del database (%s) per confermare la sostituzione.",

	// Profile and passwords
	"Un jeton d'accès ne peut pas changer le mot de passe : connectez-vous avec votre mot de passe.": "Un token di accesso non può cambiare la password: accedi con la tua password.",
	"Indiquez votre mot de passe actuel.":                    "Indica la tua password attuale.",
	"Mot de passe actuel incorrect.":                         "Password attuale errata.",
	"Changez votre propre mot de passe depuis votre profil.": "Cambia la tua password dal tuo profilo.",
}
