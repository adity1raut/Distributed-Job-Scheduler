package httpx

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Cursor is a keyset pagination cursor over (created_at, id) — cheap to seek
// on the covering index regardless of how deep the page is, unlike OFFSET.
type Cursor struct {
	CreatedAt time.Time `json:"t"`
	ID        uuid.UUID `json:"i"`
}

func (c Cursor) Encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func DecodeCursor(s string) (*Cursor, bool) {
	if s == "" {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	var c Cursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, false
	}
	return &c, true
}

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

func ClampLimit(limit int) int {
	if limit <= 0 {
		return DefaultPageLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit
	}
	return limit
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}
