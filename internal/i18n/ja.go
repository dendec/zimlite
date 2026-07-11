package i18n

var jaTranslations = map[string]string{
	// Menu
	"menu.title":      "📖 メニュー",
	"menu.help":       "❓ ヘルプ",
	"menu.settings":   "⚙️ 設定",
	"menu.library":    "📥 アーカイブライブラリ",
	"menu.downloads":  "⬇️ ダウンロード",
	"menu.stop":       "停止",
	"menu.start":      "開始",
	"menu.delete":     "削除",
	"menu.archives":   "📦 アーカイブ",
	"menu.no_content": "📭 現在のディレクトリにドキュメントまたはZIMアーカイブが見つかりません。",

	// Help
	"help.title":      "❓ ヘルプ",
	"help.navigation": "🧭 ナビゲーション",
	"help.scroll":     "📜 スクロール",
	"help.open_link":  "🔗 リンクを開く",
	"help.go_back":    "🔙 戻る",
	"help.main_menu":  "🏠 メインメニュー",
	"help.toc":        "📑 目次",
	"help.zoom":       "🔍 拡大/縮小",
	"help.help":       "❓ ヘルプ",
	"help.settings":   "⚙️ 設定",
	"help.exit":       "🚪 終了",

	// Settings
	"settings.title":     "⚙️ 設定",
	"settings.theme":     "🎨 テーマ",
	"settings.fontsize":  "🔤 フォントサイズ",
	"settings.language":  "🌐 言語",
	"settings.save_note": "💾 設定は自動的に保存され、次回起動時に適用されます。",

	// Theme names
	"theme.light": "☀️ ライト",
	"theme.dark":  "🌙 ダーク",
	"theme.sepia": "📜 セピア",

	// Library
	"library.title_languages":  "🌐 アーカイブライブラリ - 言語",
	"library.select_language":  "👇 言語を選択：",
	"library.back":             "🔙 戻る",
	"library.title_categories": "🌐 アーカイブライブラリ -",
	"library.select_category":  "👇 カテゴリを選択：",
	"library.title_entries":    "🌐 アーカイブライブラリ -",
	"library.back_categories":  "🔙 カテゴリに戻る",
	"library.no_entries":       "📭 このページにはこれ以上アーカイブがありません。",
	"library.no_archives":      "📭 この言語とカテゴリにはアーカイブが見つかりません。",
	"library.download":         "ダウンロード",
	"library.prev_page":        "◀ 前のページ",
	"library.next_page":        "次のページ ▶",
	"library.archives_count":   "%d アーカイブ",

	// Library errors
	"library.section_languages":  "言語",
	"library.section_categories": "カテゴリ",
	"library.section_archives":   "アーカイブ",
	"library.error_loading":      "❌ %sの読み込みエラー",
	"library.error_message":      "Kiwixライブラリカタログとの通信中にエラーが発生しました：\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 メニューに戻る](virtual:menu)",

	// Download status
	"download.connecting": "⏳ 接続中... %s",
	"download.retry":      "⏳ 接続が切れました。再試行中... (%d/5)",
	"download.stopped":    "🛑 ダウンロードを停止しました",
	"download.failed":     "❌ ダウンロード失敗：%s",
	"download.finished":   "✅ ダウンロードが正常に完了しました！",
	"download.progress":   "⬇ %sをダウンロード中：%.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 記事ツリー",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
