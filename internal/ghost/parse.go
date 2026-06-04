package ghost

import (
	"encoding/json/v2"
	"fmt"
	"io"
)

// Parse reads a Ghost JSON export from r and decodes it into an Export. It does
// not interpret or transform the content; callers get the raw export structure
// to work with. A malformed-but-valid export (missing fields, null values, or
// sparse join tables) decodes successfully, with absent data left as zero values.
func Parse(r io.Reader) (*Export, error) {
	var export Export
	if err := json.UnmarshalRead(r, &export); err != nil {
		return nil, fmt.Errorf("decode ghost export: %w", err)
	}

	return &export, nil
}
