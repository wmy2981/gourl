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
	for _, code := range []string{"api/v1/links", "ADMIN/foo"} {
		if !IsReserved(code, nil) {
			t.Errorf("%q should be reserved via first segment", code)
		}
	}
	// Normal codes are fine.
	for _, code := range []string{"abc", "a1b2c3", "my-link", "docs/readme", "link1/link2"} {
		if IsReserved(code, nil) {
			t.Errorf("%q should not be reserved", code)
		}
	}
}

func TestIsReservedExtra(t *testing.T) {
	extra := []string{"foo", "Bar"}
	for _, code := range []string{"foo", "FOO", "bar", "foo/x"} {
		if !IsReserved(code, extra) {
			t.Errorf("%q should be reserved via extra list", code)
		}
	}
	if IsReserved("baz", extra) {
		t.Error("baz should not be reserved")
	}
}

func TestValidate(t *testing.T) {
	valid := []string{"abc", "a1b2c3", "my-link", "my_link", "link1/link2", "a/b/c/d/e"}
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
		{"abc?x", "query char"},
	}
	for _, tc := range invalid {
		if err := Validate(tc.code); err == nil {
			t.Errorf("Validate(%q) should fail (%s)", tc.code, tc.why)
		}
	}
}
