// Command seed loads development data. It is destructive: it truncates all
// application tables before inserting the fixture rows.
//
//	go run ./cmd/seed
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/haqi/gophersocial/seeds"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgresql://admin:password@localhost:5432/gopherdb"
	}

	db, err := sql.Open("pgx", url)
	if err != nil {
		fail("open database", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, seeds.SQL); err != nil {
		fail("apply seed", err)
	}
	fmt.Println("seed: ok (all seeded users have password Password123!)")
}

func fail(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
