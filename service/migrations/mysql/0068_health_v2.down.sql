-- Acknowledgements by non-numeric user ids cannot be kept as integers; they are cleared.
UPDATE health_issues SET acknowledged_at = NULL, acknowledged_by = NULL
    WHERE acknowledged_by IS NOT NULL AND acknowledged_by NOT REGEXP '^[0-9]+$';
ALTER TABLE health_issues MODIFY acknowledged_by BIGINT NULL;
ALTER TABLE health_issues DROP COLUMN IF EXISTS attributes;
