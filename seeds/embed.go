// Package seeds embeds the development seed SQL.
package seeds

import _ "embed"

// SQL is the destructive development seed script.
//
//go:embed seed.sql
var SQL string
