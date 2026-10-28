package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type session struct {
	t      *testing.T
	client *mcp.ClientSession
	reply  *mcp.ElicitResult
	asked  *[]string
}

func TestPilotGuardsTheBrowser(t *testing.T) {
	if testing.Short() {
		t.Skip("starts Chrome")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	site := httptest.NewServer(http.FileServer(http.Dir(".")))
	t.Cleanup(site.Close)
	fixture := site.URL + "/ping.html"

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	s := startPilot(t, root, auditPath)

	listing := s.call("list_pages", nil)
	if strings.Contains(listing, "about:blank") {
		t.Fatalf("list_pages shows a page this session did not open:\n%s", listing)
	}

	s.denied("new_page", map[string]any{"url": "https://example.com/"}, "outside allowHosts")
	s.denied("new_page", map[string]any{"url": "file:///etc/passwd"}, "outside allowHosts")

	pageID := selectedPage(t, s.call("new_page", map[string]any{"url": fixture}))
	s.denied("take_snapshot", map[string]any{"pageId": 1}, "not opened by this session")
	s.denied("take_snapshot", map[string]any{"pageId": float64(pageID) + 0.5}, "whole number")
	s.denied("take_snapshot", map[string]any{}, "whole number")

	button := uidOf(t, s.call("take_snapshot", map[string]any{"pageId": pageID}), "button", "Ping")

	s.denied("take_snapshot", map[string]any{"pageId": pageID, "filePath": "/tmp/pilotab-snapshot.txt"}, "filePath is not allowed")
	s.denied("evaluate_script", map[string]any{"pageId": pageID, "function": "() => document.cookie"}, "evaluate_script is blocked")

	*s.reply = mcp.ElicitResult{Action: "decline"}
	s.denied("click", map[string]any{"pageId": pageID, "uid": button}, "did not allow click")
	if last := (*s.asked)[len(*s.asked)-1]; !strings.Contains(last, `click button "Ping"`) {
		t.Fatalf("confirmation does not name the element: %q", last)
	}
	if snapshot := s.call("take_snapshot", map[string]any{"pageId": pageID}); !strings.Contains(snapshot, "idle") {
		t.Fatalf("a declined click still reached the page:\n%s", snapshot)
	}

	*s.reply = mcp.ElicitResult{Action: "accept", Content: map[string]any{"allowHost": true}}
	s.call("click", map[string]any{"pageId": pageID, "uid": button})
	s.call("wait_for", map[string]any{"pageId": pageID, "text": []string{"pong"}})

	askedBefore := len(*s.asked)
	*s.reply = mcp.ElicitResult{Action: "decline"}
	s.call("click", map[string]any{"pageId": pageID, "uid": button})
	if len(*s.asked) != askedBefore {
		t.Fatalf("asked again inside a host grant: %q", (*s.asked)[askedBefore:])
	}

	s.call("close_page", map[string]any{"pageId": pageID})
	s.denied("take_snapshot", map[string]any{"pageId": pageID}, "not opened by this session")

	assertAudited(t, auditPath, map[string]bool{"denied": true, "asked": true, "declined": true, "confirmed": true, "granted": true, "allowed": true})
}

func TestDangerousModeSkipsConfirmation(t *testing.T) {
	if testing.Short() {
		t.Skip("starts Chrome")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	site := httptest.NewServer(http.FileServer(http.Dir(".")))
	t.Cleanup(site.Close)

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	s := startPilot(t, root, auditPath, "-dangerously-skip-confirmations")

	s.denied("new_page", map[string]any{"url": "https://example.com/"}, "outside allowHosts")
	pageID := selectedPage(t, s.call("new_page", map[string]any{"url": site.URL + "/ping.html"}))
	button := uidOf(t, s.call("take_snapshot", map[string]any{"pageId": pageID}), "button", "Ping")
	s.call("click", map[string]any{"pageId": pageID, "uid": button})
	s.call("wait_for", map[string]any{"pageId": pageID, "text": []string{"pong"}})
	s.call("close_page", map[string]any{"pageId": pageID})

	assertAudited(t, auditPath, map[string]bool{"denied": true, "unconfirmed": true})
}

func startPilot(t *testing.T, root, auditPath string, flags ...string) *session {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "pilotab")
	build := exec.Command("go", "build", "-o", binary, "./cmd/pilotab")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	upstreamArgs := []string{"--headless", "--isolated", "--no-category-performance", "--no-category-memory", "--no-category-emulation"}
	if os.Getenv("PILOTAB_LIVE") == "1" {
		upstreamArgs = []string{"--autoConnect"}
	}
	config := map[string]any{
		"upstream":     map[string]any{"command": filepath.Join(root, "bin", "chrome-devtools-mcp.sh"), "args": upstreamArgs},
		"tabs":         "own",
		"allowHosts":   []string{"127.0.0.1"},
		"confirmHosts": []string{"127.0.0.1"},
		"grantMinutes": 10,
		"auditLog":     auditPath,
	}
	configPath := filepath.Join(t.TempDir(), "pilot.json")
	raw, _ := json.Marshal(config)
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	reply := &mcp.ElicitResult{Action: "decline"}
	var asked []string
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "0"}, &mcp.ClientOptions{
		ElicitationHandler: func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			asked = append(asked, req.Params.Message)
			answer := *reply
			return &answer, nil
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	cs, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(binary, append(flags, "-config", configPath)...)}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return &session{t: t, client: cs, reply: reply, asked: &asked}
}

func (s *session) result(tool string, args map[string]any) (string, bool) {
	s.t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	res, err := s.client.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		s.t.Fatalf("%s: %v", tool, err)
	}
	var parts []string
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n"), res.IsError
}

func (s *session) call(tool string, args map[string]any) string {
	s.t.Helper()
	text, isError := s.result(tool, args)
	if isError {
		s.t.Fatalf("%s failed: %s", tool, text)
	}
	return text
}

func (s *session) denied(tool string, args map[string]any, reason string) {
	s.t.Helper()
	text, isError := s.result(tool, args)
	if !isError || !strings.Contains(text, reason) {
		s.t.Fatalf("%s: want a denial containing %q, got (error=%v) %s", tool, reason, isError, text)
	}
}

func selectedPage(t *testing.T, listing string) int {
	t.Helper()
	match := regexp.MustCompile(`(?m)^(\d+): .*\[selected\]$`).FindStringSubmatch(listing)
	if match == nil {
		t.Fatalf("no selected page in:\n%s", listing)
	}
	id, _ := strconv.Atoi(match[1])
	return id
}

func uidOf(t *testing.T, snapshot, role, name string) string {
	t.Helper()
	match := regexp.MustCompile(`uid=(\S+) ` + role + ` "` + regexp.QuoteMeta(name) + `"`).FindStringSubmatch(snapshot)
	if match == nil {
		t.Fatalf("no %s %q in snapshot:\n%s", role, name, snapshot)
	}
	return match[1]
}

func assertAudited(t *testing.T, path string, decisions map[string]bool) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "Ping\"") && strings.Contains(string(raw), "\"value\"") {
		t.Fatalf("audit log holds typed values:\n%s", raw)
	}
	seen := map[string]bool{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		var entry struct{ Decision string }
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("bad audit line %q: %v", line, err)
		}
		seen[entry.Decision] = true
	}
	for decision := range decisions {
		if !seen[decision] {
			t.Errorf("audit log has no %q entry:\n%s", decision, raw)
		}
	}
}
