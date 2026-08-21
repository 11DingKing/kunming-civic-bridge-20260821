CREATE TABLE IF NOT EXISTS campaigns (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    theme TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('draft','open','review','closed')),
    opens_at TEXT NOT NULL,
    closes_at TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_campaign_status_window ON campaigns(status, opens_at, closes_at);

CREATE TABLE IF NOT EXISTS collection_points (
    id TEXT PRIMARY KEY,
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
    district TEXT NOT NULL,
    name TEXT NOT NULL,
    address TEXT NOT NULL,
    topic TEXT NOT NULL,
    daily_quota INTEGER NOT NULL CHECK(daily_quota > 0),
    status TEXT NOT NULL CHECK(status IN ('preparing','active','paused','retired')),
    version INTEGER NOT NULL DEFAULT 1,
    activated_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(campaign_id, district, name)
);

CREATE INDEX IF NOT EXISTS idx_point_campaign_status ON collection_points(campaign_id, status);
CREATE INDEX IF NOT EXISTS idx_point_district ON collection_points(district, status);

CREATE TABLE IF NOT EXISTS invited_advisors (
    id TEXT PRIMARY KEY,
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
    user_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    expertise TEXT NOT NULL DEFAULT '[]',
    districts TEXT NOT NULL DEFAULT '[]',
    review_quota INTEGER NOT NULL CHECK(review_quota > 0),
    active INTEGER NOT NULL DEFAULT 1,
    version INTEGER NOT NULL DEFAULT 1,
    enrolled_at TEXT NOT NULL,
    deactivated_at TEXT,
    UNIQUE(campaign_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_advisor_campaign_active ON invited_advisors(campaign_id, active);

CREATE TABLE IF NOT EXISTS suggestion_intakes (
    id TEXT PRIMARY KEY,
    suggestion_id TEXT NOT NULL UNIQUE REFERENCES items(id) ON DELETE RESTRICT,
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
    collection_point_id TEXT NOT NULL REFERENCES collection_points(id) ON DELETE RESTRICT,
    idempotency_key TEXT NOT NULL,
    source TEXT NOT NULL CHECK(source IN ('online','offline','advisor')),
    accepted_at TEXT NOT NULL,
    UNIQUE(campaign_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_intake_point_day ON suggestion_intakes(collection_point_id, accepted_at);
CREATE INDEX IF NOT EXISTS idx_intake_campaign_time ON suggestion_intakes(campaign_id, accepted_at);

CREATE TRIGGER IF NOT EXISTS trg_intake_daily_quota
BEFORE INSERT ON suggestion_intakes
BEGIN
    SELECT CASE WHEN (
        SELECT COUNT(*) FROM suggestion_intakes
        WHERE collection_point_id = NEW.collection_point_id
          AND date(accepted_at) = date(NEW.accepted_at)
    ) >= (
        SELECT daily_quota FROM collection_points WHERE id = NEW.collection_point_id
    ) THEN RAISE(ABORT, 'collection point daily quota reached') END;
END;

CREATE TABLE IF NOT EXISTS professional_reviews (
    id TEXT PRIMARY KEY,
    suggestion_id TEXT NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    advisor_id TEXT NOT NULL REFERENCES invited_advisors(id) ON DELETE RESTRICT,
    panel_key TEXT NOT NULL,
    verdict TEXT NOT NULL CHECK(verdict IN ('pending','accepted','revise','rejected')),
    notes TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    assigned_at TEXT NOT NULL,
    reviewed_at TEXT,
    UNIQUE(suggestion_id, advisor_id),
    UNIQUE(panel_key, advisor_id)
);

CREATE INDEX IF NOT EXISTS idx_review_suggestion_verdict ON professional_reviews(suggestion_id, verdict);
CREATE INDEX IF NOT EXISTS idx_review_advisor_pending ON professional_reviews(advisor_id, verdict);

CREATE TRIGGER IF NOT EXISTS trg_advisor_review_quota
BEFORE INSERT ON professional_reviews
BEGIN
    SELECT CASE WHEN (
        SELECT COUNT(*) FROM professional_reviews
        WHERE advisor_id = NEW.advisor_id AND verdict = 'pending'
    ) >= (
        SELECT review_quota FROM invited_advisors WHERE id = NEW.advisor_id AND active = 1
    ) THEN RAISE(ABORT, 'advisor pending review quota reached') END;
END;

CREATE TABLE IF NOT EXISTS handling_plans (
    id TEXT PRIMARY KEY,
    suggestion_id TEXT NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    review_id TEXT NOT NULL REFERENCES professional_reviews(id) ON DELETE RESTRICT,
    department TEXT NOT NULL,
    commitment TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('proposed','accepted','implementing','completed','cancelled')),
    idempotency_key TEXT NOT NULL,
    due_at TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(suggestion_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_plan_department_status ON handling_plans(department, status, due_at);
CREATE INDEX IF NOT EXISTS idx_plan_suggestion_status ON handling_plans(suggestion_id, status);

CREATE TABLE IF NOT EXISTS conversion_outcomes (
    id TEXT PRIMARY KEY,
    suggestion_id TEXT NOT NULL UNIQUE REFERENCES items(id) ON DELETE RESTRICT,
    plan_id TEXT NOT NULL REFERENCES handling_plans(id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK(status IN ('proposed','verified','published','rejected')),
    benefit_scope TEXT NOT NULL,
    evidence_ref TEXT NOT NULL,
    reviewer TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    verified_at TEXT,
    published_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_conversion_status ON conversion_outcomes(status, created_at);

CREATE TABLE IF NOT EXISTS feedback_receipts (
    id TEXT PRIMARY KEY,
    suggestion_id TEXT NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    citizen_hash TEXT NOT NULL,
    channel TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('queued','delivering','delivered','acknowledged','permanent_failed')),
    attempt INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TEXT NOT NULL,
    lease_until TEXT,
    last_error TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(suggestion_id, citizen_hash, channel)
);

CREATE INDEX IF NOT EXISTS idx_feedback_ready ON feedback_receipts(status, next_attempt_at, lease_until);

CREATE TABLE IF NOT EXISTS outbox_events (
    id TEXT PRIMARY KEY,
    aggregate_id TEXT NOT NULL,
    topic TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending','processing','published','failed')),
    attempt INTEGER NOT NULL DEFAULT 0,
    available_at TEXT NOT NULL,
    lease_until TEXT,
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_outbox_ready ON outbox_events(status, available_at, lease_until);
