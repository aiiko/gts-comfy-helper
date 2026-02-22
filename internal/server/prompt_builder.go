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

func buildCharacterDefinition(giantessCount int, giantessName, giantessAction, tiniesMode string, tinyCount int, tinyGender, tinyDescriptor, tinyAction string) (string, error) {
	giantessPhrase, err := buildGiantessPhrase(giantessCount, giantessName, giantessAction)
	if err != nil {
		return "", err
	}
	tiniesPhrase, err := buildTiniesPhrase(tiniesMode, tinyCount, tinyGender, tinyDescriptor, tinyAction)
	if err != nil {
		return "", err
	}
	return giantessPhrase + ", " + tiniesPhrase, nil
}

func buildGiantessPhrase(giantessCount int, giantessName, giantessAction string) (string, error) {
	name := normalizeName(giantessName)
	action := normalizeText(giantessAction)
	descriptor := ""
	head := ""

	switch giantessCount {
	case 1:
		head = "1girl"
		descriptor = "a giantess girl"
	case 2:
		head = "2girls"
		descriptor = "two giantess girls"
	default:
		return "", fmt.Errorf("giantess_count must be 1 or 2")
	}

	if action != "" {
		descriptor += " " + action
	}

	parts := []string{head}
	if name != "" {
		parts = append(parts, name)
	}
	parts = append(parts, descriptor)
	return strings.Join(parts, ", "), nil
}

func buildTiniesPhrase(tiniesMode string, tinyCount int, tinyGender, tinyDescriptor, tinyAction string) (string, error) {
	mode := strings.TrimSpace(strings.ToLower(tiniesMode))
	descriptor := normalizeText(tinyDescriptor)
	gender := strings.TrimSpace(strings.ToLower(tinyGender))
	action := normalizeText(tinyAction)

	phrase := ""
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
		phrase = strings.Join(parts, " ")
	case "group":
		parts := []string{"a group of"}
		if descriptor != "" {
			parts = append(parts, descriptor)
		}
		parts = append(parts, "tinies")
		phrase = strings.Join(parts, " ")
	default:
		return "", fmt.Errorf("tinies_mode must be count or group")
	}

	if action != "" {
		phrase += " " + action
	}
	return phrase, nil
}

func normalizeText(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(raw), " "))
}

func normalizeName(raw string) string {
	return strings.Join(strings.Fields(raw), " ")
}
