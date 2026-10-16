package scripts

import "embed"

// FS contains the audited installation engine and its libraries.
//
//go:embed install.sh lib/*.sh catalog.json timezones.json
var FS embed.FS
