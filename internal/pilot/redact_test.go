package pilot

import (
	"strings"
	"testing"
)

func TestRedactRemovesCredentials(t *testing.T) {
	secrets := map[string]string{
		"authorization header": "Authorization: Bearer abcdefghijklmnop",
		"cookie header":        "cookie: session=s3cr3t; theme=dark",
		"set-cookie in json":   `"set-cookie": "sid=s3cr3t; HttpOnly"`,
		"network list header":  "### Request headers\n- cookie:user_session=s3cr3t; theme=dark",
		"x-csrf-token header":  "- x-csrf-token:s3cr3t",
		"bearer inline":        "token was Bearer abcdefghijklmnop.qrs",
		"jwt":                  "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		"github token":         "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		"aws key":              "AKIAIOSFODNN7EXAMPLE",
		"google key":           "AIzaSyA-1234567890abcdefghijklmnopqrstu",
		"slack token":          "xoxb-1234567890-abcdefghij",
		"anthropic key":        "sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
	}
	leaks := []string{"abcdefghijklmnop", "s3cr3t", "dozjgNryP4J3", "ABCDEFGHIJKLMNOP", "IOSFODNN7", "1234567890abcdefghij", "api03-abcdefghij"}
	for name, input := range secrets {
		got := redact(input)
		for _, leak := range leaks {
			if strings.Contains(got, leak) {
				t.Errorf("%s: %q leaks %q", name, got, leak)
			}
		}
	}
}

func TestRedactKeepsOrdinaryText(t *testing.T) {
	text := "## Pages\n1: https://github.com/settings/tokens [selected]\nuid=1_2 button \"Generate new token\""
	if got := redact(text); got != text {
		t.Fatalf("redact() changed ordinary text: %q", got)
	}
}

func TestAuditArgsOmitsTypedValues(t *testing.T) {
	got := auditArgs(map[string]any{"pageId": float64(3), "uid": "1_4", "value": "hunter2", "url": "https://x.dev/?t=Bearer abcdefghijklmnop"})
	if got["value"] != "[omitted]" || got["uid"] != "1_4" || got["pageId"] != float64(3) {
		t.Fatalf("auditArgs() = %v", got)
	}
	if strings.Contains(got["url"].(string), "abcdefghijklmnop") {
		t.Fatalf("auditArgs() leaked a token in url: %v", got["url"])
	}
}
