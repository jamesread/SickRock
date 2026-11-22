package server

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	repo "github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

// bulkEntryJSON is table_configurations.bulk_entry_json.
type bulkEntryJSON struct {
	ScanColumn     string   `json:"scan_column"`
	QuantityColumn string   `json:"quantity_column"`
	MatchColumns   []string `json:"match_columns"`
	Fields         []string `json:"fields"`
	SaveOnScan     bool     `json:"save_on_scan"`
}

// bulkEntryToProto parses a table configuration's bulk_entry_json against its column names. Invalid
// JSON or an unknown scan column turns bulk entry off (logged); other unknown columns are dropped.
func bulkEntryToProto(tcName, raw string, columnNames []string) *sickrockpb.BulkEntryConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	logger := log.WithField("table", tcName)
	var parsed bulkEntryJSON
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		logger.WithError(err).Warn("Invalid bulk_entry_json on table configuration")
		return nil
	}
	known := func(name string) bool { return slices.Contains(columnNames, name) }
	if !known(parsed.ScanColumn) {
		logger.Warnf("bulk_entry_json scan_column %q is not a column", parsed.ScanColumn)
		return nil
	}
	cfg := &sickrockpb.BulkEntryConfig{ScanColumn: parsed.ScanColumn, SaveOnScan: parsed.SaveOnScan}
	if known(parsed.QuantityColumn) {
		cfg.QuantityColumn = parsed.QuantityColumn
	}
	cfg.MatchColumns = knownColumns(parsed.MatchColumns, columnNames, []string{cfg.ScanColumn})
	taken := append([]string{cfg.ScanColumn, cfg.QuantityColumn}, cfg.MatchColumns...)
	cfg.Fields = knownColumns(parsed.Fields, columnNames, taken)
	return cfg
}

// knownColumns returns names that are in columnNames and not in exclude, without duplicates,
// in their original order.
func knownColumns(names, columnNames, exclude []string) []string {
	var out []string
	for _, c := range names {
		if slices.Contains(columnNames, c) && !slices.Contains(exclude, c) && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}

// AddItemQuantity adds to (or takes from) a matching row's quantity, creating the row when none matches.
func (s *SickRockServer) AddItemQuantity(ctx context.Context, req *connect.Request[sickrockpb.AddItemQuantityRequest]) (*connect.Response[sickrockpb.AddItemQuantityResponse], error) {
	table := cmp.Or(req.Msg.GetPageId(), "items")
	if err := s.requireItemQuantityAccess(ctx, table); err != nil {
		return nil, err
	}
	if req.Msg.GetQuantity() == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("quantity must not be zero"))
	}

	res, err := s.repo.AddItemQuantity(ctx, table, req.Msg.GetMatch(), req.Msg.GetQuantityColumn(), req.Msg.GetQuantity(), req.Msg.GetFields())
	if err != nil {
		return nil, addItemQuantityError(err)
	}
	return connect.NewResponse(&sickrockpb.AddItemQuantityResponse{
		Item:     itemRepoToProto(res.Item, itemFieldsToStrings(res.Item.Fields, true)),
		Created:  res.Created,
		Quantity: res.Quantity,
	}), nil
}

func (s *SickRockServer) requireItemQuantityAccess(ctx context.Context, table string) error {
	for _, action := range []repo.ResourceGrantAction{repo.ResourceActionInsert, repo.ResourceActionEdit} {
		if err := s.requireItemWriteAccess(ctx, table, action); err != nil {
			return err
		}
	}
	return nil
}

func addItemQuantityError(err error) error {
	switch {
	case errors.Is(err, repo.ErrNoMatchingItem):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, repo.ErrNegativeQuantity):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return err
	}
}
