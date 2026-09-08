package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusExpired  Status = "expired"
	StatusReleased Status = "released"
	StatusDenied   Status = "denied"
)

type Request struct {
	ID               string
	Username         string
	Justification    string
	Status           Status
	CreatedAt        time.Time
	RequestExpiresAt time.Time
	ApprovedAt       *time.Time
	ApprovedBy       string
	ApprovalNotes    string // approver written feedback / guidance
	ActiveExpiresAt  *time.Time
	ReleasedAt       *time.Time
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS requests (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL,
  justification TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  request_expires_at TEXT NOT NULL,
  approved_at TEXT,
  approved_by TEXT,
  active_expires_at TEXT,
  released_at TEXT,
  approval_notes TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status);
`)
	if err != nil {
		return err
	}
	if err := s.migrateAudit(); err != nil {
		return err
	}
	if err := s.migrateNotify(); err != nil {
		return err
	}
	return s.migrateApprovalNotes()
}

func (s *Store) migrateApprovalNotes() error {
	_, err := s.db.Exec(`ALTER TABLE requests ADD COLUMN approval_notes TEXT NOT NULL DEFAULT ''`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		// SQLite: "duplicate column name: approval_notes"
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return err
		}
	}
	return nil
}

func (s *Store) Create(r *Request) error {
	_, err := s.db.Exec(`INSERT INTO requests
(id, username, justification, status, created_at, request_expires_at, approved_at, approved_by, active_expires_at, released_at, approval_notes)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Username, r.Justification, r.Status,
		r.CreatedAt.UTC().Format(time.RFC3339Nano),
		r.RequestExpiresAt.UTC().Format(time.RFC3339Nano),
		nullTime(r.ApprovedAt), nullStr(r.ApprovedBy), nullTime(r.ActiveExpiresAt), nullTime(r.ReleasedAt),
		r.ApprovalNotes,
	)
	return err
}

func (s *Store) Get(id string) (*Request, error) {
	row := s.db.QueryRow(`SELECT id, username, justification, status, created_at, request_expires_at,
approved_at, approved_by, active_expires_at, released_at, COALESCE(approval_notes,'') FROM requests WHERE id = ?`, id)
	return scanRequest(row)
}

func (s *Store) List() ([]Request, error) {
	rows, err := s.db.Query(`SELECT id, username, justification, status, created_at, request_expires_at,
approved_at, approved_by, active_expires_at, released_at, COALESCE(approval_notes,'') FROM requests ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Request
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (s *Store) Update(r *Request) error {
	res, err := s.db.Exec(`UPDATE requests SET
status=?, approved_at=?, approved_by=?, active_expires_at=?, released_at=?, approval_notes=?
WHERE id=?`,
		r.Status, nullTime(r.ApprovedAt), nullStr(r.ApprovedBy), nullTime(r.ActiveExpiresAt), nullTime(r.ReleasedAt),
		r.ApprovalNotes, r.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("request %s not found", r.ID)
	}
	return nil
}

func (s *Store) PendingExpired(now time.Time) ([]Request, error) {
	return s.byStatusBefore(StatusPending, "request_expires_at", now)
}

func (s *Store) ActiveExpired(now time.Time) ([]Request, error) {
	return s.byStatusBefore(StatusApproved, "active_expires_at", now)
}

func (s *Store) byStatusBefore(status Status, col string, now time.Time) ([]Request, error) {
	q := fmt.Sprintf(`SELECT id, username, justification, status, created_at, request_expires_at,
approved_at, approved_by, active_expires_at, released_at, COALESCE(approval_notes,'') FROM requests
WHERE status = ? AND %s IS NOT NULL AND %s <= ?`, col, col)
	rows, err := s.db.Query(q, status, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Request
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRequest(row rowScanner) (*Request, error) {
	var r Request
	var created, reqExp string
	var approvedAt, activeExp, releasedAt, approvedBy sql.NullString
	var notes string
	if err := row.Scan(&r.ID, &r.Username, &r.Justification, &r.Status, &created, &reqExp,
		&approvedAt, &approvedBy, &activeExp, &releasedAt, &notes); err != nil {
		return nil, err
	}
	var err error
	r.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return nil, err
	}
	r.RequestExpiresAt, err = time.Parse(time.RFC3339Nano, reqExp)
	if err != nil {
		return nil, err
	}
	r.ApprovedBy = approvedBy.String
	r.ApprovalNotes = notes
	r.ApprovedAt = parseNullTime(approvedAt)
	r.ActiveExpiresAt = parseNullTime(activeExp)
	r.ReleasedAt = parseNullTime(releasedAt)
	return &r, nil
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func parseNullTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, ns.String)
	if err != nil {
		return nil
	}
	return &t
}
