package store

import (
	"context"
	"time"
)

func (s *Store) SortServers(ctx context.Context, serverIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	for index, serverID := range serverIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE servers SET sort = ?, updated_at = ? WHERE id = ?`, index+1, now, serverID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteUser(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM subscription_links WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM quick_logins WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}
