package bot

import "sync"

// sessionState is where a user currently is in a button-driven,
// multi-step flow (e.g. "add a group expense").
type sessionState int

const (
	stateNone sessionState = iota
	stateAwaitingGroupName
	stateAwaitingAddAmount
	stateAwaitingAddDescription
	stateAwaitingSettleAmount
)

// session holds the in-progress state for one Telegram user across a
// sequence of button taps and plain-text replies. It's intentionally
// in-memory only (lost on restart) - state is short-lived by nature
// (a few messages long), so this keeps things simple.
type session struct {
	state sessionState

	groupID int64

	// selected holds internal user IDs chosen as participants for the
	// current "add expense" flow. Empty/nil means "everyone in the group".
	selected map[int64]bool

	// amount is the parsed expense amount, set once the amount step
	// completes and carried into the description step.
	amount int64

	// settleTargetID is who the current user picked to settle up with.
	settleTargetID int64
}

type sessionStore struct {
	mu       sync.Mutex
	sessions map[int64]*session
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[int64]*session)}
}

// get returns the live session for a Telegram user, creating an empty one
// on first use. The returned pointer can be mutated directly by callers.
func (s *sessionStore) get(tgUserID int64) *session {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[tgUserID]
	if !ok {
		sess = &session{}
		s.sessions[tgUserID] = sess
	}
	return sess
}

// reset abandons whatever flow a user was in the middle of.
//
// It zeroes the existing session struct in place (rather than swapping in
// a new pointer) so that a caller holding an older *session from get()
// earlier in the same request sees the reset too, instead of writing to a
// now-orphaned copy.
func (s *sessionStore) reset(tgUserID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.sessions[tgUserID]; ok {
		*sess = session{}
		return
	}
	s.sessions[tgUserID] = &session{}
}
