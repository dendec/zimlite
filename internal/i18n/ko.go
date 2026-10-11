package i18n

var koTranslations = map[string]string{
	"tree.load_error":           "문서 트리를 불러올 수 없습니다: %s",
	"settings.renderer":         "🖼️ 문서 렌더러",
	"renderer.full":             "전체",
	"renderer.simple":           "간단",
	"settings.renderer_note":    "메모리가 부족하거나 오류가 발생하면 전체 렌더러가 자동으로 간단 렌더러로 전환됩니다.",
	"settings.render_mode":      "⚡ 렌더링 모드",
	"render_mode.speed":         "속도",
	"render_mode.quality":       "화질",
	"settings.render_mode_note": "속도: 최대 640×480의 고정 해상도로 선명도를 낮추고 스크롤 속도를 높입니다. 화질: 화면 해상도를 사용하고 스크롤 중에는 해상도를 낮춥니다.",
	"servo.loading":             "전체 렌더러: 문서를 불러오는 중…",
	"servo.fallback":            "전체 렌더러를 사용할 수 없어 간단 렌더러를 사용합니다: %s",
	// Menu
	"menu.title":          "📖 메뉴",
	"menu.help":           "❓ 도움말",
	"menu.settings":       "⚙️ 설정",
	"menu.library":        "📥 아카이브 라이브러리",
	"menu.downloads":      "⬇️ 다운로드",
	"menu.stop":           "중지",
	"menu.start":          "시작",
	"menu.delete":         "삭제",
	"menu.delete_confirm": "%s을(를) 삭제할까요? 확인하려면 삭제를 다시 선택하세요. 뒤로 가면 취소됩니다.",
	"menu.archives":       "📦 아카이브",
	"menu.no_content":     "📭 현재 디렉토리에서 문서나 ZIM 아카이브를 찾을 수 없습니다.",

	// Help
	"help.title":        "❓ 도움말",
	"help.navigation":   "🧭 탐색",
	"help.scroll":       "📜 스크롤",
	"help.table_scroll": "↔️ 넓은 표 스크롤",
	"help.page_scroll":  "📄 페이지 위/아래",
	"help.open_link":    "🔗 링크 열기",
	"help.go_back":      "🔙 뒤로",
	"help.main_menu":    "🏠 메인 메뉴",
	"help.toc":          "📑 목차",
	"help.zoom":         "🔍 확대/축소",
	"help.help":         "❓ 도움말",
	"help.settings":     "⚙️ 설정",
	"help.exit":         "🚪 종료",

	// Settings
	"settings.title":     "⚙️ 설정",
	"settings.theme":     "🎨 테마",
	"settings.fontsize":  "🔤 글꼴 크기",
	"settings.language":  "🌐 언어",
	"settings.save_note": "💾 설정이 자동으로 저장되며 다음 시작 시 적용됩니다.",

	// Theme names
	"theme.light": "☀️ 라이트",
	"theme.dark":  "🌙 다크",
	"theme.sepia": "📜 세피아",

	// Library
	"library.title_languages":  "🌐 아카이브 라이브러리 - 언어",
	"library.select_language":  "👇 언어 선택:",
	"library.back":             "🔙 뒤로",
	"library.title_categories": "🌐 아카이브 라이브러리 -",
	"library.select_category":  "👇 카테고리 선택:",
	"library.title_entries":    "🌐 아카이브 라이브러리 -",
	"library.back_categories":  "🔙 카테고리로 돌아가기",
	"library.no_entries":       "📭 이 페이지에 더 이상 아카이브가 없습니다.",
	"library.no_archives":      "📭 이 언어와 카테고리에 아카이브가 없습니다.",
	"library.download":         "다운로드",
	"library.prev_page":        "◀ 이전 페이지",
	"library.next_page":        "다음 페이지 ▶",
	"library.archives_count":   "%d개 아카이브",

	// Library errors
	"library.section_languages":  "언어",
	"library.section_categories": "카테고리",
	"library.section_archives":   "아카이브",
	"library.error_loading":      "❌ %s 로딩 오류",
	"library.error_message":      "Kiwix 라이브러리 카탈로그와 통신 중 오류가 발생했습니다:\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 메뉴로 돌아가기](virtual:menu)",

	// Download status
	"download.connecting": "⏳ 연결 중... %s",
	"download.retry":      "⏳ 연결이 끊어졌습니다. 재시도 중... (%d/5)",
	"download.stopped":    "🛑 다운로드 중지됨",
	"download.failed":     "❌ 다운로드 실패: %s",
	"download.finished":   "✅ 다운로드가 성공적으로 완료되었습니다!",
	"download.progress":   "⬇ %s 다운로드 중: %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 문서 트리",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
