package pagination

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	want := Cursor{CreatedAt: time.Now().UTC().Truncate(time.Microsecond), ID: uuid.New()}
	got, err := Decode(want.Encode())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) || got.ID != want.ID {
		t.Fatalf("Decode = %+v, want %+v", got, want)
	}
}

func TestCursorRejectsGarbage(t *testing.T) {
	for _, raw := range []string{"", "!!!", "bm90LWEtY3Vyc29y", "MjAyNi0wMS0wMXxub3QtYS11dWlk"} {
		if _, err := Decode(raw); err == nil {
			t.Errorf("Decode(%q) accepted invalid input", raw)
		}
	}
}

func TestOffsetMeta(t *testing.T) {
	meta := OffsetMeta(20, 40, true)
	if meta["next_offset"] != 60 {
		t.Fatalf("next_offset = %v, want 60", meta["next_offset"])
	}
	last := OffsetMeta(20, 40, false)
	if last["next_offset"] != nil {
		t.Fatalf("next_offset = %v, want nil", last["next_offset"])
	}
}

func FuzzCursorDecode(f *testing.F) {
	f.Add(Cursor{CreatedAt: time.Now(), ID: uuid.New()}.Encode())
	f.Add("garbage")
	f.Fuzz(func(t *testing.T, raw string) {
		c, err := Decode(raw)
		if err != nil {
			return
		}
		// Any accepted cursor must re-encode to a decodable cursor.
		if _, err := Decode(c.Encode()); err != nil {
			t.Fatalf("re-encoded cursor does not decode: %v", err)
		}
	})
}

func BenchmarkCursorEncode(b *testing.B) {
	c := Cursor{CreatedAt: time.Now(), ID: uuid.New()}
	for i := 0; i < b.N; i++ {
		_ = c.Encode()
	}
}
