package configs

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed language_extensions.json
var languageExtensionJSON embed.FS

type languageExtension struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Extensions []string `json:"extensions"`
}

func LoadExtensionsMap() map[string][]string {
	var languages []languageExtension
	// Изменяем способ чтения файла с использованием go:embed
	data, err := languageExtensionJSON.ReadFile("language_extensions.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка при чтении файла: %v\n", err)
		os.Exit(1)
	}

	err = json.Unmarshal(data, &languages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка при декодировании JSON: %v\n", err)
		os.Exit(1)
	}

	extensionsMap := make(map[string][]string)
	for _, lang := range languages {
		for _, ext := range lang.Extensions {
			langName := strings.ToLower(lang.Name)
			extensionsMap[langName] = append(extensionsMap[langName], ext)
		}
	}
	return extensionsMap
}
