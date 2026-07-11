package i18n

var ptTranslations = map[string]string{
	// Menu
	"menu.title":      "📖 Menu",
	"menu.help":       "❓ Ajuda",
	"menu.settings":   "⚙️ Configurações",
	"menu.library":    "📥 Biblioteca de arquivos",
	"menu.downloads":  "⬇️ Downloads",
	"menu.stop":       "Parar",
	"menu.start":      "Iniciar",
	"menu.delete":     "Excluir",
	"menu.archives":   "📦 Arquivos",
	"menu.no_content": "📭 Nenhum documento ou arquivo ZIM encontrado no diretório atual.",

	// Help
	"help.title":      "❓ Ajuda",
	"help.navigation": "🧭 Navegação",
	"help.scroll":     "📜 Rolar página",
	"help.open_link":  "🔗 Abrir link",
	"help.go_back":    "🔙 Voltar",
	"help.main_menu":  "🏠 Menu principal",
	"help.toc":        "📑 Índice",
	"help.zoom":       "🔍 Ampliar/Reduzir",
	"help.help":       "❓ Ajuda",
	"help.settings":   "⚙️ Configurações",
	"help.exit":       "🚪 Sair",

	// Settings
	"settings.title":     "⚙️ Configurações",
	"settings.theme":     "🎨 Tema",
	"settings.fontsize":  "🔤 Tamanho da fonte",
	"settings.language":  "🌐 Idioma",
	"settings.save_note": "💾 As configurações são salvas automaticamente e aplicadas na próxima inicialização.",

	// Theme names
	"theme.light": "☀️ Claro",
	"theme.dark":  "🌙 Escuro",
	"theme.sepia": "📜 Sépia",

	// Library
	"library.title_languages":  "🌐 Biblioteca de arquivos - Idiomas",
	"library.select_language":  "👇 Selecione um idioma:",
	"library.back":             "🔙 Voltar",
	"library.title_categories": "🌐 Biblioteca de arquivos -",
	"library.select_category":  "👇 Selecione uma categoria:",
	"library.title_entries":    "🌐 Biblioteca de arquivos -",
	"library.back_categories":  "🔙 Voltar para categorias",
	"library.no_entries":       "📭 Não há mais arquivos nesta página.",
	"library.no_archives":      "📭 Nenhum arquivo encontrado neste idioma e categoria.",
	"library.download":         "Baixar",
	"library.prev_page":        "◀ Página anterior",
	"library.next_page":        "Próxima página ▶",
	"library.archives_count":   "%d arquivos",

	// Library errors
	"library.section_languages":  "idiomas",
	"library.section_categories": "categorias",
	"library.section_archives":   "arquivos",
	"library.error_loading":      "❌ Erro ao carregar %s",
	"library.error_message":      "Ocorreu um erro ao comunicar com o catálogo da biblioteca Kiwix:\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 Voltar ao menu](virtual:menu)",

	// Download status
	"download.connecting": "⏳ Conectando... %s",
	"download.retry":      "⏳ Conexão perdida. Tentando novamente... (%d/5)",
	"download.stopped":    "🛑 Download interrompido",
	"download.failed":     "❌ Falha no download: %s",
	"download.finished":   "✅ Download concluído com sucesso!",
	"download.progress":   "⬇ Baixando %s: %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 Árvore de artigos",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
