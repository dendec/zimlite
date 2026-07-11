package i18n

var ukTranslations = map[string]string{
	// Menu
	"menu.title":      "📖 Меню",
	"menu.help":       "❓ Довідка",
	"menu.settings":   "⚙️ Налаштування",
	"menu.library":    "📥 Бібліотека архівів",
	"menu.downloads":  "⬇️ Завантаження",
	"menu.stop":       "Стоп",
	"menu.start":      "Старт",
	"menu.delete":     "Видалити",
	"menu.archives":   "📦 Архіви",
	"menu.no_content": "📭 Не знайдено документів або ZIM-архівів у поточній директорії.",

	// Help
	"help.title":      "❓ Довідка",
	"help.navigation": "🧭 Навігація",
	"help.scroll":     "📜 Прокрутка",
	"help.open_link":  "🔗 Відкрити посилання",
	"help.go_back":    "🔙 Назад",
	"help.main_menu":  "🏠 Головне меню",
	"help.toc":        "📑 Зміст",
	"help.zoom":       "🔍 Масштаб",
	"help.help":       "❓ Довідка",
	"help.settings":   "⚙️ Налаштування",
	"help.exit":       "🚪 Вихід",

	// Settings
	"settings.title":     "⚙️ Налаштування",
	"settings.theme":     "🎨 Тема",
	"settings.fontsize":  "🔤 Розмір шрифту",
	"settings.language":  "🌐 Мова",
	"settings.save_note": "💾 Налаштування зберігаються автоматично і застосуються при наступному запуску.",

	// Theme names
	"theme.light": "☀️ Світла",
	"theme.dark":  "🌙 Темна",
	"theme.sepia": "📜 Сепія",

	// Library
	"library.title_languages":  "🌐 Бібліотека архівів - Мови",
	"library.select_language":  "👇 Виберіть мову:",
	"library.back":             "🔙 Назад",
	"library.title_categories": "🌐 Бібліотека архівів -",
	"library.select_category":  "👇 Виберіть категорію:",
	"library.title_entries":    "🌐 Бібліотека архівів -",
	"library.back_categories":  "🔙 Назад до категорій",
	"library.no_entries":       "📭 На цій сторінці більше немає архівів.",
	"library.no_archives":      "📭 Не знайдено архівів для цієї мови та категорії.",
	"library.download":         "Завантажити",
	"library.prev_page":        "◀ Попередня сторінка",
	"library.next_page":        "Наступна сторінка ▶",
	"library.archives_count":   "%d архівів",

	// Library errors
	"library.section_languages":  "мови",
	"library.section_categories": "категорії",
	"library.section_archives":   "архіви",
	"library.error_loading":      "❌ Помилка завантаження %s",
	"library.error_message":      "Сталася помилка при зверненні до каталогу бібліотеки Kiwix:\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 Назад до меню](virtual:menu)",

	// Download status
	"download.connecting": "⏳ Підключення... %s",
	"download.retry":      "⏳ З'єднання втрачено. Повтор... (%d/5)",
	"download.stopped":    "🛑 Завантаження зупинено",
	"download.failed":     "❌ Помилка завантаження: %s",
	"download.finished":   "✅ Завантаження успішно завершено!",
	"download.progress":   "⬇ Завантаження %s: %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 Дерево статей",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
