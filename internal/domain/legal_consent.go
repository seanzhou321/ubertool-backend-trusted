package domain

import "time"

// KnownLegalDocs is the canonical list of legal document names (filenames without .md extension).
// When checking consent status the backend verifies that the user has a row for every entry here.
var KnownLegalDocs = []string{
	"00_user_agreement_and_consent",
	"01_terms_of_service",
	"02_privacy_policy",
	"03_cookie_policy",
	"04_community_bylaws",
	"05_liability_waiver",
	"06_acceptable_use_policy",
	"07_eula",
}

// LegalConsent records that a user accepted a specific document at a specific version.
type LegalConsent struct {
	UserID      int32     `json:"user_id"`
	DocName     string    `json:"doc_name"`
	Version     string    `json:"version"`
	ConsentedAt time.Time `json:"consented_at"`
}
