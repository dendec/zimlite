package i18n

var esTranslations = map[string]string{
	"tree.load_error":        "No se pudo cargar el árbol de artículos: %s",
	"settings.renderer":      "🖼️ Renderizador de artículos",
	"renderer.full":          "Completo",
	"renderer.simple":        "Simple",
	"settings.renderer_note": "El renderizador completo cambia automáticamente al simple si falta memoria o se produce un error.",
	"servo.loading":          "Renderizador completo: cargando artículo…",
	"servo.fallback":         "Renderizador completo no disponible; usando el simple: %s",
	// Menu
	"menu.title":          "📖 Menú",
	"menu.help":           "❓ Ayuda",
	"menu.settings":       "⚙️ Configuración",
	"menu.library":        "📥 Biblioteca de archivos",
	"menu.downloads":      "⬇️ Descargas",
	"menu.stop":           "Detener",
	"menu.start":          "Iniciar",
	"menu.delete":         "Eliminar",
	"menu.delete_confirm": "¿Eliminar %s? Selecciona Eliminar otra vez para confirmar; Atrás cancela.",
	"menu.archives":       "📦 Archivos",
	"menu.no_content":     "📭 No se encontraron documentos o archivos ZIM en el directorio actual.",

	// Help
	"help.title":      "❓ Ayuda",
	"help.navigation": "🧭 Navegación",
	"help.scroll":     "📜 Desplazarse",
	"help.open_link":  "🔗 Abrir enlace",
	"help.go_back":    "🔙 Volver",
	"help.main_menu":  "🏠 Menú principal",
	"help.toc":        "📑 Índice",
	"help.zoom":       "🔍 Acercar/Alejar",
	"help.help":       "❓ Ayuda",
	"help.settings":   "⚙️ Configuración",
	"help.exit":       "🚪 Salir",

	// Settings
	"settings.title":     "⚙️ Configuración",
	"settings.theme":     "🎨 Tema",
	"settings.fontsize":  "🔤 Tamaño de fuente",
	"settings.language":  "🌐 Idioma",
	"settings.save_note": "💾 La configuración se guarda automáticamente y se aplicará al próximo inicio.",

	// Theme names
	"theme.light": "☀️ Claro",
	"theme.dark":  "🌙 Oscuro",
	"theme.sepia": "📜 Sepia",

	// Library
	"library.title_languages":  "🌐 Biblioteca de archivos - Idiomas",
	"library.select_language":  "👇 Seleccione un idioma:",
	"library.back":             "🔙 Volver",
	"library.title_categories": "🌐 Biblioteca de archivos -",
	"library.select_category":  "👇 Seleccione una categoría:",
	"library.title_entries":    "🌐 Biblioteca de archivos -",
	"library.back_categories":  "🔙 Volver a categorías",
	"library.no_entries":       "📭 No hay más archivos en esta página.",
	"library.no_archives":      "📭 No se encontraron archivos en este idioma y categoría.",
	"library.download":         "Descargar",
	"library.prev_page":        "◀ Página anterior",
	"library.next_page":        "Página siguiente ▶",
	"library.archives_count":   "%d archivos",

	// Library errors
	"library.section_languages":  "idiomas",
	"library.section_categories": "categorías",
	"library.section_archives":   "archivos",
	"library.error_loading":      "❌ Error al cargar %s",
	"library.error_message":      "Ocurrió un error al comunicarse con el catálogo de la biblioteca Kiwix:\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 Volver al menú](virtual:menu)",

	// Download status
	"download.connecting": "⏳ Conectando... %s",
	"download.retry":      "⏳ Conexión perdida. Reintentando... (%d/5)",
	"download.stopped":    "🛑 Descarga detenida",
	"download.failed":     "❌ Error en la descarga: %s",
	"download.finished":   "✅ ¡Descarga completada con éxito!",
	"download.progress":   "⬇ Descargando %s: %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 Árbol de artículos",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
