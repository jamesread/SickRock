-- armature-health v2 (its migrate/sqlite/health_v2.sql): app-defined issue attributes, and the
-- acknowledging user stored as a string. Existing integer ids become their decimal strings.
ALTER TABLE health_issues ADD COLUMN attributes TEXT NOT NULL DEFAULT '{}';
ALTER TABLE health_issues ADD COLUMN acknowledged_by_v2 TEXT;
UPDATE health_issues SET acknowledged_by_v2 = CAST(acknowledged_by AS TEXT) WHERE acknowledged_by IS NOT NULL;
ALTER TABLE health_issues DROP COLUMN acknowledged_by;
ALTER TABLE health_issues RENAME COLUMN acknowledged_by_v2 TO acknowledged_by;
