#!/usr/bin/env bash
# Seeds the throwaway SickRock backend with fictional marketing data via its
# public Connect JSON API. Usage: seed.sh <backend-base-url>
#
# Row values like "@day+3 09:30" become a datetime relative to today, so the
# calendar and "upcoming" views always look current.
set -euo pipefail

base="${1:?backend base url}"
here="$(cd "$(dirname "$0")" && pwd)"
api="$base/api/sickrock.SickRock"
db="sickrock"

token="$(curl -fsS -X POST -H 'Content-Type: application/json' \
	-d '{"username":"admin","password":"admin"}' "$api/Login" | jq -er .token)"

# rpc <method> <json>: prints the response, fails on HTTP errors and success=false.
rpc() {
	local out
	out="$(curl -fsS -X POST -H 'Content-Type: application/json' \
		-H "Authorization: Bearer $token" -d "$2" "$api/$1")" || {
		echo "seed: $1 failed: $2" >&2
		return 1
	}
	if [ "$(jq -r '.success // true' <<<"$out")" = "false" ]; then
		echo "seed: $1 failed: $(jq -r .message <<<"$out")" >&2
		return 1
	fi
	printf '%s\n' "$out"
}

schema="$here/seed/schema.json"

declare -A app_ids
while read -r app; do
	title="$(jq -r .title <<<"$app")"
	id="$(rpc CreateItem "$(jq -c '{pageId: "table_apps", additionalFields: {title, ordinal: (.ordinal | tostring)}}' <<<"$app")" | jq -r .item.id)"
	app_ids["$title"]="$id"
done < <(jq -c '.apps[]' "$schema")

while read -r table; do
	name="$(jq -r .name <<<"$table")"
	app="$(jq -r '.app // ""' <<<"$table")"
	app_id=0
	[ -n "$app" ] && app_id="${app_ids[$app]}"

	rpc CreateTable "$(jq -nc --arg db "$db" --arg t "$name" '{database: $db, table: $t}')" >/dev/null
	rpc CreateTableConfiguration "$(jq -nc --arg db "$db" --arg t "$name" --argjson app "$app_id" \
		'{name: $t, database: $db, table: $t, navigationAppId: $app}')" >/dev/null

	jq -c '.columns[] | {name, type, required: (.required // false)}' <<<"$table" | while read -r field; do
		rpc AddTableColumn "$(jq -nc --arg t "$name" --argjson f "$field" '{pageId: $t, field: $f}')" >/dev/null
	done

	rpc SetColumnDisplaySettings "$(jq -c '{tableConfiguration: .name, settings: [.columns[] |
		{columnName: .name, displayLabel: .label, valuePrefix: (.prefix // ""), valueSuffix: (.suffix // "")}]}' <<<"$table")" >/dev/null
	rpc SetTableRowLabels "$(jq -c '{tableConfiguration: .name, rowName, rowNamePlural}' <<<"$table")" >/dev/null
	rpc SetItemFieldGroups "$(jq -c '{tableConfiguration: .name, groups: [.fieldGroups | to_entries[] |
		{ordinal: .key, title: .value.title, columnNames: .value.columns}]}' <<<"$table")" >/dev/null

	jq -c '.views[]' <<<"$table" | while read -r view; do
		id="$(rpc CreateTableView "$(jq -c --arg t "$name" '{tableName: $t, viewName: .name, viewType: .type,
			columns: [(.sort // {}) as $sort | .columns | to_entries[] |
				{columnName: .value, isVisible: true, columnOrder: .key, sortOrder: ($sort[.value] // "")}]}' <<<"$view")" | jq -r .viewId)"
		if [ "$(jq -r '.default // false' <<<"$view")" = "true" ]; then
			rpc SetUserDefaultTableView "$(jq -nc --arg t "$name" --argjson id "$id" '{tableName: $t, viewId: $id}')" >/dev/null
		fi
	done
done < <(jq -c '.tables[]' "$schema")

# Navigation icons live on table_configurations.icon.
tcs="$(rpc ListItems '{"tcName":"table_configurations"}')"
while read -r table; do
	name="$(jq -r .name <<<"$table")"
	icon="$(jq -r .icon <<<"$table")"
	id="$(jq -r --arg t "$name" '.items[] | select(.additionalFields.name == $t) | .id' <<<"$tcs")"
	rpc EditItem "$(jq -nc --arg id "$id" --arg icon "$icon" \
		'{pageId: "table_configurations", id: $id, additionalFields: {icon: $icon}}')" >/dev/null
done < <(jq -c '.tables[]' "$schema")

while read -r fk; do
	rpc CreateForeignKey "$(jq -c '{tableName: .table, columnName: .column, referencedTable: .references,
		referencedColumn: .referencedColumn, onDeleteAction: "SET NULL", onUpdateAction: "CASCADE"}' <<<"$fk")" >/dev/null
	constraint="$(rpc GetForeignKeys "$(jq -c '{tableName: .table}' <<<"$fk")" |
		jq -r --argjson fk "$fk" '.foreignKeys[] | select(.columnName == $fk.column) | .constraintName')"
	rpc UpdateTableRelation "$(jq -c --arg c "$constraint" '{constraintName: $c, displayName}' <<<"$fk")" >/dev/null
done < <(jq -c '.foreignKeys[]' "$schema")

while read -r table; do
	name="$(jq -r .name <<<"$table")"
	jq -c '.[] | map_values(
		if type == "string" and test("^@day[+-][0-9]+ ") then
			capture("^@day(?<d>[+-][0-9]+) (?<t>.*)$") as $m
			| ((now | floor) + ($m.d | tonumber) * 86400 | strflocaltime("%Y-%m-%d")) + " " + $m.t + ":00"
		else . end)' "$here/seed/$name.json" | while read -r row; do
		rpc CreateItem "$(jq -nc --arg t "$name" --argjson f "$row" '{pageId: $t, additionalFields: $f}')" >/dev/null
	done
done < <(jq -c '.tables[]' "$schema")

# Replace the default admin password (which the health checks flag as an
# error), then re-run the checks so the sidebar shows a healthy system.
password="$(jq -r .adminPassword "$schema")"
rpc ChangePassword "$(jq -nc --arg p "$password" '{currentPassword: "admin", newPassword: $p}')" >/dev/null
token="$(curl -fsS -X POST -H 'Content-Type: application/json' \
	-d "$(jq -nc --arg p "$password" '{username: "admin", password: $p}')" "$api/Login" | jq -er .token)"
rpc RunHealthChecks '{}' >/dev/null

echo "Seeded $(jq '.tables | length' "$schema") tables."
