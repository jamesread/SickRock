package server

import (
	"encoding/json"
	"os"
	"strings"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
)

type oauthProviderEnv struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	AuthURL string `json:"auth_url"`
}

func loadOAuthProvidersFromEnv() []*sickrockpb.OAuthProvider {
	raw := strings.TrimSpace(os.Getenv("SICKROCK_OAUTH_PROVIDERS"))
	if raw == "" {
		return nil
	}
	var parsed []oauthProviderEnv
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	out := make([]*sickrockpb.OAuthProvider, 0, len(parsed))
	for _, p := range parsed {
		id := strings.TrimSpace(p.ID)
		label := strings.TrimSpace(p.Label)
		authURL := strings.TrimSpace(p.AuthURL)
		if id == "" || authURL == "" {
			continue
		}
		if label == "" {
			label = id
		}
		out = append(out, &sickrockpb.OAuthProvider{
			Id:      id,
			Label:   label,
			AuthUrl: authURL,
		})
	}
	return out
}
