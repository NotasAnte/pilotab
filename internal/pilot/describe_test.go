package pilot

import (
	"strings"
	"testing"
)

const snapshot = `uid=3_0 RootWebArea "Sign in"
  uid=3_1 textbox "Email"
  uid=3_2 textbox "Password"
  uid=3_3 button "Delete repository"`

func TestDescribeActionNamesTheElementAndHidesSecrets(t *testing.T) {
	labels := snapshotLabels(snapshot)
	cases := []struct {
		tool     string
		args     map[string]any
		want     string
		mustHide string
	}{
		{"click", map[string]any{"uid": "3_3"}, `click button "Delete repository"`, ""},
		{"click", map[string]any{"uid": "9_9"}, "click element 9_9", ""},
		{"fill", map[string]any{"uid": "3_1", "value": "me@x.com"}, `fill textbox "Email" with "me@x.com"`, ""},
		{"fill", map[string]any{"uid": "3_2", "value": "hunter22"}, `fill textbox "Password" with [hidden, 8 chars]`, "hunter22"},
		{"fill_form", map[string]any{"elements": []any{
			map[string]any{"uid": "3_1", "value": "me@x.com"},
			map[string]any{"uid": "3_2", "value": "hunter22"},
		}}, `fill the form: textbox "Email" = "me@x.com"; textbox "Password" = [hidden, 8 chars]`, "hunter22"},
		{"type_text", map[string]any{"text": "line1\nline2"}, `type "line1 line2"`, ""},
	}
	for _, c := range cases {
		got := describeAction(c.tool, c.args, labels)
		if got != c.want {
			t.Errorf("describeAction(%s) = %q, want %q", c.tool, got, c.want)
		}
		if c.mustHide != "" && strings.Contains(got, c.mustHide) {
			t.Errorf("describeAction(%s) leaks %q", c.tool, c.mustHide)
		}
	}
}

func TestClipKeepsMessagesShort(t *testing.T) {
	got := describeAction("type_text", map[string]any{"text": strings.Repeat("a", 500)}, nil)
	if len(got) > 80 || !strings.HasSuffix(got, `…"`) {
		t.Errorf("not clipped: %q", got)
	}
}
