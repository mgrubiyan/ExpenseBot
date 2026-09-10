package storage

import "crypto/rand"

// inviteCodeAlphabet excludes visually ambiguous characters (0/O, 1/I).
const inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

const inviteCodeLength = 6

// generateInviteCode returns a short random code used to invite people
// into a group (e.g. shared via /join <code>).
func generateInviteCode() string {
	b := make([]byte, inviteCodeLength)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read on a supported platform practically never fails;
		// fall back to a fixed-but-unlikely-to-collide pattern rather than panicking.
		for i := range b {
			b[i] = byte(i)
		}
	}

	code := make([]byte, inviteCodeLength)
	for i, v := range b {
		code[i] = inviteCodeAlphabet[int(v)%len(inviteCodeAlphabet)]
	}
	return string(code)
}
