package i18n

import (
	"os"
	"strings"
)

// Language represents supported languages
type Language string

const (
	English Language = "en"
	Chinese Language = "zh"
)

// AllMessages holds all translatable strings grouped by module
type AllMessages struct {
	App         AppMessages
	Common      CommonMessages
	Extract     ExtractMessages
	List        ListMessages
	Metadata    MetadataMessages
	ExtractDiff ExtractDiffMessages
	Dumper      DumperMessages
}

// CurrentLanguage holds the current language setting
var CurrentLanguage Language = English

// I18nMsg holds the current message set - Global variable for easy access
var I18nMsg AllMessages

// English messages
var EnglishAllMessages = AllMessages{
	App:         EnglishAppMessages,
	Common:      EnglishCommonMessages,
	Extract:     EnglishExtractMessages,
	List:        EnglishListMessages,
	Metadata:    EnglishMetadataMessages,
	ExtractDiff: EnglishExtractDiffMessages,
	Dumper:      EnglishDumperMessages,
}

// Chinese messages
var ChineseAllMessages = AllMessages{
	App:         ChineseAppMessages,
	Common:      ChineseCommonMessages,
	Extract:     ChineseExtractMessages,
	List:        ChineseListMessages,
	Metadata:    ChineseMetadataMessages,
	ExtractDiff: ChineseExtractDiffMessages,
	Dumper:      ChineseDumperMessages,
}

// DetectLanguage detects the user's language preference based on environment variables
func DetectLanguage() Language {
	envVars := []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"}

	for _, envVar := range envVars {
		if lang := os.Getenv(envVar); lang != "" {
			lang = strings.ToLower(lang)
			if strings.Contains(lang, "zh") ||
				strings.Contains(lang, "chinese") ||
				strings.Contains(lang, "cn") {
				return Chinese
			}
		}
	}

	return English
}

// SetLanguage sets the current language and updates messages
func SetLanguage(lang Language) {
	CurrentLanguage = lang
	switch lang {
	case Chinese:
		I18nMsg = ChineseAllMessages
	default:
		I18nMsg = EnglishAllMessages
	}
}

// InitLanguage initializes the language system
func InitLanguage() {
	detectedLang := DetectLanguage()
	SetLanguage(detectedLang)
}

// IsChineseEnvironment returns true if the current environment is Chinese
func IsChineseEnvironment() bool {
	return CurrentLanguage == Chinese
}
