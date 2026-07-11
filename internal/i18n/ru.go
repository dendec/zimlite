package i18n

var ruTranslations = map[string]string{
	// Menu
	"menu.title":      "📖 Меню",
	"menu.help":       "❓ Помощь",
	"menu.settings":   "⚙️ Настройки",
	"menu.library":    "📥 Библиотека архивов",
	"menu.downloads":  "⬇️ Загрузки",
	"menu.stop":       "Стоп",
	"menu.start":      "Старт",
	"menu.delete":     "Удалить",
	"menu.archives":   "📦 Архивы",
	"menu.no_content": "📭 Документы или ZIM-архивы не найдены в текущей директории.",

	// Help
	"help.title":      "❓ Помощь",
	"help.navigation": "🧭 Навигация",
	"help.scroll":     "📜 Прокрутка",
	"help.open_link":  "🔗 Открыть ссылку",
	"help.go_back":    "🔙 Назад",
	"help.main_menu":  "🏠 Главное меню",
	"help.toc":        "📑 Оглавление",
	"help.zoom":       "🔍 Масштаб",
	"help.help":       "❓ Помощь",
	"help.settings":   "⚙️ Настройки",
	"help.exit":       "🚪 Выход",

	// Settings
	"settings.title":     "⚙️ Настройки",
	"settings.theme":     "🎨 Тема",
	"settings.fontsize":  "🔤 Размер шрифта",
	"settings.language":  "🌐 Язык",
	"settings.save_note": "💾 Настройки сохраняются автоматически и применятся при следующем запуске.",

	// Theme names
	"theme.light": "☀️ Светлая",
	"theme.dark":  "🌙 Тёмная",
	"theme.sepia": "📜 Сепия",

	// Library
	"library.title_languages":  "🌐 Библиотека архивов - Языки",
	"library.select_language":  "👇 Выберите язык:",
	"library.back":             "🔙 Назад",
	"library.title_categories": "🌐 Библиотека архивов -",
	"library.select_category":  "👇 Выберите категорию:",
	"library.title_entries":    "🌐 Библиотека архивов -",
	"library.back_categories":  "🔙 Назад к категориям",
	"library.no_entries":       "📭 На этой странице больше нет архивов.",
	"library.no_archives":      "📭 Архивы не найдены для этого языка и категории.",
	"library.download":         "Скачать",
	"library.prev_page":        "◀ Предыдущая страница",
	"library.next_page":        "Следующая страница ▶",
	"library.archives_count":   "%d архивов",

	// Library errors
	"library.section_languages":  "языки",
	"library.section_categories": "категории",
	"library.section_archives":   "архивы",
	"library.error_loading":      "❌ Ошибка загрузки %s",
	"library.error_message":      "Произошла ошибка при обращении к каталогу библиотеки Kiwix:\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 Назад в меню](virtual:menu)",

	// Download status
	"download.connecting": "⏳ Подключение... %s",
	"download.retry":      "⏳ Соединение потеряно. Повтор... (%d/5)",
	"download.stopped":    "🛑 Загрузка остановлена",
	"download.failed":     "❌ Ошибка загрузки: %s",
	"download.finished":   "✅ Загрузка успешно завершена!",
	"download.progress":   "⬇ Загрузка %s: %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 Дерево статей",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%%  ·  🔗 %d/%d",
}
