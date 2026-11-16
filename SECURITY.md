[Documentation index](docs/index.md) · [Русский](SECURITY.ru.md)

# Security policy

## Reporting a vulnerability

Report privately through [GitHub Security Advisories](https://github.com/akrhin/portainer-mcp/security/advisories/new). The advisory stays private until a fix is released.

Do not open a public issue, discussion or pull request that describes the problem, the affected tool or the steps to reproduce it. Public reports are closed without discussion and moved to a private advisory.

Include what you can: the version or commit, the transport you used (stdio or HTTP), the configuration shape (toolsets, `read_only`, instance count), the tool name, and the smallest request that shows the problem.

## What counts as a vulnerability here

This server holds Portainer API keys and acts with their permissions. The following are security issues, not bugs:

- An API key, `ca_file` content or other secret appearing in a tool response, an error message, a log line or a stack trace.
- Server-side request forgery: a request that makes the server call a host the operator did not configure, including through redirects or through a URL-shaped argument.
- Path traversal or injection through tool arguments that reach a Portainer path, a file path on the server host, or a shell.
- Privilege escalation through `read_only`: any write tool that remains registered or callable when `server.read_only: true`. Read-only mode is enforced at registration time, so a callable write tool is a defect by definition.
- Authentication bypass or tenant confusion on the HTTP transport: a request acting with another client's API key, or with no key at all.
- A tool or annotation that lies about its effect — a write tool declared `readOnlyHint: true`, or a destructive tool without `destructiveHint: true`.

Out of scope: vulnerabilities in Portainer itself (report those to Portainer), vulnerabilities in the MCP client, and misconfiguration of an operator's own instance URL or API key permissions.

## Supported versions

The latest minor release and the `main` branch receive fixes. Older minors do not.

There is no SLA. This is a small project, and responses are best effort.

## Response times

- Acknowledgement of a private report: within 72 hours.
- Assessment and a fix plan: as soon as the report can be reproduced, typically within a week.
- A fix is released as a patch version with an entry in [CHANGELOG.md](CHANGELOG.md); the advisory is published at the same time.

## Disclosure

Do not disclose the issue publicly before a fix is released. Once a patch is out, the advisory is published with the affected versions, the impact and the workaround if one exists. Credit is given in the advisory if you want it.
