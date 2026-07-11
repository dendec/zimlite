package i18n

var zhTranslations = map[string]string{
	// Menu
	"menu.title":      "📖 菜单",
	"menu.help":       "❓ 帮助",
	"menu.settings":   "⚙️ 设置",
	"menu.library":    "📥 存档库",
	"menu.downloads":  "⬇️ 下载",
	"menu.stop":       "停止",
	"menu.start":      "开始",
	"menu.delete":     "删除",
	"menu.archives":   "📦 档案",
	"menu.no_content": "📭 当前目录未找到文档或ZIM档案。",

	// Help
	"help.title":      "❓ 帮助",
	"help.navigation": "🧭 导航",
	"help.scroll":     "📜 滚动页面",
	"help.open_link":  "🔗 打开链接",
	"help.go_back":    "🔙 返回",
	"help.main_menu":  "🏠 主菜单",
	"help.toc":        "📑 目录",
	"help.zoom":       "🔍 放大/缩小",
	"help.help":       "❓ 帮助",
	"help.settings":   "⚙️ 设置",
	"help.exit":       "🚪 退出",

	// Settings
	"settings.title":     "⚙️ 设置",
	"settings.theme":     "🎨 主题",
	"settings.fontsize":  "🔤 字体大小",
	"settings.language":  "🌐 语言",
	"settings.save_note": "💾 设置会自动保存，下次启动时生效。",

	// Theme names
	"theme.light": "☀️ 浅色",
	"theme.dark":  "🌙 深色",
	"theme.sepia": "📜 棕褐色",

	// Library
	"library.title_languages":  "🌐 存档库 - 语言",
	"library.select_language":  "👇 选择语言：",
	"library.back":             "🔙 返回",
	"library.title_categories": "🌐 存档库 -",
	"library.select_category":  "👇 选择分类：",
	"library.title_entries":    "🌐 存档库 -",
	"library.back_categories":  "🔙 返回分类",
	"library.no_entries":       "📭 此页面没有更多档案。",
	"library.no_archives":      "📭 未找到该语言和分类的档案。",
	"library.download":         "下载",
	"library.prev_page":        "◀ 上一页",
	"library.next_page":        "下一页 ▶",
	"library.archives_count":   "%d 个档案",

	// Library errors
	"library.section_languages":  "语言",
	"library.section_categories": "分类",
	"library.section_archives":   "档案",
	"library.error_loading":      "❌ 加载%s时出错",
	"library.error_message":      "与Kiwix存档目录通信时出错：\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 返回菜单](virtual:menu)",

	// Download status
	"download.connecting": "⏳ 连接中... %s",
	"download.retry":      "⏳ 连接断开。重试中... (%d/5)",
	"download.stopped":    "🛑 下载已停止",
	"download.failed":     "❌ 下载失败：%s",
	"download.finished":   "✅ 下载成功完成！",
	"download.progress":   "⬇ 正在下载 %s：%.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 文章树",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
