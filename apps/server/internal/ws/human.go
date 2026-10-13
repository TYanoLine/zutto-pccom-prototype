package ws

import (
	"crypto/sha256"
	"encoding/hex"
)

// humanID derives a user's human ID from the key their browser keeps (spec
// 019). This is provisional, for as long as users are not identified; the
// account's identifier will replace it once users sign in. The key is a bearer secret and is never stored or logged; only a
// one-way digest is. Without a well-formed key the call gets an ID of its own.
func humanID(callerKey string) string {
	if isCallerKey(callerKey) {
		sum := sha256.Sum256([]byte("zutto-human:" + callerKey))
		return "h" + hex.EncodeToString(sum[:8])
	}
	id, err := newSessionID()
	if err != nil {
		return ""
	}
	return "h" + id[:16]
}

// isCallerKey reports whether s is 32 lowercase hexadecimal digits.
func isCallerKey(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
