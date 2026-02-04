package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
)

func TestSickRockServer_Ping(t *testing.T) {
	srv := NewSickRockServer(nil, nil)
	ctx := context.Background()

	res, err := srv.Ping(ctx, connect.NewRequest(&sickrockpb.PingRequest{Message: "hello"}))
	require.NoError(t, err)
	assert.Equal(t, "hello", res.Msg.GetMessage())
	assert.Greater(t, res.Msg.GetTimestampUnix(), int64(0))
}

func TestSickRockServer_Ping_EmptyMessage(t *testing.T) {
	srv := NewSickRockServer(nil, nil)
	ctx := context.Background()

	res, err := srv.Ping(ctx, connect.NewRequest(&sickrockpb.PingRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "pong", res.Msg.GetMessage())
}
