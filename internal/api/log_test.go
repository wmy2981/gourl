package api

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// TestParseJSONAttrs covers the request-log body mirroring: JSON responses
// flatten into "key=value" attrs, nested objects join with dots, arrays
// collapse to their item count, and anything else is dropped.
func TestParseJSONAttrs(t *testing.T) {
	cases := []struct {
		name, body string
		want       []string
	}{
		{
			"error envelope",
			`{"error":{"code":"invalid_config","message":"bad field"}}`,
			[]string{"error.code=invalid_config", "error.message=bad field"},
		},
		{
			"flat object",
			`{"ok":true}`,
			[]string{"ok=true"},
		},
		{
			"numbers and nulls",
			`{"total":42,"note":null}`,
			[]string{"note=null", "total=42"},
		},
		{
			"arrays collapse",
			`{"links":[{"code":"a"},{"code":"b"}],"total":2}`,
			[]string{"links=[2 items]", "total=2"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attrs, ok := parseJSONAttrs(tc.body)
			if !ok {
				t.Fatalf("parseJSONAttrs(%q): expected ok", tc.body)
			}
			got := make([]string, 0, len(attrs)/2)
			for i := 0; i < len(attrs); i += 2 {
				got = append(got, fmt.Sprintf("%v=%v", attrs[i], attrs[i+1]))
			}
			sort.Strings(got)
			sort.Strings(tc.want)
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Errorf("attrs = %v, want %v", got, tc.want)
			}
		})
	}

	// HTML pages, arrays at the top level, unparseable and empty bodies are
	// never mirrored into the log.
	for _, bad := range []string{"", "<html>oops</html>", "[1,2]", "not json", `{"broken"`} {
		if _, ok := parseJSONAttrs(bad); ok {
			t.Errorf("parseJSONAttrs(%q): expected failure", bad)
		}
	}
}
