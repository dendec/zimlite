// Package i18n provides simple key-value translation for UI strings.
package i18n

import "fmt"

// langEntry bundles a language code, display name, and its translation map.
// Register all languages in the init() below — one place, no sync issues.
type langEntry struct {
	Code         string            // e.g. "en"
	Name         string            // e.g. "🇬🇧 English"
	Translations map[string]string // key → translated value
}

var registry []langEntry

func init() {
	// Sorted by estimated potential user base for this app:
	// Chinese (TrimUI native), Spanish/Portuguese (PortMaster),
	// French/Arabic (Kiwix/ZIM adoption), European + Asian markets.
	registry = []langEntry{
		{Code: "en", Name: "🇬🇧 English", Translations: enTranslations},
		{Code: "ru", Name: "🇷🇺 Русский", Translations: ruTranslations},
		{Code: "zh", Name: "🇨🇳 中文", Translations: zhTranslations},
		{Code: "es", Name: "🇪🇸 Español", Translations: esTranslations},
		{Code: "fr", Name: "🇫🇷 Français", Translations: frTranslations},
		{Code: "pt", Name: "🇧🇷 Português", Translations: ptTranslations},
		{Code: "de", Name: "🇩🇪 Deutsch", Translations: deTranslations},
		{Code: "ja", Name: "🇯🇵 日本語", Translations: jaTranslations},
		{Code: "it", Name: "🇮🇹 Italiano", Translations: itTranslations},
		{Code: "ko", Name: "🇰🇷 한국어", Translations: koTranslations},
		{Code: "tr", Name: "🇹🇷 Türkçe", Translations: trTranslations},
		{Code: "id", Name: "🇮🇩 Bahasa Indonesia", Translations: idTranslations},
		{Code: "uk", Name: "🇺🇦 Українська", Translations: ukTranslations},
	}

	// Build lookup map from registry.
	for i := range registry {
		byCode[registry[i].Code] = &registry[i]
	}
}

var byCode = make(map[string]*langEntry)

// Language exposes code and display name for the settings selector.
type Language struct {
	Code string
	Name string
}

// Languages returns a copy of all registered UI languages.
func Languages() []Language {
	out := make([]Language, len(registry))
	for i, e := range registry {
		out[i] = Language{Code: e.Code, Name: e.Name}
	}
	return out
}

// T returns the translated string for key in the given language.
// Falls back to English if key is missing in the requested language.
func T(lang, key string) string {
	if lang != "en" {
		if e, ok := byCode[lang]; ok {
			if v, ok := e.Translations[key]; ok {
				return v
			}
		}
	}
	if e, ok := byCode["en"]; ok {
		if v, ok := e.Translations[key]; ok {
			return v
		}
	}
	return key
}

// Tf returns a formatted translated string (fmt.Sprintf wrapper).
func Tf(lang, key string, args ...any) string {
	return fmt.Sprintf(T(lang, key), args...)
}
