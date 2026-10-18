DELETE FROM notification_events WHERE event_code = 'health.issue_raised';
DROP TABLE IF EXISTS health_check_runs;
DROP TABLE IF EXISTS health_issues;
