package pilot

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var snapshotLine = regexp.MustCompile(`(?m)^\s*uid=(\S+) (.+)$`)

func snapshotLabels(text string) map[string]string {
	matches := snapshotLine.FindAllStringSubmatch(text, -1)
	if matches == nil {
		return nil
	}
	labels := make(map[string]string, len(matches))
	for _, match := range matches {
		labels[match[1]] = clip(match[2], 100)
	}
	return labels
}

func describeAction(tool string, args map[string]any, labels map[string]string) string {
	element := func(key string) string {
		uid, _ := args[key].(string)
		if label, ok := labels[uid]; ok {
			return label
		}
		return "element " + uid
	}
	text := func(key string) string {
		value, _ := args[key].(string)
		return quote(value)
	}
	switch tool {
	case "click":
		if args["dblClick"] == true {
			return "double-click " + element("uid")
		}
		return "click " + element("uid")
	case "fill":
		return fmt.Sprintf("fill %s with %s", element("uid"), secretAware(element("uid"), args["value"]))
	case "fill_form":
		var fields []string
		items, _ := args["elements"].([]any)
		for _, item := range items {
			field, _ := item.(map[string]any)
			uid, _ := field["uid"].(string)
			label, ok := labels[uid]
			if !ok {
				label = "element " + uid
			}
			fields = append(fields, fmt.Sprintf("%s = %s", label, secretAware(label, field["value"])))
		}
		return "fill the form: " + strings.Join(fields, "; ")
	case "type_text":
		return "type " + text("text")
	case "press_key":
		return "press " + text("key")
	case "drag":
		return fmt.Sprintf("drag %s onto %s", element("from_uid"), element("to_uid"))
	case "upload_file":
		paths, _ := args["filePaths"].([]any)
		return fmt.Sprintf("upload %v to %s", paths, element("uid"))
	case "handle_dialog":
		return fmt.Sprintf("%s the dialog %s", text("action"), text("promptText"))
	default:
		return tool
	}
}

func secretAware(label string, value any) string {
	text, _ := value.(string)
	lower := strings.ToLower(label)
	for _, hint := range []string{"password", "contraseña", "passcode", "secret", "token"} {
		if strings.Contains(lower, hint) {
			return fmt.Sprintf("[hidden, %d chars]", len([]rune(text)))
		}
	}
	return quote(text)
}

func quote(text string) string {
	return fmt.Sprintf("%q", clip(text, 60))
}

func clip(text string, limit int) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}
