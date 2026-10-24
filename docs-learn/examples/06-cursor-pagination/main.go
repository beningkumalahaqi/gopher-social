// Command cursor-pagination walks a list with opaque keyset cursors.
//
//	go run ./docs-learn/examples/06-cursor-pagination
package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/pagination"
)

type post struct {
	ID        uuid.UUID
	CreatedAt time.Time
	Title     string
}

func main() {
	now := time.Now().UTC()
	posts := make([]post, 0, 10)
	for i := 0; i < 10; i++ {
		posts = append(posts, post{
			ID:        uuid.New(),
			CreatedAt: now.Add(-time.Duration(i) * time.Minute),
			Title:     fmt.Sprintf("post %d", i),
		})
	}
	sort.Slice(posts, func(i, j int) bool {
		if posts[i].CreatedAt.Equal(posts[j].CreatedAt) {
			return posts[i].ID.String() > posts[j].ID.String()
		}
		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})

	const limit = 3
	cursor := ""
	for page := 1; ; page++ {
		items, next, err := list(posts, cursor, limit)
		if err != nil {
			fmt.Println("error:", err)
			return
		}
		fmt.Printf("page %d: %v\n", page, titles(items))
		if next == "" {
			break
		}
		cursor = next
	}
}

// list returns at most limit items after cursor, plus the next cursor.
func list(all []post, cursor string, limit int) ([]post, string, error) {
	start := 0
	if cursor != "" {
		c, err := pagination.Decode(cursor)
		if err != nil {
			return nil, "", err
		}
		for i, p := range all {
			if p.CreatedAt.Before(c.CreatedAt) ||
				(p.CreatedAt.Equal(c.CreatedAt) && p.ID.String() < c.ID.String()) {
				start = i
				break
			}
			start = len(all)
		}
	}

	end := min(start+limit, len(all))
	items := all[start:end]
	if end == len(all) {
		return items, "", nil
	}
	last := items[len(items)-1]
	next := pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.Encode()
	return items, next, nil
}

func titles(posts []post) []string {
	out := make([]string, len(posts))
	for i, p := range posts {
		out[i] = p.Title
	}
	return out
}
