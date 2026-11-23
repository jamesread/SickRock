-- armature-health v2 (its migrate/mysql/health_v2.sql): app-defined issue attributes, and the
-- acknowledging user stored as a string. Existing integer ids become their decimal strings.
ALTER TABLE health_issues ADD COLUMN IF NOT EXISTS attributes TEXT NULL;
UPDATE health_issues SET attributes = '{}' WHERE attributes IS NULL;
ALTER TABLE health_issues MODIFY attributes TEXT NOT NULL;
ALTER TABLE health_issues MODIFY acknowledged_by VARCHAR(191) NULL;
