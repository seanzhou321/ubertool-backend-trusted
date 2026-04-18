
-- Record of each user's acceptance of each document version.
-- doc_name = filename without .md extension, e.g. '00_user_agreement_and_consent'
-- version  = ISO date string matching CURRENT_LEGAL_VERSION, e.g. '2026-04-15'
CREATE TABLE user_legal_consents (
    user_id      INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    doc_name     TEXT        NOT NULL,
    version      TEXT        NOT NULL,
    consented_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, doc_name, version)
);

CREATE INDEX idx_user_legal_consents_user_id ON user_legal_consents(user_id);
