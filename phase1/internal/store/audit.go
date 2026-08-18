package store

import (
	"time"
)

type AuditEvent struct {
	ID            int64
	At            time.Time
	Action        string
	Actor         string
	Subject       string
	RequestID     string
	Justification string
	Detail        string
	BreakGlass    bool
}

func (s *Store) migrateAudit() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  at TEXT NOT NULL,
  action TEXT NOT NULL,
  actor TEXT NOT NULL DEFAULT '',
  subject TEXT NOT NULL DEFAULT '',
  request_id TEXT NOT NULL DEFAULT '',
  justification TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '',
  break_glass INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_audit_at ON audit_events(at);
CREATE INDEX IF NOT EXISTS idx_audit_request ON audit_events(request_id);
`)
	return err
}

func (s *Store) AppendAudit(e *AuditEvent) error {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	bg := 0
	if e.BreakGlass {
		bg = 1
	}
	res, err := s.db.Exec(`INSERT INTO audit_events
(at, action, actor, subject, request_id, justification, detail, break_glass)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.At.UTC().Format(time.RFC3339Nano),
		e.Action, e.Actor, e.Subject, e.RequestID, e.Justification, e.Detail, bg,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	e.ID = id
	return nil
}

type AuditFilter struct {
	RequestID  string
	Actor      string
	BreakGlass *bool
	Limit      int
}

func (s *Store) ListAudit(f AuditFilter) ([]AuditEvent, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	q := `SELECT id, at, action, actor, subject, request_id, justification, detail, break_glass
FROM audit_events WHERE 1=1`
	args := []any{}
	if f.RequestID != "" {
		q += ` AND request_id = ?`
		args = append(args, f.RequestID)
	}
	if f.Actor != "" {
		q += ` AND actor = ?`
		args = append(args, f.Actor)
	}
	if f.BreakGlass != nil {
		v := 0
		if *f.BreakGlass {
			v = 1
		}
		q += ` AND break_glass = ?`
		args = append(args, v)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, f.Limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var at string
		var bg int
		if err := rows.Scan(&e.ID, &at, &e.Action, &e.Actor, &e.Subject, &e.RequestID,
			&e.Justification, &e.Detail, &bg); err != nil {
			return nil, err
		}
		e.At, err = time.Parse(time.RFC3339Nano, at)
		if err != nil {
			return nil, err
		}
		e.BreakGlass = bg != 0
		out = append(out, e)
	}
	if out == nil {
		out = []AuditEvent{}
	}
	return out, rows.Err()
}
