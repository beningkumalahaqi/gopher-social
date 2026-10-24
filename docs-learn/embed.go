// Package curriculum embeds the canonical Markdown curriculum.
//
// The Markdown files remain the source of truth. Nothing here transforms them;
// the learning platform's loader parses this filesystem at startup, so a
// deployed binary serves exactly the lessons that were committed.
package curriculum

import "embed"

// FS holds every lesson, exercise, solution and runnable example.
//
//go:embed *.md exercises/*.md solutions/*.md examples/*/*.go
var FS embed.FS
