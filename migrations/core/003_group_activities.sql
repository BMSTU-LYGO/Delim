ALTER TABLE groups
    ADD COLUMN activity_type TEXT NOT NULL DEFAULT '' CHECK (activity_type IN ('', 'trip', 'hike', 'event')),
    ADD COLUMN location TEXT NOT NULL DEFAULT '',
    ADD COLUMN start_date DATE,
    ADD COLUMN end_date DATE,
    ADD COLUMN planned_budget_minor BIGINT CHECK (planned_budget_minor >= 0),
    ADD CONSTRAINT groups_activity_dates_check CHECK ((start_date IS NULL) = (end_date IS NULL) AND (start_date IS NULL OR end_date >= start_date));
