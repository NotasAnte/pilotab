# pilotab

Lets Claude Code drive the developer's real Chrome and guards it:
Claude ⇄ `pilotab` (Go proxy) ⇄ `chrome-devtools-mcp` (Node, pinned) ⇄ Chrome.

## Setup
1. Go 1.26+ and Node from `.nvmrc`, then `make install`.
2. Enable remote debugging in `chrome://inspect/#remote-debugging` while you use it; turn it off after.
3. Run `claude` here and approve the `pilotab` MCP.

## Layout
- `config/pilot.json` — shipped defaults; `config/pilot.local.json` (git-ignored) or `PILOTAB_CONFIG`
  overrides it. Proxy policy: `allowHosts`, `confirmHosts`, `tabs` (`own`|`all`), `grantMinutes`
  (0 turns host grants off), `auditLog`.
- `config/chrome-devtools.json` — upstream options (`--config`, read at start); upstream rejects unknown keys.
- `bin/pilotab.sh` builds and runs the proxy; `bin/chrome-devtools-mcp.sh` picks a supported Node.
- Confirmations use MCP multi round-trip (SEP-2322): the tool returns `InputRequests` plus an HMAC
  `RequestState` bound to tool + args + page URL. A client that cannot answer fails closed.
- The confirmation names the action from the page's last snapshot (`click button "Delete"`, typed text
  clipped, password-like fields hidden) and can grant every action on that host for `grantMinutes`.
  `upload_file` is never granted.
- `evaluate_script` is blocked on confirmHosts, even in dangerous mode; `initScript` and `isolatedContext`
  names outside `[A-Za-z0-9_-]` are rejected everywhere. `pageId` must be a whole number: the checked page
  is the page that runs.
- File-output args (`filePath`, `requestFilePath`, `responseFilePath`, `outputDirPath`, `sourcePath`) are
  rejected and hidden from tool schemas: files would bypass redaction.
- Text results are redacted (auth headers, cookies, JWTs, API keys). Screenshots are not.
- `chrome-devtools-mcp` and `go-sdk` are pinned; bump them on purpose.

## Commands
- `make test` — unit tests + e2e against headless, isolated Chrome.
- `make test-live` — e2e against your open Chrome.
- `make claude-dangerous [ARGS=...]` — start `claude` with pilotab confirmations off
  (`PILOTAB_DANGEROUSLY_SKIP_CONFIRMATIONS=1`). allowHosts, tab scoping, audit and redaction stay on;
  the audit log records those calls as `unconfirmed`. Only an env var at launch turns it on, never the config.
  Policy files themselves are editable: review agent edits to them.
