package docs

import _ "embed"

// AgentGuide is the offline copy of docs/agent-guide.md embedded in the CLI.
//
//go:embed agent-guide.md
var AgentGuide string
