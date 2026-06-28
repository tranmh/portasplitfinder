// Package migrations embeds the goose SQL migration files so they can be run
// programmatically (on boot, in --seed, and in tests).
package migrations

import "embed"

// FS holds the embedded migration SQL files.
//
//go:embed *.sql
var FS embed.FS
