package store

import "time"

func (s *Store) migrateNotify() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS notifications (
  request_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  sent_at TEXT NOT NULL,
  PRIMARY KEY (request_id, kind)
);
`)
	return err
}

// TryMarkNotified records a send intent. Returns true if this is the first time
// for (requestID, kind) — callers should only send when true.
func (s *Store) TryMarkNotified(requestID, kind string) (bool, error) {
	res, err := s.db.Exec(
		`INSERT OR IGNORE INTO notifications (request_id, kind, sent_at) VALUES (?, ?, ?)`,
		requestID, kind, time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}
