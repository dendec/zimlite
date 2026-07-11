package i18n

var enTranslations = map[string]string{
	// Menu
	"menu.title":      "📖 Menu",
	"menu.help":       "❓ Help",
	"menu.settings":   "⚙️ Settings",
	"menu.library":    "📥 Archive Library",
	"menu.downloads":  "⬇️ Downloads",
	"menu.stop":       "Stop",
	"menu.start":      "Start",
	"menu.delete":     "Delete",
	"menu.archives":   "📦 Archives",
	"menu.no_content": "📭 No documents or ZIM archives found in current directory.",

	// Help
	"help.title":      "❓ Help",
	"help.navigation": "🧭 Navigation",
	"help.scroll":     "📜 Scroll Page",
	"help.open_link":  "🔗 Open Link",
	"help.go_back":    "🔙 Go Back",
	"help.main_menu":  "🏠 Main Menu",
	"help.toc":        "📑 Table of Contents",
	"help.zoom":       "🔍 Zoom In/Out",
	"help.help":       "❓ Help",
	"help.settings":   "⚙️ Settings",
	"help.exit":       "🚪 Exit",

	// Settings
	"settings.title":     "⚙️ Settings",
	"settings.theme":     "🎨 Theme",
	"settings.fontsize":  "🔤 Font Size",
	"settings.language":  "🌐 Language",
	"settings.save_note": "💾 Settings are saved automatically and will be applied on the next startup.",

	// Theme names
	"theme.light": "☀️ Light",
	"theme.dark":  "🌙 Dark",
	"theme.sepia": "📜 Sepia",

	// Library
	"library.title_languages":  "🌐 Archive Library - Languages",
	"library.select_language":  "👇 Select a language:",
	"library.back":             "🔙 Back",
	"library.title_categories": "🌐 Archive Library -",
	"library.select_category":  "👇 Select a category:",
	"library.title_entries":    "🌐 Archive Library -",
	"library.back_categories":  "🔙 Back to Categories",
	"library.no_entries":       "📭 No more archives on this page.",
	"library.no_archives":      "📭 No archives found in this language and category.",
	"library.download":         "Download",
	"library.prev_page":        "◀ Previous Page",
	"library.next_page":        "Next Page ▶",
	"library.archives_count":   "%d archives",

	// Library errors
	"library.section_languages":  "languages",
	"library.section_categories": "categories",
	"library.section_archives":   "archives",
	"library.error_loading":      "❌ Error loading %s",
	"library.error_message":      "An error occurred while communicating with the Kiwix library catalog:\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 Back to Menu](virtual:menu)",

	// Download status
	"download.connecting": "⏳ Connecting... %s",
	"download.retry":      "⏳ Connection lost. Retrying... (%d/5)",
	"download.stopped":    "🛑 Download stopped",
	"download.failed":     "❌ Download failed: %s",
	"download.finished":   "✅ Download finished successfully!",
	"download.progress":   "⬇ Downloading %s: %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 Article tree",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%%  ·  🔗 %d/%d",
}
