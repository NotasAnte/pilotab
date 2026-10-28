package pilot

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.1.0"

type Proxy struct {
	cfg      *Config
	upstream *mcp.ClientSession
	audit    *auditLog
	stateKey []byte

	pageScoped map[string]bool

	mu     sync.Mutex
	owned  map[int]bool
	labels map[int]map[string]string
	grants map[string]time.Time
}

func Start(ctx context.Context, cfg *Config) (*Proxy, error) {
	audit, err := openAuditLog(cfg.AuditPath())
	if err != nil {
		return nil, fmt.Errorf("audit log: %w", err)
	}
	cmd := exec.Command(cfg.UpstreamCommand(), cfg.Upstream.Args...)
	cmd.Dir = cfg.Dir()
	cmd.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "pilotab", Version: version}, nil)
	upstream, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		_ = audit.Close()
		return nil, fmt.Errorf("start upstream %s: %w", cfg.UpstreamCommand(), err)
	}
	return &Proxy{cfg: cfg, upstream: upstream, audit: audit, stateKey: []byte(rand.Text()), pageScoped: map[string]bool{}, owned: map[int]bool{}, labels: map[int]map[string]string{}, grants: map[string]time.Time{}}, nil
}

func (p *Proxy) Close() error {
	err := p.upstream.Close()
	if auditErr := p.audit.Close(); err == nil {
		err = auditErr
	}
	return err
}

func (p *Proxy) Server(ctx context.Context) (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{Name: "pilotab", Version: version}, nil)
	for tool, err := range p.upstream.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list upstream tools: %w", err)
		}
		if takesPageID(tool.InputSchema) {
			p.pageScoped[tool.Name] = true
		}
		tool.InputSchema = withoutForbiddenArgs(tool.InputSchema)
		server.AddTool(tool, p.handle)
	}
	return server, nil
}

func (p *Proxy) handle(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	started := time.Now()
	name := req.Params.Name
	args := map[string]any{}
	if len(req.Params.Arguments) > 0 {
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return denied(fmt.Sprintf("arguments must be a JSON object: %v", err)), nil
		}
	}
	entry := auditEntry{Time: started, Tool: name, Args: auditArgs(args)}
	finish := func(result *mcp.CallToolResult, decision, reason string) (*mcp.CallToolResult, error) {
		entry.Decision, entry.Reason = decision, reason
		entry.ToolError = result.IsError
		entry.DurationMs = time.Since(started).Milliseconds()
		p.audit.write(entry)
		return result, nil
	}

	decision := "allowed"
	if reason := p.checkNavigation(name, args); reason != "" {
		return finish(denied(reason), "denied", reason)
	}
	if reason := argsReason(args); reason != "" {
		return finish(denied(reason), "denied", reason)
	}
	hasPage := p.pageScoped[name]
	pageID, validPage := pageIDArg(args)
	if hasPage && !validPage {
		reason := "pageId must be a whole number taken from list_pages"
		return finish(denied(reason), "denied", reason)
	}
	if hasPage {
		args["pageId"] = float64(pageID)
		pageURL, reason, err := p.checkPage(ctx, pageID)
		if err != nil {
			return nil, err
		}
		entry.PageURL = pageURL
		if reason == "" {
			reason = p.cfg.scriptReason(name, pageURL)
		}
		if reason != "" {
			return finish(denied(reason), "denied", reason)
		}
		host, _ := hostOf(pageURL)
		switch {
		case !p.cfg.needsConfirmation(name, pageURL):
		case p.cfg.SkipConfirmations:
			decision = "unconfirmed"
		case p.granted(name, host):
			decision = "granted"
		default:
			state := p.actionState(name, args, pageURL)
			response, answered := req.Params.InputResponses[confirmationID]
			if !answered || !hmac.Equal([]byte(req.Params.RequestState), []byte(state)) {
				action := describeAction(name, args, p.labels[pageID])
				return finish(p.askConfirmation(name, action, pageURL, host, state), "asked", "")
			}
			result, ok := response.(*mcp.ElicitResult)
			if !ok || result.Action != "accept" {
				reason := fmt.Sprintf("the user did not allow %s on %s", name, pageURL)
				return finish(denied(reason), "declined", reason)
			}
			decision = "confirmed"
			if result.Content[grantField] == true && p.grantable(name, host) {
				p.grants[host] = time.Now().Add(p.grantDuration())
				entry.Reason = fmt.Sprintf("granted %s for %s", host, p.grantDuration())
			}
		}
	}

	var before map[int]bool
	if name == "new_page" {
		pages, err := p.listPages(ctx)
		if err != nil {
			return nil, err
		}
		before = pageIDs(pages)
	}

	result, err := p.upstream.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return finish(denied(redact(err.Error())), "upstream-error", "")
	}
	if !result.IsError {
		p.trackOwnership(name, args, result, before)
	}
	p.sanitize(result)
	if labels := snapshotLabels(textOf(result)); hasPage && labels != nil {
		p.labels[pageID] = labels
	}
	return finish(result, decision, entry.Reason)
}

