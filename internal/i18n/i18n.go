package i18n

import (
	"embed"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/Xuanwo/go-locale"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localeFS embed.FS

var (
	bundle    *i18n.Bundle
	localizer *i18n.Localizer
	once      sync.Once
)

// Init initializes the i18n module. Call this once at program startup.
// If lang is empty, the system locale will be detected automatically.
func Init(lang string) {
	once.Do(func() {
		bundle = i18n.NewBundle(language.English)
		bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

		// Load embedded translation files
		bundle.LoadMessageFileFS(localeFS, "locales/en.toml")
		bundle.LoadMessageFileFS(localeFS, "locales/zh.toml")

		// Determine language to use
		langs := []string{}
		if lang != "" {
			langs = append(langs, lang)
		} else {
			// Auto-detect system locale using POSIX standard order:
			// LANGUAGE > LC_ALL > LC_MESSAGES > LANG
			if detected, err := locale.Detect(); err == nil {
				langs = append(langs, detected.String())
			}
		}

		localizer = i18n.NewLocalizer(bundle, langs...)
	})
}

// T returns the translated string for the given message ID.
func T(messageID string) string {
	Init("") // Ensure initialized
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
	})
	if err != nil {
		return messageID // Fallback to message ID
	}
	return msg
}

// Tf returns the translated string with template data substitution.
func Tf(messageID string, data map[string]interface{}) string {
	Init("") // Ensure initialized
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
	})
	if err != nil {
		return messageID
	}
	return msg
}
