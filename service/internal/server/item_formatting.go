package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

// itemFieldsToStrings converts an item's dynamic fields to the string map used by protobuf.
// Times use MySQL datetime format (YYYY-MM-DD HH:MM:SS) without timezone. Nil values are
// skipped, or included as empty strings when includeNulls is set.
func itemFieldsToStrings(fields map[string]any, includeNulls bool) map[string]string {
	out := make(map[string]string, len(fields))
	for key, value := range fields {
		switch v := value.(type) {
		case nil:
			if includeNulls {
				out[key] = ""
			}
		case time.Time:
			out[key] = v.Format("2006-01-02 15:04:05")
		default:
			out[key] = fmt.Sprintf("%v", v)
		}
	}
	return out
}

// conditionalFormattingRules loads the caller's conditional formatting rules for a table,
// logging and returning nil on failure.
func (s *SickRockServer) conditionalFormattingRules(ctx context.Context, table, logPrefix string) []*repo.ConditionalFormattingRule {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		log.WithError(err).Error(logPrefix + "Failed to get user ID from context")
		return nil
	}
	rules, err := s.repo.GetConditionalFormattingRules(ctx, userID, table)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"table":  table,
			"userID": userID,
		}).Error(logPrefix + "Failed to get conditional formatting rules")
		return nil
	}
	log.WithFields(log.Fields{
		"table":     table,
		"userID":    userID,
		"ruleCount": len(rules),
	}).Info(logPrefix + "Retrieved conditional formatting rules")
	return rules
}

// applyMarkdownRules renders active markdown rules whose condition matches into
// additionalFields as "<column>Markdown".
func applyMarkdownRules(rules []*repo.ConditionalFormattingRule, fields map[string]any, additionalFields map[string]string) {
	for _, rule := range rules {
		if rule.FormatType != "markdown" || !rule.IsActive {
			continue
		}
		fieldValue := ""
		if val := fields[rule.ColumnName]; val != nil {
			fieldValue = fmt.Sprintf("%v", val)
		}
		shouldApply := markdownRuleApplies(rule, fieldValue)
		log.WithFields(log.Fields{
			"ruleID":         rule.ID,
			"columnName":     rule.ColumnName,
			"conditionType":  rule.ConditionType,
			"conditionValue": rule.ConditionValue,
			"shouldApply":    shouldApply,
		}).Debug("Evaluated markdown rule condition")
		if shouldApply {
			additionalFields[rule.ColumnName+"Markdown"] = renderMarkdown(joinNonEmpty(fieldValue, rule.FormatValue))
		}
	}
}

// joinNonEmpty appends suffix to value as a new paragraph when suffix is set.
func joinNonEmpty(value, suffix string) string {
	if suffix == "" {
		return value
	}
	return value + "\n\n" + suffix
}

func markdownRuleApplies(rule *repo.ConditionalFormattingRule, fieldValue string) bool {
	switch rule.ConditionType {
	case "always":
		return true
	case "equals":
		return fieldValue == rule.ConditionValue
	case "contains":
		return strings.Contains(strings.ToLower(fieldValue), strings.ToLower(rule.ConditionValue))
	case "greater_than":
		return compareNumeric(fieldValue, rule.ConditionValue, func(a, b float64) bool { return a > b })
	case "less_than":
		return compareNumeric(fieldValue, rule.ConditionValue, func(a, b float64) bool { return a < b })
	}
	return false
}

// compareNumeric parses both values as floats and applies cmp; unparsable values never match.
func compareNumeric(fieldValue, conditionValue string, cmp func(a, b float64) bool) bool {
	fieldNum, err := strconv.ParseFloat(fieldValue, 64)
	if err != nil {
		return false
	}
	conditionNum, err := strconv.ParseFloat(conditionValue, 64)
	if err != nil {
		return false
	}
	return cmp(fieldNum, conditionNum)
}

// secondsSince returns the whole seconds elapsed since t, or 0 when t is unset.
func secondsSince(t time.Time) int32 {
	if t.IsZero() {
		return 0
	}
	return safeInt64ToInt32(int64(time.Since(t).Seconds()))
}

func itemRepoToProto(it repo.Item, additionalFields map[string]string) *sickrockpb.Item {
	return &sickrockpb.Item{
		Id:                it.ID,
		SrCreated:         it.SrCreated.Unix(),
		SrCreatedRelative: secondsSince(it.SrCreated),
		SrUpdated:         it.SrUpdated.Unix(),
		SrUpdatedRelative: secondsSince(it.SrUpdated),
		AdditionalFields:  additionalFields,
	}
}
