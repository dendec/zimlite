package i18n

var trTranslations = map[string]string{
	// Menu
	"menu.title":      "📖 Menü",
	"menu.help":       "❓ Yardım",
	"menu.settings":   "⚙️ Ayarlar",
	"menu.library":    "📥 Arşiv Kütüphanesi",
	"menu.downloads":  "⬇️ İndirilenler",
	"menu.stop":       "Durdur",
	"menu.start":      "Başlat",
	"menu.delete":     "Sil",
	"menu.archives":   "📦 Arşivler",
	"menu.no_content": "📭 Geçerli dizinde belge veya ZIM arşivi bulunamadı.",

	// Help
	"help.title":      "❓ Yardım",
	"help.navigation": "🧭 Gezinme",
	"help.scroll":     "📜 Sayfayı kaydır",
	"help.open_link":  "🔗 Bağlantıyı aç",
	"help.go_back":    "🔙 Geri",
	"help.main_menu":  "🏠 Ana menü",
	"help.toc":        "📑 İçindekiler",
	"help.zoom":       "🔍 Yakınlaştır/Uzaklaştır",
	"help.help":       "❓ Yardım",
	"help.settings":   "⚙️ Ayarlar",
	"help.exit":       "🚪 Çıkış",

	// Settings
	"settings.title":     "⚙️ Ayarlar",
	"settings.theme":     "🎨 Tema",
	"settings.fontsize":  "🔤 Yazı boyutu",
	"settings.language":  "🌐 Dil",
	"settings.save_note": "💾 Ayarlar otomatik olarak kaydedilir ve bir sonraki başlatışta uygulanır.",

	// Theme names
	"theme.light": "☀️ Açık",
	"theme.dark":  "🌙 Koyu",
	"theme.sepia": "📜 Sepya",

	// Library
	"library.title_languages":  "🌐 Arşiv Kütüphanesi - Diller",
	"library.select_language":  "👇 Bir dil seçin:",
	"library.back":             "🔙 Geri",
	"library.title_categories": "🌐 Arşiv Kütüphanesi -",
	"library.select_category":  "👇 Bir kategori seçin:",
	"library.title_entries":    "🌐 Arşiv Kütüphanesi -",
	"library.back_categories":  "🔙 Kategorilere dön",
	"library.no_entries":       "📭 Bu sayfada başka arşiv yok.",
	"library.no_archives":      "📭 Bu dil ve kategoride arşiv bulunamadı.",
	"library.download":         "İndir",
	"library.prev_page":        "◀ Önceki sayfa",
	"library.next_page":        "Sonraki sayfa ▶",
	"library.archives_count":   "%d arşiv",

	// Library errors
	"library.section_languages":  "diller",
	"library.section_categories": "kategoriler",
	"library.section_archives":   "arşivler",
	"library.error_loading":      "❌ %s yüklenirken hata",
	"library.error_message":      "Kiwix kütüphane kataloğu ile iletişim kurulurken bir hata oluştu:\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 Menüye dön](virtual:menu)",

	// Download status
	"download.connecting": "⏳ Bağlanıyor... %s",
	"download.retry":      "⏳ Bağlantı kesildi. Yeniden deneniyor... (%d/5)",
	"download.stopped":    "🛑 İndirme durduruldu",
	"download.failed":     "❌ İndirme başarısız: %s",
	"download.finished":   "✅ İndirme başarıyla tamamlandı!",
	"download.progress":   "⬇ %s indiriliyor: %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 Makale ağacı",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
