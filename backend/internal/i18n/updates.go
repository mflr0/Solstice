package i18n

// Messages of the update check and of updates through the helper (package
// api, updates.go), kept in their own file and merged into the catalogues
// at start. An entry the main catalogue already has wins.
func init() {
	for locale, msgs := range map[string]map[string]string{"fr": updatesFR, "de": updatesDE, "it": updatesIT, "hi": updatesHI} {
		for k, v := range msgs {
			if _, ok := catalogues[locale][k]; !ok {
				catalogues[locale][k] = v
			}
		}
	}
}

var updatesFR = map[string]string{
	"Incorrect password.": "Mot de passe incorrect.",
	"This installation cannot be updated from the interface: follow the instructions on the page.": "Cette installation ne se met pas à jour depuis l'interface : suivez les instructions de la page.",
	"This is not the latest release. Check for updates again.":                                     "Cette version n'est pas la dernière publiée : vérifiez à nouveau les mises à jour.",
	"Solstice is already up to date.":                                                              "Solstice est déjà à jour.",
	"An update is already in progress.":                                                            "Une mise à jour est déjà en cours.",
}

var updatesDE = map[string]string{
	"Incorrect password.": "Falsches Passwort.",
	"This installation cannot be updated from the interface: follow the instructions on the page.": "Diese Installation lässt sich nicht über die Oberfläche aktualisieren. Folgen Sie den Anweisungen auf der Seite.",
	"This is not the latest release. Check for updates again.":                                     "Diese Version ist nicht die zuletzt veröffentlichte. Suchen Sie erneut nach Updates.",
	"Solstice is already up to date.":                                                              "Solstice ist bereits auf dem neuesten Stand.",
	"An update is already in progress.":                                                            "Es läuft bereits ein Update.",
}

var updatesIT = map[string]string{
	"Incorrect password.": "Password errata.",
	"This installation cannot be updated from the interface: follow the instructions on the page.": "Questa installazione non si aggiorna dall'interfaccia: segui le istruzioni della pagina.",
	"This is not the latest release. Check for updates again.":                                     "Questa non è l'ultima versione pubblicata: controlla di nuovo gli aggiornamenti.",
	"Solstice is already up to date.":                                                              "Solstice è già aggiornato.",
	"An update is already in progress.":                                                            "Un aggiornamento è già in corso.",
}

var updatesHI = map[string]string{
	"Incorrect password.": "पासवर्ड गलत है।",
	"This installation cannot be updated from the interface: follow the instructions on the page.": "यह इंस्टॉलेशन इंटरफ़ेस से अपडेट नहीं होता। पेज पर दिए निर्देशों का पालन करें।",
	"This is not the latest release. Check for updates again.":                                     "यह नवीनतम प्रकाशित वर्ज़न नहीं है। अपडेट दोबारा जाँचें।",
	"Solstice is already up to date.":                                                              "Solstice पहले से अप-टू-डेट है।",
	"An update is already in progress.":                                                            "एक अपडेट पहले से चल रहा है।",
}
