package i18n

// Messages of the update check and of updates through the helper (package
// api, updates.go), kept in their own file and merged into the catalogues
// at start. An entry the main catalogue already has wins.
func init() {
	for locale, msgs := range map[string]map[string]string{"en": updatesEN, "de": updatesDE, "it": updatesIT, "hi": updatesHI} {
		for k, v := range msgs {
			if _, ok := catalogues[locale][k]; !ok {
				catalogues[locale][k] = v
			}
		}
	}
}

var updatesEN = map[string]string{
	"Mot de passe incorrect.": "Incorrect password.",
	"Cette installation ne se met pas à jour depuis l'interface : suivez les instructions de la page.": "This installation cannot be updated from the interface: follow the instructions on the page.",
	"Cette version n'est pas la dernière publiée : vérifiez à nouveau les mises à jour.":               "This is not the latest release. Check for updates again.",
	"Solstice est déjà à jour.":          "Solstice is already up to date.",
	"Une mise à jour est déjà en cours.": "An update is already in progress.",
}

var updatesDE = map[string]string{
	"Mot de passe incorrect.": "Falsches Passwort.",
	"Cette installation ne se met pas à jour depuis l'interface : suivez les instructions de la page.": "Diese Installation lässt sich nicht über die Oberfläche aktualisieren. Folgen Sie den Anweisungen auf der Seite.",
	"Cette version n'est pas la dernière publiée : vérifiez à nouveau les mises à jour.":               "Diese Version ist nicht die zuletzt veröffentlichte. Suchen Sie erneut nach Updates.",
	"Solstice est déjà à jour.":          "Solstice ist bereits auf dem neuesten Stand.",
	"Une mise à jour est déjà en cours.": "Es läuft bereits ein Update.",
}

var updatesIT = map[string]string{
	"Mot de passe incorrect.": "Password errata.",
	"Cette installation ne se met pas à jour depuis l'interface : suivez les instructions de la page.": "Questa installazione non si aggiorna dall'interfaccia: segui le istruzioni della pagina.",
	"Cette version n'est pas la dernière publiée : vérifiez à nouveau les mises à jour.":               "Questa non è l'ultima versione pubblicata: controlla di nuovo gli aggiornamenti.",
	"Solstice est déjà à jour.":          "Solstice è già aggiornato.",
	"Une mise à jour est déjà en cours.": "Un aggiornamento è già in corso.",
}

var updatesHI = map[string]string{
	"Mot de passe incorrect.": "पासवर्ड गलत है।",
	"Cette installation ne se met pas à jour depuis l'interface : suivez les instructions de la page.": "यह इंस्टॉलेशन इंटरफ़ेस से अपडेट नहीं होता। पेज पर दिए निर्देशों का पालन करें।",
	"Cette version n'est pas la dernière publiée : vérifiez à nouveau les mises à jour.":               "यह नवीनतम प्रकाशित वर्ज़न नहीं है। अपडेट दोबारा जाँचें।",
	"Solstice est déjà à jour.":          "Solstice पहले से अप-टू-डेट है।",
	"Une mise à jour est déjà en cours.": "एक अपडेट पहले से चल रहा है।",
}
