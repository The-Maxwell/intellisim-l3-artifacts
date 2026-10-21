package simulations

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// ticketBytes is the number of random bytes in a connect-ticket token (256-bit,
// hex-encoded to a 64-char opaque string).
const ticketBytes = 32

// ticketEntry binds an issued connect ticket to the principal and simulation it
// was minted for, plus its expiry. Tickets are single-use.
type ticketEntry struct {
	userID  string
	simID   string
	expires time.Time
}

// TicketStore is an in-memory store of short-lived, single-use WebSocket connect
// tickets. A REST caller (authenticated + ownership-gated) mints a ticket bound
// to (userID, simID); the WebSocket upgrade handler later consumes it to prove
// authorization without putting a JWT in the URL.
//
// In-memory only by design: tickets are ephemeral (~30s) and a process restart
// simply invalidates every outstanding ticket, which is the safe default.
type TicketStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	tickets map[string]ticketEntry
}

// NewTicketStore creates a TicketStore whose issued tickets live for ttl.
func NewTicketStore(ttl time.Duration) *TicketStore {
	return &TicketStore{
		ttl:     ttl,
		tickets: make(map[string]ticketEntry),
	}
}

// Issue mints a fresh opaque token bound to (userID, simID), expiring ttl after
// now, and records it. now is injected for testability; production passes
// time.Now(). The returned token is a hex-encoded crypto/rand value.
func (s *TicketStore) Issue(userID, simID string, now time.Time) string {
	token := newTicketToken()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(now)
	s.tickets[token] = ticketEntry{
		userID:  userID,
		simID:   simID,
		expires: now.Add(s.ttl),
	}
	return token
}

// Consume validates token against simID at time now and, on success, deletes it
// (single-use) and returns the bound userID. It fails (ok=false) when the token
// is unknown, has expired, or was minted for a different simulation.
func (s *TicketStore) Consume(token, simID string, now time.Time) (userID string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(now)

	entry, found := s.tickets[token]
	if !found {
		return "", false
	}
	// purgeLocked already removed expired entries, but guard explicitly so a
	// caller passing a now equal to the issue time still sees a valid ticket
	// and an exactly-expired one is rejected.
	if !now.Before(entry.expires) {
		delete(s.tickets, token)
		return "", false
	}
	if entry.simID != simID {
		return "", false
	}
	delete(s.tickets, token)
	return entry.userID, true
}

// Len reports the number of live (non-purged) tickets. Primarily for tests.
func (s *TicketStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.tickets)
}

// purgeLocked removes every entry that has expired as of now. Caller must hold mu.
func (s *TicketStore) purgeLocked(now time.Time) {
	for token, entry := range s.tickets {
		if !now.Before(entry.expires) {
			delete(s.tickets, token)
		}
	}
}

// newTicketToken returns a hex-encoded cryptographically-random opaque token.
func newTicketToken() string {
	var b [ticketBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is fatal-grade; degrade to a still-unique but
		// time-derived token rather than panicking inside a request path.
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b[:])
}
