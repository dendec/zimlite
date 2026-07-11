package i18n

var deTranslations = map[string]string{
	// Menu
	"menu.title":      "📖 Menü",
	"menu.help":       "❓ Hilfe",
	"menu.settings":   "⚙️ Einstellungen",
	"menu.library":    "📥 Archiv-Bibliothek",
	"menu.downloads":  "⬇️ Downloads",
	"menu.stop":       "Stopp",
	"menu.start":      "Start",
	"menu.delete":     "Löschen",
	"menu.archives":   "📦 Archive",
	"menu.no_content": "📭 Keine Dokumente oder ZIM-Archive im aktuellen Verzeichnis gefunden.",

	// Help
	"help.title":      "❓ Hilfe",
	"help.navigation": "🧭 Navigation",
	"help.scroll":     "📜 Seite scrollen",
	"help.open_link":  "🔗 Link öffnen",
	"help.go_back":    "🔙 Zurück",
	"help.main_menu":  "🏠 Hauptmenü",
	"help.toc":        "📑 Inhaltsverzeichnis",
	"help.zoom":       "🔍 Vergrößern/Verkleinern",
	"help.help":       "❓ Hilfe",
	"help.settings":   "⚙️ Einstellungen",
	"help.exit":       "🚪 Beenden",

	// Settings
	"settings.title":     "⚙️ Einstellungen",
	"settings.theme":     "🎨 Design",
	"settings.fontsize":  "🔤 Schriftgröße",
	"settings.language":  "🌐 Sprache",
	"settings.save_note": "💾 Einstellungen werden automatisch gespeichert und beim nächsten Start übernommen.",

	// Theme names
	"theme.light": "☀️ Hell",
	"theme.dark":  "🌙 Dunkel",
	"theme.sepia": "📜 Sepia",

	// Library
	"library.title_languages":  "🌐 Archiv-Bibliothek - Sprachen",
	"library.select_language":  "👇 Wählen Sie eine Sprache:",
	"library.back":             "🔙 Zurück",
	"library.title_categories": "🌐 Archiv-Bibliothek -",
	"library.select_category":  "👇 Wählen Sie eine Kategorie:",
	"library.title_entries":    "🌐 Archiv-Bibliothek -",
	"library.back_categories":  "🔙 Zurück zu Kategorien",
	"library.no_entries":       "📭 Keine weiteren Archive auf dieser Seite.",
	"library.no_archives":      "📭 Keine Archive in dieser Sprache und Kategorie gefunden.",
	"library.download":         "Herunterladen",
	"library.prev_page":        "◀ Vorherige Seite",
	"library.next_page":        "Nächste Seite ▶",
	"library.archives_count":   "%d Archive",

	// Library errors
	"library.section_languages":  "Sprachen",
	"library.section_categories": "Kategorien",
	"library.section_archives":   "Archive",
	"library.error_loading":      "❌ Fehler beim Laden von %s",
	"library.error_message":      "Bei der Kommunikation mit dem Kiwix-Bibliothekskatalog ist ein Fehler aufgetreten:\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 Zurück zum Menü](virtual:menu)",

	// Download status
	"download.connecting": "⏳ Verbinde... %s",
	"download.retry":      "⏳ Verbindung verloren. Erneuter Versuch... (%d/5)",
	"download.stopped":    "🛑 Download gestoppt",
	"download.failed":     "❌ Download fehlgeschlagen: %s",
	"download.finished":   "✅ Download erfolgreich abgeschlossen!",
	"download.progress":   "⬇ Lade %s herunter: %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 Artikelbaum",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
