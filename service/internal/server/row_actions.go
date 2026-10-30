package server

import (
	"encoding/json"
	"strings"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	log "github.com/sirupsen/logrus"
)

// rowActionJSON is one entry of the table_configurations.row_actions JSON array.
type rowActionJSON struct {
	Query map[string]string `json:"query"`
	Label string            `json:"label"`
	Path  string            `json:"path"`
	Icon  string            `json:"icon"`
}

// rowActionsToProto parses a table configuration's row_actions JSON. Invalid JSON is logged and
// yields no actions; entries without a label or path are skipped.
func rowActionsToProto(tcName, raw string) []*sickrockpb.RowAction {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var parsed []rowActionJSON
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		log.WithError(err).WithField("table", tcName).Warn("Invalid row_actions JSON on table configuration")
		return nil
	}
	out := make([]*sickrockpb.RowAction, 0, len(parsed))
	for _, a := range parsed {
		label, path := strings.TrimSpace(a.Label), strings.TrimSpace(a.Path)
		if label == "" || path == "" {
			continue
		}
		out = append(out, &sickrockpb.RowAction{
			Label: label,
			Path:  path,
			Query: a.Query,
			Icon:  strings.TrimSpace(a.Icon),
		})
	}
	return out
}