func (p *Proxy) checkNavigation(tool string, args map[string]any) string {
	target, _ := args["url"].(string)
	navigates := tool == "new_page" || (tool == "navigate_page" && (args["type"] == nil || args["type"] == "url"))
	if !navigates || target == "" || p.cfg.urlAllowed(target) {
		return ""
	}
	return fmt.Sprintf("navigation to %s is outside allowHosts", target)
}

func (p *Proxy) checkPage(ctx context.Context, pageID int) (pageURL, reason string, err error) {
	pages, err := p.listPages(ctx)
	if err != nil {
		return "", "", err
	}
	if p.cfg.Tabs == TabsOwn && !p.owned[pageID] {
		return "", fmt.Sprintf("page %d was not opened by this session; open one with new_page (tabs: %q)", pageID, p.cfg.Tabs), nil
	}
	for _, pg := range pages {
		if pg.ID != pageID {
			continue
		}
		if !p.cfg.urlAllowed(pg.URL) {
			return pg.URL, fmt.Sprintf("page %d is on %s, outside allowHosts", pageID, pg.URL), nil
		}
		return pg.URL, "", nil
	}
	return "", "", nil
}

func (p *Proxy) trackOwnership(tool string, args map[string]any, result *mcp.CallToolResult, before map[int]bool) {
	switch tool {
	case "new_page":
		var opened []int
		for _, pg := range parsePages(textOf(result)) {
			if !before[pg.ID] {
				opened = append(opened, pg.ID)
			}
		}
		if len(opened) == 1 {
			p.owned[opened[0]] = true
		}
	case "close_page":
		if pageID, ok := pageIDArg(args); ok {
			delete(p.owned, pageID)
			delete(p.labels, pageID)
		}
	}
}

func (p *Proxy) sanitize(result *mcp.CallToolResult) {
	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}
		text.Text = redact(text.Text)
		if p.cfg.Tabs == TabsOwn {
			text.Text = filterPages(text.Text, func(id int) bool { return p.owned[id] })
		}
	}
	result.StructuredContent = nil
}

func (p *Proxy) listPages(ctx context.Context) ([]page, error) {
	result, err := p.upstream.CallTool(ctx, &mcp.CallToolParams{Name: "list_pages", Arguments: map[string]any{}})
	if err != nil {
		return nil, fmt.Errorf("list_pages: %w", err)
	}
	return parsePages(textOf(result)), nil
}

const (
	confirmationID = "confirm"
	grantField     = "allowHost"
)

func (p *Proxy) askConfirmation(tool, action, pageURL, host, state string) *mcp.CallToolResult {
	properties := map[string]any{}
	if p.grantable(tool, host) {
		properties[grantField] = map[string]any{
			"type":    "boolean",
			"title":   fmt.Sprintf("Allow every action on %s for %s", host, p.grantDuration()),
			"default": false,
		}
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{confirmationID: &mcp.ElicitParams{
			Message:         fmt.Sprintf("pilotab wants to %s\non %s", action, pageURL),
			RequestedSchema: map[string]any{"type": "object", "properties": properties},
		}},
		RequestState: state,
	}
}

func (p *Proxy) grantDuration() time.Duration {
	return time.Duration(p.cfg.GrantMinutes) * time.Minute
}

func (p *Proxy) grantable(tool, host string) bool {
	return p.cfg.GrantMinutes > 0 && host != "" && !alwaysConfirmTools[tool]
}

func (p *Proxy) granted(tool, host string) bool {
	return p.grantable(tool, host) && time.Now().Before(p.grants[host])
}

// The state binds a confirmation to this exact action, so an answer cannot be replayed for another one.
func (p *Proxy) actionState(tool string, args map[string]any, pageURL string) string {
	canonical, _ := json.Marshal(map[string]any{"tool": tool, "args": args, "pageUrl": pageURL})
	mac := hmac.New(sha256.New, p.stateKey)
	mac.Write(canonical)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func denied(reason string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "pilotab: " + reason}}}
}

func textOf(result *mcp.CallToolResult) string {
	var parts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func pageIDs(pages []page) map[int]bool {
	ids := make(map[int]bool, len(pages))
	for _, pg := range pages {
		ids[pg.ID] = true
	}
	return ids
}

func takesPageID(schema any) bool {
	object, _ := schema.(map[string]any)
	properties, _ := object["properties"].(map[string]any)
	_, ok := properties["pageId"]
	return ok
}
