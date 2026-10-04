package rsscalendar

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FieldMapping maps one RSS field to a table column.
type FieldMapping struct {
	RssField string `json:"rss_field"`
	Column   string `json:"column"`
}

// FieldMappingConfig is persisted in rss_calendar_feeds.field_mapping_json.
type FieldMappingConfig struct {
	Mappings []FieldMapping `json:"mappings"`
}

func ParseFieldMappingConfig(raw string) (FieldMappingConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return FieldMappingConfig{Mappings: []FieldMapping{}}, nil
	}
	var cfg FieldMappingConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return FieldMappingConfig{}, fmt.Errorf("invalid field_mapping_json: %w", err)
	}
	return cfg, nil
}

func EncodeFieldMappingConfig(cfg FieldMappingConfig) (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ApplyMappings builds table row fields from RSS item values.
func ApplyMappings(itemValues map[string]string, mappings []FieldMapping) map[string]string {
	out := map[string]string{}
	for _, m := range mappings {
		if v, ok := itemValues[m.RssField]; ok {
			out[m.Column] = v
		}
	}
	return out
}
