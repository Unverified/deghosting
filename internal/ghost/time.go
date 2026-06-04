package ghost

import (
	"encoding/json/jsontext"
	"fmt"
	"time"
)

// ghostTimeLayout is the timestamp format Ghost writes in its JSON export:
// space-separated date and time in UTC, e.g. "2020-05-19 12:03:00". This is not
// RFC 3339, so the default time.Time JSON decoding cannot parse it.
const ghostTimeLayout = "2006-01-02 15:04:05"

// Time wraps time.Time so Ghost's non-RFC3339 export timestamps decode cleanly.
// Embedding time.Time promotes all its methods (IsZero, Format, Year, ...), so
// callers use a ghost.Time almost exactly like a time.Time.
//
// A JSON null — which Ghost uses for unpublished drafts' published_at — decodes
// to the zero Time, detectable with t.IsZero().
type Time struct {
	time.Time
}

// UnmarshalJSONFrom implements the json/v2 [json.UnmarshalerFrom] interface for
// Ghost's timestamp format. It accepts a JSON null or empty string as the zero
// Time, and otherwise parses the "2006-01-02 15:04:05" layout in UTC.
func (t *Time) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}

	switch tok.Kind() {
	case 'n': // JSON null — Ghost uses this for unpublished drafts' published_at.
		t.Time = time.Time{}
		return nil
	case '"':
		s := tok.String()
		if s == "" {
			t.Time = time.Time{}
			return nil
		}

		parsed, err := time.Parse(ghostTimeLayout, s)
		if err != nil {
			return fmt.Errorf("parse ghost time %q: %w", s, err)
		}

		t.Time = parsed

		return nil
	default:
		return fmt.Errorf("parse ghost time: expected JSON string or null, got %s", tok.Kind())
	}
}
