package i18n

var itTranslations = map[string]string{
	// Menu
	"menu.title":      "📖 Menu",
	"menu.help":       "❓ Aiuto",
	"menu.settings":   "⚙️ Impostazioni",
	"menu.library":    "📥 Libreria archivi",
	"menu.downloads":  "⬇️ Download",
	"menu.stop":       "Ferma",
	"menu.start":      "Avvia",
	"menu.delete":     "Elimina",
	"menu.archives":   "📦 Archivi",
	"menu.no_content": "📭 Nessun documento o archivio ZIM trovato nella directory corrente.",

	// Help
	"help.title":      "❓ Aiuto",
	"help.navigation": "🧭 Navigazione",
	"help.scroll":     "📜 Scorri pagina",
	"help.open_link":  "🔗 Apri link",
	"help.go_back":    "🔙 Indietro",
	"help.main_menu":  "🏠 Menu principale",
	"help.toc":        "📑 Indice",
	"help.zoom":       "🔍 Ingrandisci/Riduci",
	"help.help":       "❓ Aiuto",
	"help.settings":   "⚙️ Impostazioni",
	"help.exit":       "🚪 Esci",

	// Settings
	"settings.title":     "⚙️ Impostazioni",
	"settings.theme":     "🎨 Tema",
	"settings.fontsize":  "🔤 Dimensione carattere",
	"settings.language":  "🌐 Lingua",
	"settings.save_note": "💾 Le impostazioni vengono salvate automaticamente e applicate al prossimo avvio.",

	// Theme names
	"theme.light": "☀️ Chiaro",
	"theme.dark":  "🌙 Scuro",
	"theme.sepia": "📜 Sepia",

	// Library
	"library.title_languages":  "🌐 Libreria archivi - Lingue",
	"library.select_language":  "👇 Seleziona una lingua:",
	"library.back":             "🔙 Indietro",
	"library.title_categories": "🌐 Libreria archivi -",
	"library.select_category":  "👇 Seleziona una categoria:",
	"library.title_entries":    "🌐 Libreria archivi -",
	"library.back_categories":  "🔙 Torna alle categorie",
	"library.no_entries":       "📭 Nessun altro archivio in questa pagina.",
	"library.no_archives":      "📭 Nessun archivio trovato per questa lingua e categoria.",
	"library.download":         "Scarica",
	"library.prev_page":        "◀ Pagina precedente",
	"library.next_page":        "Pagina successiva ▶",
	"library.archives_count":   "%d archivi",

	// Library errors
	"library.section_languages":  "lingue",
	"library.section_categories": "categorie",
	"library.section_archives":   "archivi",
	"library.error_loading":      "❌ Errore durante il caricamento di %s",
	"library.error_message":      "Si è verificato un errore durante la comunicazione con il catalogo della libreria Kiwix:\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 Torna al menu](virtual:menu)",

	// Download status
	"download.connecting": "⏳ Connessione... %s",
	"download.retry":      "⏳ Connessione persa. Riprovo... (%d/5)",
	"download.stopped":    "🛑 Download interrotto",
	"download.failed":     "❌ Download fallito: %s",
	"download.finished":   "✅ Download completato con successo!",
	"download.progress":   "⬇ Download di %s: %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 Albero degli articoli",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
