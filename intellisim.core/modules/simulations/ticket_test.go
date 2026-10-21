package simulations

import (
	"testing"
	"time"
)

// fixedNow is declared in memory_test.go (package-level) and reused here.

func TestTicketStore_IssueAndConsumeOnce(t *testing.T) {
	ts := NewTicketStore(30 * time.Second)
	tok := ts.Issue("user1", "sim1", fixedNow)
	uid, ok := ts.Consume(tok, "sim1", fixedNow)
	if !ok || uid != "user1" {
		t.Fatal("first consume should succeed for user1")
	}
	if _, ok := ts.Consume(tok, "sim1", fixedNow); ok {
		t.Fatal("ticket must be single-use")
	}
}

func TestTicketStore_RejectsExpired(t *testing.T) {
	ts := NewTicketStore(30 * time.Second)
	tok := ts.Issue("u", "s", fixedNow)
	if _, ok := ts.Consume(tok, "s", fixedNow.Add(31*time.Second)); ok {
		t.Fatal("expired ticket accepted")
	}
}

func TestTicketStore_RejectsWrongSim(t *testing.T) {
	ts := NewTicketStore(30 * time.Second)
	tok := ts.Issue("u", "s1", fixedNow)
	if _, ok := ts.Consume(tok, "s2", fixedNow); ok {
		t.Fatal("ticket bound to s1 accepted for s2")
	}
}

func TestTicketStore_IssueReturnsOpaqueUniqueTokens(t *testing.T) {
	ts := NewTicketStore(30 * time.Second)
	a := ts.Issue("u", "s", fixedNow)
	b := ts.Issue("u", "s", fixedNow)
	if a == "" || b == "" {
		t.Fatal("issued tokens must be non-empty")
	}
	if a == b {
		t.Fatal("issued tokens must be unique")
	}
}

func TestTicketStore_ConsumeUnknownTokenFails(t *testing.T) {
	ts := NewTicketStore(30 * time.Second)
	if _, ok := ts.Consume("does-not-exist", "s", fixedNow); ok {
		t.Fatal("unknown token must not be accepted")
	}
}

func TestTicketStore_PurgesExpiredOnAccess(t *testing.T) {
	ts := NewTicketStore(30 * time.Second)
	// Issue a ticket that will be expired by the time of the next access.
	ts.Issue("u", "s", fixedNow)
	// Issuing again well past the first ticket's expiry triggers lazy purge.
	ts.Issue("u2", "s2", fixedNow.Add(time.Hour))
	if n := ts.Len(); n != 1 {
		t.Fatalf("expected 1 live ticket after lazy purge, got %d", n)
	}
}
