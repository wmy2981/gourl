package shortcode

import (
	"strings"
	"testing"
)

func TestRandomLengthAndAlphabet(t *testing.T) {
	for n := 1; n <= 12; n++ {
		code, err := Random(n)
		if err != nil {
			t.Fatalf("Random(%d): %v", n, err)
		}
		if len(code) != n {
			t.Errorf("Random(%d) = %q, length %d", n, code, len(code))
		}
		for _, r := range code {
			if !strings.ContainsRune(alphabet, r) {
				t.Errorf("Random(%d) = %q contains %q outside alphabet", n, code, r)
			}
		}
	}
}

func TestRandomRejectsZeroLength(t *testing.T) {
	if _, err := Random(0); err == nil {
		t.Fatal("Random(0) should fail")
	}
}

func TestRandomDoesNotCollide(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code, err := Random(6)
		if err != nil {
			t.Fatal(err)
		}
		if seen[code] {
			t.Fatalf("collision: %q", code)
		}
		seen[code] = true
	}
}

func TestIsReservedBuiltin(t *testing.T) {
	for _, code := range []string{"api", "API", "Api", "admin", "health", "expired", "assets", "favicon"} {
		if !IsReserved(code, nil) {
			t.Errorf("%q should be reserved", code)
		}
	}
	// First segment only.
	for _, code := range []string{"api/v1/links", "ADMIN/foo", "docs/readme"} {
		if !IsReserved(code, nil) {
			t.Errorf("%q should be reserved via first segment", code)
		}
	}
	// Normal codes are fine.
	for _, code := range []string{"abc", "a1b2c3", "my-link", "link1/link2"} {
		if IsReserved(code, nil) {
			t.Errorf("%q should not be reserved", code)
		}
	}
}

func TestIsReservedExtra(t *testing.T) {
	extra := []string{"foo", "Bar", "中文", "help/中文", "x/y"}
	// Single-segment entries match the first segment, case-insensitively.
	for _, code := range []string{"foo", "FOO", "bar", "BAR", "foo/x", "foo/a/b"} {
		if !IsReserved(code, extra) {
			t.Errorf("%q should be reserved via extra list", code)
		}
	}
	// Chinese single-segment entries match their exact segment.
	for _, code := range []string{"中文", "中文/x"} {
		if !IsReserved(code, extra) {
			t.Errorf("%q should be reserved via Chinese entry", code)
		}
	}
	// Multi-segment entries reserve the full code and everything below it.
	for _, code := range []string{"help/中文", "help/中文/a", "x/y", "x/y/z", "X/Y/abc"} {
		if !IsReserved(code, extra) {
			t.Errorf("%q should be reserved via multi-segment entry", code)
		}
	}
	// Prefix matching respects segment boundaries.
	for _, code := range []string{"x/yz", "x/z", "x", "help/中文abc"} {
		if IsReserved(code, extra) {
			t.Errorf("%q should not be reserved", code)
		}
	}
	if IsReserved("baz", extra) {
		t.Error("baz should not be reserved")
	}
}

func TestValidate(t *testing.T) {
	valid := []string{
		"abc", "a1b2c3", "my-link", "my_link", "link1/link2", "a/b/c/d/e",
		// Simplified Chinese codes.
		"中文", "短链", "中文/子码", "混合abc-123", "链接_2026",
	}
	for _, code := range valid {
		if err := Validate(code); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", code, err)
		}
	}

	invalid := []struct {
		code string
		why  string
	}{
		{"", "empty"},
		{"a b", "space"},
		{"ab/cd/ef/gh/ij/kl", "too many segments"},
		{"/abc", "leading empty segment"},
		{"abc/", "trailing empty segment"},
		{"abc//def", "double slash"},
		{strings.Repeat("a", 65), "too long"},
		{strings.Repeat("字", 65), "too long in runes"},
		{"abc?x", "query char"},
		{"码 ", "trailing space"},
		{"emoji😀", "outside CJK"},
	}
	// 64 runes is fine (the limit counts characters, not UTF-8 bytes).
	if err := Validate(strings.Repeat("字", 64)); err != nil {
		t.Errorf("Validate(64 runes) = %v, want nil", err)
	}
	for _, tc := range invalid {
		if err := Validate(tc.code); err == nil {
			t.Errorf("Validate(%q) should fail (%s)", tc.code, tc.why)
		}
	}
}
