# pilotab

Let your coding agent use **your** browser — with the sessions you are already signed into — without
handing it the keys.

pilotab is an [MCP](https://modelcontextprotocol.io) server that sits in front of Google's
[chrome-devtools-mcp](https://github.com/ChromeDevTools/chrome-devtools-mcp) and enforces a policy you
own:

```
agent ⇄ pilotab (policy) ⇄ chrome-devtools-mcp ⇄ your browser
```

| Guard | What it does |
| --- | --- |
| Host allowlist | `allowHosts` gates every navigation and every action on a page. |
| Own tabs only | With `tabs: "own"` the agent sees and touches only the tabs it opened. They share your cookies. |
| Confirmations | Writes (`click`, `fill`, `type_text`…) on `confirmHosts`, and every `upload_file`, ask you first, naming the element: *wants to click button "Delete repository"*. Password-like fields are hidden. |
| Host grants | Answer once with "allow every action on this host for 10 minutes". Uploads are never granted. |
| No scripts where it matters | `evaluate_script` is blocked on `confirmHosts`; `initScript` is blocked everywhere, since it also runs after redirects. |
| Redaction | Auth headers, cookies, JWTs and common API keys are stripped from text results. File-output arguments are rejected so nothing bypasses it. |
| Audit log | Every call, decision and page URL goes to a JSONL file (`0600`); typed values are never logged. |

Confirmations use MCP multi round-trip requests (protocol `2026-07-28`) with an HMAC that binds each
answer to one exact action; older clients get a plain elicitation (handled by the Go SDK). A client that can do neither is
refused, never waved through.

## Requirements

- Go 1.26+ and Node.js `^20.19 || ^22.12 || >=23` (`.nvmrc` pins 24).
- Chrome 144+ (or another browser chrome-devtools-mcp supports).
- An MCP client. Tested with Claude Code.

## Setup

```sh
git clone https://github.com/NotasAnte/pilotab
cd pilotab
make install
```

1. In Chrome, open `chrome://inspect/#remote-debugging` and turn remote debugging on **while you use
   pilotab**. Chrome asks you to allow the connection. Turn it off when you are done.
2. Copy `config/pilot.json` to `config/pilot.local.json` (git-ignored) and set your hosts.
3. Run `claude` in this folder and approve the `pilotab` MCP. From another project, add to its
   `.mcp.json`:

   ```json
   "pilotab": { "command": "/absolute/path/to/pilotab/bin/pilotab.sh", "args": [] }
   ```

   and copy `.claude/skills/pilotab` into that project.

## Configuration

`config/pilot.json`:

| Key | Meaning |
| --- | --- |
| `upstream` | How to start chrome-devtools-mcp; relative paths resolve against the config folder. |
| `tabs` | `own` (default) or `all`. |
| `allowHosts` | Host globs the agent may visit, e.g. `["github.com", "*.github.com"]`. `["*"]` allows all. |
| `confirmHosts` | Host globs where writes need your confirmation and scripts are blocked. |
| `grantMinutes` | Length of a host grant; `0` turns grants off. |
| `auditLog` | JSONL path; `~` expands. Empty disables it. |

`config/chrome-devtools.json` is passed to chrome-devtools-mcp (`--config`): it connects to your running
browser, turns usage statistics and CrUX lookups off, redacts sensitive network headers, blocks `file:`
navigations and drops tool categories the agent does not need. Config changes apply on restart.

To run without confirmations (allowlist, tab scope, audit and redaction stay on):

```sh
make claude-dangerous            # or PILOTAB_DANGEROUSLY_SKIP_CONFIRMATIONS=1 claude
```

It can only be switched on from the environment at launch, not from a config file.

## Tests

```sh
make test        # unit + end-to-end against a headless, isolated Chrome
make test-live   # end-to-end against your running browser
```

## Limits

- The allowlist reduces what the agent can reach; it is not a sandbox.
- Web pages can carry prompt injection. Confirmations and grants are your backstop.
- Screenshots are not redacted.
- Element names in confirmations come from the page's accessibility tree, which the page controls.
- The policy lives in files inside this folder. If your agent works in this repo and may edit files, it can
  change the policy for the next start: review edits to `config/`, `bin/`, `cmd/` and `internal/`.

## License

[Apache-2.0](LICENSE). Security issues: see [SECURITY.md](SECURITY.md).
