// Package migrations embeds the SQL migration files so the binary can migrate
// a database without relying on the working directory or an external binary.
package migrations

import "embed"

// FS holds every .sql file in this directory, named
// {version}_{name}.{up|down}.sql as golang-migrate expects.
//
//go:embed *.sql
var FS embed.FS
