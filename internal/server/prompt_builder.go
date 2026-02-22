package server

import "strings"

var fixedPromptOrder = []string{"positive_tags", "prompt", "art_style", "body_framing", "camera_selector"}

func buildFinalPrompt(positiveTags, userPrompt, artStyle, bodyFraming, cameraSelector string) string {
	partsByToken := map[string]string{
		"positive_tags":   strings.TrimSpace(positiveTags),
		"prompt":          strings.TrimSpace(userPrompt),
		"art_style":       strings.TrimSpace(artStyle),
		"body_framing":    strings.TrimSpace(bodyFraming),
		"camera_selector": strings.TrimSpace(cameraSelector),
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
