# Security policy

pilotab guards an agent that acts inside your signed-in browser, so we treat bypasses of its policy as
vulnerabilities: reaching a host outside `allowHosts`, acting on a tab the agent did not open, a write on a
`confirmHosts` page without confirmation, a confirmation replayed for another action, or credentials
leaking past redaction or into the audit log.

## Reporting

Report privately through GitHub: **Security → Report a vulnerability** on this repository. Please do not
open a public issue.

Include the pilotab commit, your config, the MCP client and the steps or tool calls that reproduce it.
We aim to acknowledge reports within five working days.

## Supported versions

Only the latest commit on `main` is supported.
