package repo

import (
	"context"
	"testing"
)

func insertTestUser(t *testing.T, r *Repository, username string) int {
	t.Helper()
	res, err := r.db.ExecContext(context.Background(), `INSERT INTO user_accounts (username) VALUES (?)`, username)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return int(id)
}

func insertTestGroup(t *testing.T, r *Repository, name string, members ...int) {
	t.Helper()
	ctx := context.Background()
	res, err := r.db.ExecContext(ctx, `INSERT INTO user_groups (name, created_at, updated_at) VALUES (?, datetime('now'), datetime('now'))`, name)
	if err != nil {
		t.Fatal(err)
	}
	groupID, _ := res.LastInsertId()
	for _, member := range members {
		if _, err := r.db.ExecContext(ctx, `INSERT INTO user_group_memberships (created_at, updated_at, user_account_id, user_group_id) VALUES (datetime('now'), datetime('now'), ?, ?)`, member, groupID); err != nil {
			t.Fatal(err)
		}
	}
}

func usernames(users []NotifiableUser) []string {
	out := []string{}
	for _, u := range users {
		out = append(out, u.Username)
	}
	return out
}

func TestListNotifiableUsersOnlySharedGroups(t *testing.T) {
	r := newMigratedSQLiteRepo(t)
	ctx := context.Background()
	alice := insertTestUser(t, r, "notify-alice")
	bob := insertTestUser(t, r, "notify-bob")
	carol := insertTestUser(t, r, "notify-carol")
	dave := insertTestUser(t, r, "notify-dave")
	loner := insertTestUser(t, r, "notify-loner")
	insertTestGroup(t, r, "Kitchen", alice, bob, carol)
	insertTestGroup(t, r, "Garage", alice, carol, dave) // carol shares two groups: listed once
	insertTestGroup(t, r, "Away", loner)

	users, err := r.ListNotifiableUsers(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	if got := usernames(users); len(got) != 3 || got[0] != "notify-bob" || got[1] != "notify-carol" || got[2] != "notify-dave" {
		t.Fatalf("alice can notify %v; want bob, carol, dave", got)
	}

	users, err = r.ListNotifiableUsers(ctx, bob)
	if err != nil {
		t.Fatal(err)
	}
	if got := usernames(users); len(got) != 2 || got[0] != "notify-alice" || got[1] != "notify-carol" {
		t.Fatalf("bob can notify %v; want alice, carol", got)
	}

	users, err = r.ListNotifiableUsers(ctx, loner)
	if err != nil || len(users) != 0 {
		t.Fatalf("loner can notify %v (%v); want nobody", usernames(users), err)
	}
}

func TestGetUserSubscriptionsForEventOnlyThatUsersActiveChannels(t *testing.T) {
	r := newMigratedSQLiteRepo(t)
	ctx := context.Background()
	alice := insertTestUser(t, r, "sub-alice")
	bob := insertTestUser(t, r, "sub-bob")
	event, err := r.GetNotificationEventByCode(ctx, "user.notified")
	if err != nil || event == nil {
		t.Fatalf("user.notified event missing: %v", err)
	}

	subscribe := func(user int, value string, active bool) {
		t.Helper()
		ch, chErr := r.CreateUserNotificationChannel(ctx, user, "webhook", value, nil)
		if chErr != nil {
			t.Fatal(chErr)
		}
		if !active {
			if updErr := r.UpdateUserNotificationChannel(ctx, ch.ID, value, nil, false); updErr != nil {
				t.Fatal(updErr)
			}
		}
		if _, subErr := r.CreateUserNotificationSubscription(ctx, user, event.ID, ch.ID); subErr != nil {
			t.Fatal(subErr)
		}
	}
	subscribe(alice, "https://alice.test/hook", true)
	subscribe(bob, "https://bob.test/active", true)
	subscribe(bob, "https://bob.test/paused", false)

	subs, err := r.GetUserSubscriptionsForEvent(ctx, bob, "user.notified")
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0].Channel.ChannelValue != "https://bob.test/active" {
		t.Fatalf("bob's subscriptions = %+v; want only the active bob channel", subs)
	}

	subs, err = r.GetUserSubscriptionsForEvent(ctx, alice, "health.issue_raised")
	if err != nil || len(subs) != 0 {
		t.Fatalf("other event: %d subscriptions (%v); want none", len(subs), err)
	}
}

// Users created since armature-iam exist only in user_accounts; their channels must work and
// go when the user is deleted.
func TestNotificationChannelsBelongToUserAccounts(t *testing.T) {
	r := newMigratedSQLiteRepo(t)
	ctx := context.Background()
	user := insertTestUser(t, r, "iam-only")
	event, err := r.GetNotificationEventByCode(ctx, "user.notified")
	if err != nil || event == nil {
		t.Fatalf("user.notified event missing: %v", err)
	}
	ch, err := r.CreateUserNotificationChannel(ctx, user, "telegram", "12345", nil)
	if err != nil {
		t.Fatalf("channel for a user_accounts user: %v", err)
	}
	if _, err := r.CreateUserNotificationSubscription(ctx, user, event.ID, ch.ID); err != nil {
		t.Fatalf("subscription for a user_accounts user: %v", err)
	}

	var problems []struct {
		Table  string `db:"table"`
		RowID  int    `db:"rowid"`
		Parent string `db:"parent"`
		FKID   int    `db:"fkid"`
	}
	if err := r.db.SelectContext(ctx, &problems, `PRAGMA foreign_key_check`); err != nil || len(problems) > 0 {
		t.Fatalf("foreign_key_check: %+v, %v", problems, err)
	}

	if _, err := r.db.ExecContext(ctx, `DELETE FROM user_accounts WHERE id = ?`, user); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := r.db.GetContext(ctx, &left, `SELECT (SELECT COUNT(*) FROM user_notification_channels WHERE user = ?) + (SELECT COUNT(*) FROM user_notification_subscriptions WHERE user = ?)`, user, user); err != nil || left != 0 {
		t.Fatalf("%d channels/subscriptions left after deleting the user (%v)", left, err)
	}
}
