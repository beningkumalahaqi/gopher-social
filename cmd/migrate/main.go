// Command migrate applies the embedded SQL migrations.
//
//	go run ./cmd/migrate -dir=up
//	go run ./cmd/migrate -dir=down
//	go run ./cmd/migrate -dir=version
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/haqi/gophersocial/internal/database"
)

func main() {
	dir := flag.String("dir", "up", "up, down, drop, version or force")
	version := flag.Int("version", -1, "target version for -dir=force")
	flag.Parse()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgresql://admin:password@localhost:5432/gopherdb"
	}
	var err error
	if *dir == "force" {
		if *version < 0 {
			fmt.Fprintln(os.Stderr, "migrate: -dir=force requires -version")
			os.Exit(2)
		}
		err = database.MigrateForce(url, *version)
	} else {
		err = database.Migrate(url, *dir)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
	fmt.Println("migrate:", *dir, "ok")
}
