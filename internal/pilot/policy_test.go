package pilot

import "testing"

func TestURLAllowed(t *testing.T) {
	cfg := &Config{AllowHosts: []string{"github.com", "*.google.com"}}
	cases := map[string]bool{
		"https://github.com/settings":        true,
		"https://GitHub.com/":                true,
		"https://console.cloud.google.com/x": true,
		"https://google.com/":                false,
		"https://github.com.evil.com/":       false,
		"https://evil.com/?github.com":       false,
		"http://user@evil.com@github.com/":   true,
		"file:///etc/passwd":                 false,
		"javascript:alert(1)":                false,
		"data:text/html,hi":                  false,
		"about:blank":                        true,
		"chrome://settings":                  false,
	}
	for url, want := range cases {
		if got := cfg.urlAllowed(url); got != want {
			t.Errorf("urlAllowed(%q) = %v, want %v", url, got, want)
		}
	}
}

func TestNeedsConfirmation(t *testing.T) {
	cfg := &Config{ConfirmHosts: []string{"dash.cloudflare.com"}}
	cases := []struct {
		tool, url string
		want      bool
	}{
		{"click", "https://dash.cloudflare.com/zone", true},
		{"evaluate_script", "https://dash.cloudflare.com/zone", true},
		{"take_snapshot", "https://dash.cloudflare.com/zone", false},
		{"click", "https://example.com/", false},
		{"upload_file", "https://example.com/", true},
	}
	for _, c := range cases {
		if got := cfg.needsConfirmation(c.tool, c.url); got != c.want {
			t.Errorf("needsConfirmation(%q, %q) = %v, want %v", c.tool, c.url, got, c.want)
		}
	}
}

func TestForbiddenArgsAreRejectedAndHiddenFromSchemas(t *testing.T) {
	for key := range forbiddenArgs {
		if argsReason(map[string]any{"pageId": float64(1), key: "x"}) == "" {
			t.Errorf("%s was not rejected", key)
		}
	}
	if reason := argsReason(map[string]any{"pageId": float64(1), "isolatedContext": "work_1"}); reason != "" {
		t.Errorf("plain call rejected: %s", reason)
	}
	if argsReason(map[string]any{"isolatedContext": "x\n7: https://github.com/"}) == "" {
		t.Error("isolatedContext that forges page lines was accepted")
	}

	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"pageId": map[string]any{}, "filePath": map[string]any{}, "initScript": map[string]any{}},
		"required":   []any{"pageId", "filePath"},
	}
	got := withoutForbiddenArgs(schema).(map[string]any)
	properties := got["properties"].(map[string]any)
	if _, ok := properties["filePath"]; ok {
		t.Error("filePath still advertised")
	}
	if _, ok := properties["initScript"]; ok {
		t.Error("initScript still advertised")
	}
	if required := got["required"].([]any); len(required) != 1 || required[0] != "pageId" {
		t.Errorf("required = %v", required)
	}
}

func TestPageIDMustBeAWholeNumber(t *testing.T) {
	cases := map[any]bool{float64(3): true, float64(0): true, 3.5: false, float64(-1): false, "3": false, nil: false, 1e12: false}
	for value, want := range cases {
		if _, ok := pageIDArg(map[string]any{"pageId": value}); ok != want {
			t.Errorf("pageIDArg(%v) ok = %v, want %v", value, ok, want)
		}
	}
}

func TestTrailingDotHostIsStillAConfirmHost(t *testing.T) {
	cfg := &Config{ConfirmHosts: []string{"github.com"}}
	if !cfg.needsConfirmation("click", "https://github.com./settings") {
		t.Error("github.com. escaped confirmHosts")
	}
}

func TestEvaluateIsBlockedOnlyOnConfirmHosts(t *testing.T) {
	cfg := &Config{ConfirmHosts: []string{"dash.cloudflare.com"}}
	if cfg.scriptReason("evaluate_script", "https://dash.cloudflare.com/") == "" {
		t.Error("evaluate_script allowed on a confirm host")
	}
	if reason := cfg.scriptReason("evaluate_script", "https://example.com/"); reason != "" {
		t.Errorf("evaluate_script blocked elsewhere: %s", reason)
	}
}
