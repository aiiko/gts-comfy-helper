package server

import "strings"

var artStyleOptions = []string{
	"official art",
	"official style",
	"anime screencap",
	"realistic",
	"photorealistic",
	"monochrome",
	"greyscale",
	"traditional media",
}

var bodyFramingOptions = []string{
	"full body",
	"cowboy shot",
	"upper body",
	"lower body",
}

var cameraSelectorOptions = []string{
	"from above",
	"from below",
	"from behind",
	"from side",
	"dutch angle",
	"close-up",
	"foreshortening",
	"wide shot",
}

var tiniesModeOptions = []string{
	"count",
	"group",
}

var tinyGenderOptions = []string{
	"male",
	"female",
	"girl",
	"boy",
}

var artStyleOptionsMap = buildOptionsMap(artStyleOptions)
var bodyFramingOptionsMap = buildOptionsMap(bodyFramingOptions)
var cameraSelectorOptionsMap = buildOptionsMap(cameraSelectorOptions)
var tiniesModeOptionsMap = buildOptionsMap(tiniesModeOptions)
var tinyGenderOptionsMap = buildOptionsMap(tinyGenderOptions)

func buildOptionsMap(options []string) map[string]string {
	out := make(map[string]string, len(options))
	for _, option := range options {
		normalized := strings.TrimSpace(option)
		if normalized == "" {
			continue
		}
		out[strings.ToLower(normalized)] = normalized
	}
	return out
}

func canonicalOption(input string, options map[string]string) (string, bool) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", true
	}
	option, ok := options[strings.ToLower(value)]
	if !ok {
		return "", false
	}
	return option, true
}
