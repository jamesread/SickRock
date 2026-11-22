package repo

import "context"

// NotifiableUser is a user another user may send a notification to.
type NotifiableUser struct {
	Username string `db:"username"`
	ID       int    `db:"id"`
}

// ListNotifiableUsers returns the users who share at least one IAM group with senderID, not
// including the sender, ordered by username.
func (r *Repository) ListNotifiableUsers(ctx context.Context, senderID int) ([]NotifiableUser, error) {
	query := `
		SELECT DISTINCT u.id, u.username
		FROM user_accounts u
		JOIN user_group_memberships theirs ON theirs.user_account_id = u.id
		JOIN user_group_memberships mine ON mine.user_group_id = theirs.user_group_id
		WHERE mine.user_account_id = ? AND u.id <> ?
		ORDER BY u.username
	`
	users := []NotifiableUser{}
	if err := r.db.SelectContext(ctx, &users, query, senderID, senderID); err != nil {
		return nil, err
	}
	return users, nil
}

// GetUserSubscriptionsForEvent returns one user's subscriptions to eventCode on active channels.
func (r *Repository) GetUserSubscriptionsForEvent(ctx context.Context, userID int, eventCode string) ([]eventSubscription, error) {
	query := `
		SELECT
			uns.id, uns.user, uns.event_id, uns.channel_id, uns.sr_created,
			unc.id, unc.user, unc.channel_type, unc.channel_value, unc.channel_name, unc.is_active, unc.sr_created, unc.sr_updated,
			ne.id, ne.event_code, ne.event_name, ne.description, ne.sr_created
		FROM user_notification_subscriptions uns
		JOIN user_notification_channels unc ON uns.channel_id = unc.id
		JOIN notification_events ne ON uns.event_id = ne.id
		WHERE ne.event_code = ? AND unc.is_active = 1 AND uns.user = ? AND unc.user = ?
	`
	rows, err := r.db.QueryxContext(ctx, query, eventCode, userID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanSubscriptionsForEvent(rows)
}
