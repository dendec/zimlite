package i18n

var frTranslations = map[string]string{
	"tree.load_error":           "Impossible de charger l’arborescence des articles : %s",
	"settings.renderer":         "🖼️ Rendu des articles",
	"renderer.full":             "Complet",
	"renderer.simple":           "Simple",
	"settings.renderer_note":    "Le rendu complet passe automatiquement au rendu simple si la mémoire est insuffisante ou en cas d’erreur.",
	"settings.render_mode":      "⚡ Mode de rendu",
	"render_mode.speed":         "Vitesse",
	"render_mode.quality":       "Qualité",
	"settings.render_mode_note": "Vitesse : résolution fixe jusqu’à 640×480, défilement plus rapide avec moins de netteté. Qualité : résolution de l’écran, réduite pendant le défilement.",
	"servo.loading":             "Rendu complet : chargement de l’article…",
	"servo.fallback":            "Rendu complet indisponible ; utilisation du rendu simple : %s",
	// Menu
	"menu.title":          "📖 Menu",
	"menu.help":           "❓ Aide",
	"menu.settings":       "⚙️ Paramètres",
	"menu.library":        "📥 Bibliothèque d'archives",
	"menu.downloads":      "⬇️ Téléchargements",
	"menu.stop":           "Arrêter",
	"menu.start":          "Démarrer",
	"menu.delete":         "Supprimer",
	"menu.delete_confirm": "Supprimer %s ? Sélectionnez à nouveau Supprimer pour confirmer ; Retour annule.",
	"menu.archives":       "📦 Archives",
	"menu.no_content":     "📭 Aucun document ou archive ZIM trouvé dans le répertoire actuel.",

	// Help
	"help.title":        "❓ Aide",
	"help.navigation":   "🧭 Navigation",
	"help.scroll":       "📜 Défiler",
	"help.table_scroll": "↔️ Faire défiler un tableau large",
	"help.page_scroll":  "📄 Page précédente/suivante",
	"help.open_link":    "🔗 Ouvrir le lien",
	"help.go_back":      "🔙 Retour",
	"help.main_menu":    "🏠 Menu principal",
	"help.toc":          "📑 Table des matières",
	"help.zoom":         "🔍 Zoom avant/arrière",
	"help.help":         "❓ Aide",
	"help.settings":     "⚙️ Paramètres",
	"help.exit":         "🚪 Quitter",

	// Settings
	"settings.title":     "⚙️ Paramètres",
	"settings.theme":     "🎨 Thème",
	"settings.fontsize":  "🔤 Taille de police",
	"settings.language":  "🌐 Langue",
	"settings.save_note": "💾 Les paramètres sont sauvegardés automatiquement et seront appliqués au prochain démarrage.",

	// Theme names
	"theme.light": "☀️ Clair",
	"theme.dark":  "🌙 Sombre",
	"theme.sepia": "📜 Sépia",

	// Library
	"library.title_languages":  "🌐 Bibliothèque d'archives - Langues",
	"library.select_language":  "👇 Sélectionnez une langue :",
	"library.back":             "🔙 Retour",
	"library.title_categories": "🌐 Bibliothèque d'archives -",
	"library.select_category":  "👇 Sélectionnez une catégorie :",
	"library.title_entries":    "🌐 Bibliothèque d'archives -",
	"library.back_categories":  "🔙 Retour aux catégories",
	"library.no_entries":       "📭 Plus aucune archive sur cette page.",
	"library.no_archives":      "📭 Aucune archive trouvée dans cette langue et catégorie.",
	"library.download":         "Télécharger",
	"library.prev_page":        "◀ Page précédente",
	"library.next_page":        "Page suivante ▶",
	"library.archives_count":   "%d archives",

	// Library errors
	"library.section_languages":  "langues",
	"library.section_categories": "catégories",
	"library.section_archives":   "archives",
	"library.error_loading":      "❌ Erreur lors du chargement de %s",
	"library.error_message":      "Une erreur s'est produite lors de la communication avec le catalogue de la bibliothèque Kiwix :\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 Retour au menu](virtual:menu)",

	// Download status
	"download.connecting": "⏳ Connexion... %s",
	"download.retry":      "⏳ Connexion perdue. Nouvelle tentative... (%d/5)",
	"download.stopped":    "🛑 Téléchargement arrêté",
	"download.failed":     "❌ Échec du téléchargement : %s",
	"download.finished":   "✅ Téléchargement terminé avec succès !",
	"download.progress":   "⬇ Téléchargement de %s : %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 Arborescence des articles",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
