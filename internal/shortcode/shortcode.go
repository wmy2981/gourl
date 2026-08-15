// Package shortcode generates random short codes and validates codes and
// reserved prefixes.
package shortcode

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

// alphabet is base62 (digits, uppercase, lowercase).
const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// MaxSegments limits how many levels a custom code may have (link1/link2/...).
const MaxSegments = 5

// MaxLength caps the total length of a code.
const MaxLength = 64

// builtinReserved are system paths that must never be shadowed by short
// codes. Compared case-insensitively against the first segment only, so a
// multi-level code like "api/foo" is also rejected.
var builtinReserved = []string{
	"api", "admin", "expired", "health", "assets", "favicon",
	"export", "login", "logout", "config", "icon", "dashboard",
	"links", "tokens", "ua-blocks", "settings", "static", "docs",
}

var errNotEnoughEntropy = errors.New("crypto/rand failure")

// Random returns a single-segment base62 code of length n.
func Random(n int) (string, error) {
	if n < 1 {
		return "", fmt.Errorf("shortcode length must be >= 1, got %d", n)
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("%w: %v", errNotEnoughEntropy, err)
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

// IsReserved reports whether the first segment of code matches the built-in
// reserved list or any extra reserved prefix, case-insensitively.
func IsReserved(code string, extra []string) bool {
	seg := code
	if i := strings.IndexByte(code, '/'); i >= 0 {
		seg = code[:i]
	}
	for _, r := range builtinReserved {
		if strings.EqualFold(seg, r) {
			return true
		}
	}
	for _, r := range extra {
		if strings.EqualFold(seg, r) {
			return true
		}
	}
	return false
}

// Validate checks a custom code: non-empty, url-safe characters, at most
// MaxSegments levels, each segment non-empty, total length <= MaxLength.
func Validate(code string) error {
	if code == "" {
		return errors.New("code must not be empty")
	}
	if len(code) > MaxLength {
		return fmt.Errorf("code too long: %d chars, max %d", len(code), MaxLength)
	}
	segs := strings.Split(code, "/")
	if len(segs) > MaxSegments {
		return fmt.Errorf("code has %d segments, max %d", len(segs), MaxSegments)
	}
	for _, s := range segs {
		if s == "" {
			return errors.New("code segments must not be empty")
		}
		for _, r := range s {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				return fmt.Errorf("invalid character %q in code", r)
			}
		}
	}
	return nil
}
