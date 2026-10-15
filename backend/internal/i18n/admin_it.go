package i18n

// Messages of the administration, the profile and the import, in Italian.
func init() {
	for k, v := range adminIT {
		if _, ok := catalogues["it"][k]; !ok {
			catalogues["it"][k] = v
		}
	}
}

var adminIT = map[string]string{ // #nosec G101 -- translation catalogue, not credentials
	// Projects and catalogues
	"Invalid key: 2 to 10 uppercase letters or digits, starting with a letter.": "Chiave non valida: da 2 a 10 lettere maiuscole o cifre, iniziando con una lettera.",
	"A project is already named “%s”.":                                          "Un progetto si chiama già «%s».",
	"The lead must be an active account.":                                       "Il responsabile deve essere un account attivo.",
	"Category not found.":                                                       "Categoria non trovata.",
	"Choose at least one issue type that is not a sub-task.":                    "Scegli almeno un tipo di ticket che non sia un sotto-task.",
	"Unknown template: software or simple.":                                     "Modello sconosciuto: software o simple.",
	"The key %s is already taken (by a current or former project).":             "La chiave %s è già occupata (da un progetto attuale o passato).",
	"Unknown issue type: %s.":                                                   "Tipo di ticket sconosciuto: %s.",
	"Default assignment: unassigned or lead.":                                   "Assegnazione predefinita: unassigned o lead.",
	"Some issues still use the removed types: change their type first.":         "Alcuni ticket usano ancora i tipi rimossi: cambia prima il loro tipo.",
	"This project contains issues: type its key to confirm the deletion.":       "Questo progetto contiene dei ticket: digita la sua chiave per confermare l'eliminazione.",
	"A name is required (%d characters at most).":                               "Il nome è obbligatorio (al massimo %d caratteri).",
	"“%s” already exists.":                                                      "«%s» esiste già.",

	// Default catalogue names
	"Task":             "Task",
	"Bug":              "Bug",
	"Story":            "Story",
	"Sub-task":         "Sotto-task",
	"Very high":        "Molto alta",
	"High":             "Alta",
	"Medium":           "Media",
	"Low":              "Bassa",
	"Very low":         "Molto bassa",
	"Will not be done": "Non sarà fatto",
	"Duplicate":        "Duplicato",
	"Cannot Reproduce": "Impossibile riprodurre",
	"Blocks":           "Blocco",
	"is blocked by":    "è bloccato da",
	"blocks":           "blocca",
	"is linked to":     "è collegato a",
	"is duplicated by": "è duplicato da",
	"duplicates":       "duplica",
	"Clone":            "Clone",
	"is cloned by":     "è clonato da",
	"clones":           "clona",
	"Administrators":   "Amministratori",
	"Developers":       "Sviluppatori",
	"Users":            "Utenti",

	// Parts (versions, components)
	"Target version not found in this project.":                           "Versione di destinazione non trovata in questo progetto.",
	"Unresolved issues go into a version that has not been released yet.": "I ticket non risolti vanno in una versione non ancora rilasciata.",
	"The start date must precede the release date.":                       "La data di inizio deve precedere la data di rilascio.",
	"Replacement component not found in this project.":                    "Componente di sostituzione non trovato in questo progetto.",
	"The order must list every element once and only once.":               "L'ordine deve elencare ogni elemento una volta e una sola.",
	"Unknown default assignment (choose from: %s).":                       "Assegnazione predefinita sconosciuta (scegli tra: %s).",

	// Fields
	"Unknown project: %s.": "Progetto sconosciuto: %s.",
	"Some issues have a value in this field: type its name to confirm the deletion.": "Alcuni ticket hanno un valore in questo campo: digita il suo nome per confermare l'eliminazione.",
	"Unknown field type (choose from: %s).":                                          "Tipo di campo sconosciuto (scegli tra: %s).",
	"An option's text is 255 characters at most.":                                    "Un'opzione ha un testo di al massimo 255 caratteri.",
	"This field type has no options.":                                                "Questo tipo di campo non ha opzioni.",
	"The “%s” field is not offered for this project or issue type.":                  "Il campo «%s» non è disponibile per questo progetto o questo tipo di ticket.",
	"This field is used by the boards: it cannot be deleted.":                        "Questo campo serve al funzionamento delle board: non può essere eliminato.",

	// Catalogs (issue types, link types)
	"At least one issue type must be kept.":                                         "Deve rimanere almeno un tipo di ticket.",
	"Some issues use this type: it can no longer become or stop being a sub-task.":  "Alcuni ticket usano questo tipo: non può più diventare o cessare di essere un sotto-task.",
	"The replacement must be of the same kind (sub-task or not).":                   "Il tipo di sostituzione deve essere della stessa natura (sotto-task o no).",
	"Replacement element not found.":                                                "Elemento di sostituzione non trovato.",
	"This element is still in use: choose which other one to move what uses it to.": "Questo elemento è ancora in uso: scegli verso quale altro spostare ciò che lo usa.",
	"A link type has a name, an outward direction and an inward one.":               "Un tipo di collegamento ha un nome, un senso diretto e un senso inverso.",
	"Invalid color: #rrggbb expected.":                                              "Colore non valido: atteso #rrggbb.",

	// People, roles, groups
	"The role “%s” already exists.":  "Il ruolo «%s» esiste già.",
	"The group “%s” already exists.": "Il gruppo «%s» esiste già.",
	"User not found.":                "Utente non trovato.",
	"Some comments or work logs are restricted to this role: it cannot be deleted.":  "Alcuni commenti o tempi registrati sono riservati a questo ruolo: non può essere eliminato.",
	"Some comments or work logs are restricted to this group: it cannot be deleted.": "Alcuni commenti o tempi registrati sono riservati a questo gruppo: non può essere eliminato.",

	// Settings
	"The instance name is required (%d characters at most).": "Il nome dell'istanza è obbligatorio (al massimo %d caratteri).",
	"The banner text is %d characters at most.":              "Il testo del banner può avere al massimo %d caratteri.",
	"Unknown banner level: info or warning.":                 "Livello del banner sconosciuto: info o warning.",
	"Invalid session length: 1 to 365 days.":                 "Durata di connessione non valida: da 1 a 365 giorni.",
	"Invalid minimum password length: 8 to 128 characters.":  "Lunghezza minima delle password non valida: da 8 a 128 caratteri.",

	// Import
	"An import from Jira replaces the data: Solstice will be back in a few minutes.": "Un'importazione da Jira sostituisce i dati: Solstice torna tra qualche minuto.",
	"Type the database name (%s) to confirm the replacement.":                        "Digita il nome del database (%s) per confermare la sostituzione.",

	// Profile and passwords
	"An access token cannot change the password: sign in with your password.": "Un token di accesso non può cambiare la password: accedi con la tua password.",
	"Enter your current password.":                                            "Indica la tua password attuale.",
	"Incorrect current password.":                                             "Password attuale errata.",
	"Change your own password from your profile.":                             "Cambia la tua password dal tuo profilo.",
}
