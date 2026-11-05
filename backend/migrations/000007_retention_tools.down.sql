DROP TABLE IF EXISTS transfer_plans;
DROP TABLE IF EXISTS recommendation_outcomes;
DROP TABLE IF EXISTS recommendation_snapshots;
DROP TABLE IF EXISTS notification_deliveries;
DROP TABLE IF EXISTS user_notifications;
DROP TABLE IF EXISTS watchlist_observations;
DROP TABLE IF EXISTS player_watchlists;
ALTER TABLE user_preferences
    DROP COLUMN IF EXISTS web_push_alerts,
    DROP COLUMN IF EXISTS email_alerts,
    DROP COLUMN IF EXISTS in_app_alerts,
    DROP COLUMN IF EXISTS deadline_reminder_hours,
    DROP COLUMN IF EXISTS timezone;
