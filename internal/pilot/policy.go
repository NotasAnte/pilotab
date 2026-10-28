package pilot

import (
	"fmt"
	"math"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var writeTools = map[string]bool{
	"click":           true,
	"drag":            true,
	"evaluate_script": true,
	"fill":            true,
	"fill_form":       true,
	"handle_dialog":   true,
	"press_key":       true,
	"type_text":       true,
	"upload_file":     true,
}

var alwaysConfirmTools = map[string]bool{
	"upload_file": true,
}

var sensitiveArgs = map[string]bool{
	"elements":   true,
	"filePaths":  true,
	"function":   true,
	"initScript": true,
	"promptText": true,
	"text":       true,
	"value":      true,
}

var forbiddenArgs = map[string]string{
	"filePath":         "files bypass redaction; get the result inline",
	"outputDirPath":    "files bypass redaction; get the result inline",
	"requestFilePath":  "files bypass redaction; get the result inline",
	"responseFilePath": "files bypass redaction; get the result inline",
	"sourcePath":       "files bypass redaction; get the result inline",
	"initScript":       "it also runs on whatever page a redirect lands on",
}

var contextName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func argsReason(args map[string]any) string {
	for key, why := range forbiddenArgs {
		if _, ok := args[key]; ok {
			return fmt.Sprintf("%s is not allowed: %s", key, why)
		}
	}
	if name, ok := args["isolatedContext"]; ok {
		if text, _ := name.(string); !contextName.MatchString(text) {
			return "isolatedContext must match [A-Za-z0-9_-]{1,64}"
		}
	}
	return ""
}

func pageIDArg(args map[string]any) (int, bool) {
	number, ok := args["pageId"].(float64)
	if !ok || number != math.Trunc(number) || number < 0 || number > math.MaxInt32 {
		return 0, false
	}
	return int(number), true
}

func withoutForbiddenArgs(schema any) any {
	object, ok := schema.(map[string]any)
	if !ok {
		return schema
	}
	if properties, ok := object["properties"].(map[string]any); ok {
		for key := range forbiddenArgs {
			delete(properties, key)
		}
	}
	if required, ok := object["required"].([]any); ok {
		kept := required[:0]
		for _, name := range required {
			if key, _ := name.(string); forbiddenArgs[key] == "" {
				kept = append(kept, name)
			}
		}
		object["required"] = kept
	}
	return object
}

func matchesHost(patterns []string, host string) bool {
	host = strings.ToLower(host)
	for _, pattern := range patterns {
		if ok, _ := path.Match(strings.ToLower(pattern), host); ok {
			return true
		}
	}
	return false
}

func hostOf(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	switch parsed.Scheme {
	case "http", "https":
		return strings.TrimSuffix(parsed.Hostname(), "."), true
	default:
		return "", false
	}
}

func (c *Config) urlAllowed(rawURL string) bool {
	if rawURL == "about:blank" {
		return true
	}
	host, ok := hostOf(rawURL)
	return ok && matchesHost(c.AllowHosts, host)
}

func (c *Config) isConfirmHost(rawURL string) bool {
	host, ok := hostOf(rawURL)
	return ok && matchesHost(c.ConfirmHosts, host)
}

func (c *Config) needsConfirmation(tool, pageURL string) bool {
	return alwaysConfirmTools[tool] || (writeTools[tool] && c.isConfirmHost(pageURL))
}

func (c *Config) scriptReason(tool, pageURL string) string {
	if tool == "evaluate_script" && c.isConfirmHost(pageURL) {
		return fmt.Sprintf("evaluate_script is blocked on %s (confirmHosts); use take_snapshot or wait_for", pageURL)
	}
	return ""
}
