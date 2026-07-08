package report

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Marshal renders a report as deterministic report.json: indented, with a
// trailing newline, and with the records already in canonical unit order from
// FromDecisions, so two builds of the same input write byte-identical files. HTML
// escaping is off so a span with a < in a path reads as itself.
func Marshal(r Report) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		return nil, fmt.Errorf("report: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// Parse reads a stored report.json back into a Report. It reads any schema
// version the compiler ever emitted (doc 06 section 10.4), so an old report stays
// diffable against a new one; a version newer than this build's is refused rather
// than silently misread.
func Parse(data []byte) (Report, error) {
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return Report{}, fmt.Errorf("report: decode: %w", err)
	}
	if r.Schema > SchemaVersion {
		return Report{}, fmt.Errorf("report: schema version %d is newer than this build understands (%d)", r.Schema, SchemaVersion)
	}
	return r, nil
}
