package server

import (
	"fmt"
	"strconv"
	"strings"
)

var fixedPromptOrder = []string{"positive_tags", "character_definition", "prompt", "art_style", "body_framing", "camera_selector"}

func buildFinalPrompt(positiveTags, characterDefinition, userPrompt, artStyle, bodyFraming, cameraSelector string) string {
	partsByToken := map[string]string{
		"positive_tags":        strings.TrimSpace(positiveTags),
		"character_definition": strings.TrimSpace(characterDefinition),
		"prompt":               strings.TrimSpace(userPrompt),
		"art_style":            strings.TrimSpace(artStyle),
		"body_framing":         strings.TrimSpace(bodyFraming),
		"camera_selector":      strings.TrimSpace(cameraSelector),
	}

	parts := make([]string, 0, len(fixedPromptOrder))
	for _, token := range fixedPromptOrder {
		part := strings.TrimSpace(partsByToken[token])
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func buildCharacterDefinition(giantessCount int, tiniesMode string, tinyCount int, tinyGender, tinyDescriptor string) (string, error) {
	giantessPhrase, err := buildGiantessPhrase(giantessCount)
	if err != nil {
		return "", err
	}
	tiniesPhrase, err := buildTiniesPhrase(tiniesMode, tinyCount, tinyGender, tinyDescriptor)
	if err != nil {
		return "", err
	}
	return giantessPhrase + ", " + tiniesPhrase, nil
}

func buildGiantessPhrase(giantessCount int) (string, error) {
	switch giantessCount {
	case 1:
		return "1girl, a giantess girl", nil
	case 2:
		return "2girls, two giantess girls", nil
	default:
		return "", fmt.Errorf("giantess_count must be 1 or 2")
	}
}

func buildTiniesPhrase(tiniesMode string, tinyCount int, tinyGender, tinyDescriptor string) (string, error) {
	mode := strings.TrimSpace(strings.ToLower(tiniesMode))
	descriptor := normalizeDescriptor(tinyDescriptor)
	gender := strings.TrimSpace(strings.ToLower(tinyGender))

	switch mode {
	case "count":
		if tinyCount <= 0 {
			return "", fmt.Errorf("tiny_count must be a positive integer when tinies_mode is count")
		}
		noun := "tinies"
		if tinyCount == 1 {
			noun = "tiny"
		}
		parts := []string{strconv.Itoa(tinyCount)}
		if gender != "" {
			parts = append(parts, gender)
		}
		if descriptor != "" {
			parts = append(parts, descriptor)
		}
		parts = append(parts, noun)
		return strings.Join(parts, " "), nil
	case "group":
		parts := []string{"a group of"}
		if descriptor != "" {
			parts = append(parts, descriptor)
		}
		parts = append(parts, "tinies")
		return strings.Join(parts, " "), nil
	default:
		return "", fmt.Errorf("tinies_mode must be count or group")
	}
}

func normalizeDescriptor(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(raw), " "))
}
