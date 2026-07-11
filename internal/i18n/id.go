package i18n

var idTranslations = map[string]string{
	// Menu
	"menu.title":      "📖 Menu",
	"menu.help":       "❓ Bantuan",
	"menu.settings":   "⚙️ Pengaturan",
	"menu.library":    "📥 Pustaka Arsip",
	"menu.downloads":  "⬇️ Unduhan",
	"menu.stop":       "Berhenti",
	"menu.start":      "Mulai",
	"menu.delete":     "Hapus",
	"menu.archives":   "📦 Arsip",
	"menu.no_content": "📭 Tidak ditemukan dokumen atau arsip ZIM di direktori saat ini.",

	// Help
	"help.title":      "❓ Bantuan",
	"help.navigation": "🧭 Navigasi",
	"help.scroll":     "📜 Gulir halaman",
	"help.open_link":  "🔗 Buka tautan",
	"help.go_back":    "🔙 Kembali",
	"help.main_menu":  "🏠 Menu utama",
	"help.toc":        "📑 Daftar isi",
	"help.zoom":       "🔍 Perbesar/Perkecil",
	"help.help":       "❓ Bantuan",
	"help.settings":   "⚙️ Pengaturan",
	"help.exit":       "🚪 Keluar",

	// Settings
	"settings.title":     "⚙️ Pengaturan",
	"settings.theme":     "🎨 Tema",
	"settings.fontsize":  "🔤 Ukuran huruf",
	"settings.language":  "🌐 Bahasa",
	"settings.save_note": "💾 Pengaturan disimpan otomatis dan akan diterapkan pada saat memulai berikutnya.",

	// Theme names
	"theme.light": "☀️ Terang",
	"theme.dark":  "🌙 Gelap",
	"theme.sepia": "📜 Sepia",

	// Library
	"library.title_languages":  "🌐 Pustaka Arsip - Bahasa",
	"library.select_language":  "👇 Pilih bahasa:",
	"library.back":             "🔙 Kembali",
	"library.title_categories": "🌐 Pustaka Arsip -",
	"library.select_category":  "👇 Pilih kategori:",
	"library.title_entries":    "🌐 Pustaka Arsip -",
	"library.back_categories":  "🔙 Kembali ke kategori",
	"library.no_entries":       "📭 Tidak ada arsip lain di halaman ini.",
	"library.no_archives":      "📭 Tidak ditemukan arsip dalam bahasa dan kategori ini.",
	"library.download":         "Unduh",
	"library.prev_page":        "◀ Halaman sebelumnya",
	"library.next_page":        "Halaman berikutnya ▶",
	"library.archives_count":   "%d arsip",

	// Library errors
	"library.section_languages":  "bahasa",
	"library.section_categories": "kategori",
	"library.section_archives":   "arsip",
	"library.error_loading":      "❌ Galat saat memuat %s",
	"library.error_message":      "Terjadi galat saat berkomunikasi dengan katalog pustaka Kiwix:\n\n`%v`\n\n",
	"library.back_to_menu":       "[🔙 Kembali ke menu](virtual:menu)",

	// Download status
	"download.connecting": "⏳ Menghubungkan... %s",
	"download.retry":      "⏳ Koneksi terputus. Mencoba lagi... (%d/5)",
	"download.stopped":    "🛑 Unduhan dihentikan",
	"download.failed":     "❌ Unduhan gagal: %s",
	"download.finished":   "✅ Unduhan berhasil selesai!",
	"download.progress":   "⬇ Mengunduh %s: %.1f%% (%s)",

	// Status bar
	"status.tree":        "🌳 Pohon artikel",
	"status.scroll_pct":  "📜 %d%%",
	"status.scroll_link": "📜 %d%% · 🔗 %d/%d",
}
