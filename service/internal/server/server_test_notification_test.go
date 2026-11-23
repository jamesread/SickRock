package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/notifications"
)

func sendTest(ctx context.Context, srv *SickRockServer, channelID int) (*sickrockpb.SendTestNotificationResponse, error) {
	res, err := srv.SendTestNotification(ctx, connect.NewRequest(&sickrockpb.SendTestNotificationRequest{ChannelId: int32(channelID)}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func TestSendTestNotificationReachesTheChannel(t *testing.T) {
	f := newNotifyFixture(t)
	ch, err := f.repo.CreateUserNotificationChannel(context.Background(), f.alice, "webhook", f.hookURL, nil)
	require.NoError(t, err)

	res, err := sendTest(as(f.alice, "alice"), f.srv, ch.ID)
	require.NoError(t, err)
	assert.True(t, res.Success)
	f.hookMu.Lock()
	defer f.hookMu.Unlock()
	assert.Equal(t, []string{notifications.TestNotificationMessage}, f.hookMessages)
}

func TestSendTestNotificationOnlyForOwnChannels(t *testing.T) {
	f := newNotifyFixture(t)
	ch, err := f.repo.CreateUserNotificationChannel(context.Background(), f.alice, "webhook", f.hookURL, nil)
	require.NoError(t, err)

	_, err = sendTest(as(f.bob, "bob"), f.srv, ch.ID)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = sendTest(as(f.alice, "alice"), f.srv, 999999)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	_, err = sendTest(as(f.alice, "alice"), f.srv, 0)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = sendTest(context.Background(), f.srv, ch.ID)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	assert.Empty(t, f.hookMessages)
}

func TestSendTestNotificationReportsDeliveryFailure(t *testing.T) {
	f := newNotifyFixture(t)
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)
	ch, err := f.repo.CreateUserNotificationChannel(context.Background(), f.alice, "webhook", failing.URL, nil)
	require.NoError(t, err)

	res, err := sendTest(as(f.alice, "alice"), f.srv, ch.ID)
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Equal(t, "webhook returned status 500", res.Message)
}

func TestSendTestNotificationRateLimit(t *testing.T) {
	f := newNotifyFixture(t)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	f.srv.testNotifyLimiter.now = func() time.Time { return now }
	ch, err := f.repo.CreateUserNotificationChannel(context.Background(), f.alice, "webhook", f.hookURL, nil)
	require.NoError(t, err)
	ctx := as(f.alice, "alice")

	for i := range TestNotificationRateLimit {
		_, err := sendTest(ctx, f.srv, ch.ID)
		require.NoError(t, err, "test %d", i+1)
	}
	_, err = sendTest(ctx, f.srv, ch.ID)
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))

	// It is separate from the user-to-user notification limit.
	assert.True(t, f.srv.notifyLimiter.allow(f.alice, 1))

	now = now.Add(TestNotificationRateWindow + time.Second)
	_, err = sendTest(ctx, f.srv, ch.ID)
	require.NoError(t, err, "the window has passed")
}
