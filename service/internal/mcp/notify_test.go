package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
)

func TestNotifyRecipientIDs(t *testing.T) {
	users := []*sickrockpb.NotifiableUser{{Id: 2, Username: "Bob"}, {Id: 3, Username: "carol"}}
	ids, unknown := notifyRecipientIDs(" bob , CAROL,,", users)
	assert.Equal(t, []int32{2, 3}, ids)
	assert.Empty(t, unknown)

	ids, unknown = notifyRecipientIDs("bob, mallory", users)
	assert.Equal(t, []int32{2}, ids)
	assert.Equal(t, []string{"mallory"}, unknown)
}
