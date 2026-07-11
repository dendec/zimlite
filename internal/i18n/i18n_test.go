package i18n

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

func TestT_ExistingKey(t *testing.T) {
	got := T("en", "menu.title")
	if got != "📖 Menu" {
		t.Errorf("T(\"en\", \"menu.title\") = %q, want %q", got, "📖 Menu")
	}
}

func TestT_FallbackToEnglish(t *testing.T) {
	got := T("xx", "menu.title")
	if got != "📖 Menu" {
		t.Errorf("T(\"xx\", \"menu.title\") = %q, want %q", got, "📖 Menu")
	}
}

func TestT_UnknownKey(t *testing.T) {
	got := T("en", "nonexistent.key")
	if got != "nonexistent.key" {
		t.Errorf("T(\"en\", \"nonexistent.key\") = %q, want %q", got, "nonexistent.key")
	}
}

func TestT_Russian(t *testing.T) {
	got := T("ru", "menu.title")
	if got != "📖 Меню" {
		t.Errorf("T(\"ru\", \"menu.title\") = %q, want %q", got, "📖 Меню")
	}
}

func TestTf_Formatted(t *testing.T) {
	got := Tf("en", "download.progress", "file.zim", 42.5, "1.2MB/s")
	want := "⬇ Downloading file.zim: 42.5% (1.2MB/s)"
	if got != want {
		t.Errorf("Tf() = %q, want %q", got, want)
	}
}

func TestLanguages_NonEmpty(t *testing.T) {
	langs := Languages()
	if len(langs) == 0 {
		t.Fatal("Languages() returned empty slice")
	}
	for _, l := range langs {
		if l.Code == "" {
			t.Error("language with empty Code")
		}
		if l.Name == "" {
			t.Errorf("language %q has empty Name", l.Code)
		}
	}
}

func TestLanguages_Immutable(t *testing.T) {
	langs := Languages()
	origLen := len(langs)
	_ = append(langs, Language{Code: "xx", Name: "Fake"})
	if len(Languages()) != origLen {
		t.Error("Languages() returned mutable internal slice")
	}
}

func TestParity_LanguagesAndTranslations(t *testing.T) {
	// Every registered language must have a translations map, and vice versa.
	langCodes := make(map[string]bool)
	for _, e := range registry {
		langCodes[e.Code] = true
		if len(e.Translations) == 0 {
			t.Errorf("language %q has empty translations", e.Code)
		}
	}

	// Every registry entry must have a translations map and be reachable by T().
	for _, e := range registry {
		if got := T(e.Code, "menu.title"); got == "menu.title" {
			t.Errorf("registry language %q: T() returns raw key, translations not registered", e.Code)
		}
	}
}

func TestKeyParity_AllRegistered(t *testing.T) {
	// Collect the union of all keys across all languages.
	allKeys := make(map[string]bool)
	for _, e := range registry {
		for key := range e.Translations {
			allKeys[key] = true
		}
	}
	sortedKeys := slices.Sorted(maps.Keys(allKeys))

	// Check every registered language has every key.
	for _, e := range registry {
		for _, key := range sortedKeys {
			if _, ok := e.Translations[key]; !ok {
				t.Errorf("key %q missing in language %q", key, e.Code)
			}
		}
	}
}

func TestNoEnglishLiteralsInTranslations(t *testing.T) {
	forbidden := []string{
		"Help", "Settings", "Theme", "Language",
		"Delete",
		"Error loading", "Back to",
	}
	for _, e := range registry {
		if e.Code == "en" {
			continue
		}
		for key, val := range e.Translations {
			for _, word := range forbidden {
				if strings.Contains(val, word) {
					t.Errorf("lang=%q key=%q contains English literal %q: %q", e.Code, key, word, val)
				}
			}
		}
	}
}
