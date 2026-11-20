// Package migrations embeds the ordered SQLite schema migrations.
// Add numbered SQL files such as 0001_create_items.sql in this directory.
package migrations

import "embed"

// Files holds all files in migrations/; database.Migrate applies every *.sql file in order.
//
//go:embed *
var Files embed.FS
