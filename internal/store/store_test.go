package store

import (
	"testing"
	"time"
)

func TestRequestLifecycleAndExpiryQueries(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	pending := &Request{
		ID:               "p1",
		Username:         "alice",
		Justification:    "need access now",
		Status:           StatusPending,
		CreatedAt:        now.Add(-2 * time.Hour),
		RequestExpiresAt: now.Add(-time.Minute),
	}
	if err := s.Create(pending); err != nil {
		t.Fatal(err)
	}
	exp := now.Add(-time.Minute)
	active := &Request{
		ID:               "a1",
		Username:         "carol",
		Justification:    "need access now",
		Status:           StatusApproved,
		CreatedAt:        now.Add(-10 * time.Hour),
		RequestExpiresAt: now.Add(-9 * time.Hour),
		ApprovedBy:       "bob",
		ApprovalNotes:    "ok for the window",
		ActiveExpiresAt:  &exp,
	}
	if err := s.Create(active); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get("p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "alice" || got.Status != StatusPending {
		t.Fatalf("get=%+v", got)
	}

	expiredPending, err := s.PendingExpired(now)
	if err != nil {
		t.Fatal(err)
	}
	if len(expiredPending) != 1 || expiredPending[0].ID != "p1" {
		t.Fatalf("pending expired=%v", expiredPending)
	}

	expiredActive, err := s.ActiveExpired(now)
	if err != nil {
		t.Fatal(err)
	}
	if len(expiredActive) != 1 || expiredActive[0].ID != "a1" {
		t.Fatalf("active expired=%v", expiredActive)
	}

	if err := s.AppendAudit(&AuditEvent{Action: "request_created", Actor: "alice", Subject: "alice", RequestID: "p1"}); err != nil {
		t.Fatal(err)
	}
	events, err := s.ListAudit(AuditFilter{RequestID: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Action != "request_created" {
		t.Fatalf("audit=%v", events)
	}
}
