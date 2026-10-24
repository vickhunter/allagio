package services

import (
	"regexp"
	"strings"
)

// Draft personal data is removed before matching and before the hash is
// stored. The original draft is not written to the database, a log, or the
// response. What may be stored is the SHA-256 of the stripped text and the
// keywords that overlapped an offer.
var (
	draftEmail  = regexp.MustCompile(`(?i)[a-z0-9][a-z0-9._%+\-]{0,64}@[a-z0-9][a-z0-9.\-]{0,253}\.[a-z]{2,24}`)
	draftPhone  = regexp.MustCompile(`(?:(?:\+|00)\d{1,3}[\s.\-]?)?(?:\(?\d{3}\)?[\s.\-]?)?\d{3}[\s.\-]\d{4}\b`)
	draftHandle = regexp.MustCompile(`(^|[^A-Za-z0-9_])@[A-Za-z0-9_]{1,30}\b`)
)

// StripDraftPII removes email addresses, @handles, and phone numbers.
// The remaining text is what the scorer sees. Handles are removed after
// emails so the address's @ is not left behind as a handle.
func StripDraftPII(text string) string {
	text = draftEmail.ReplaceAllString(text, " ")
	text = draftPhone.ReplaceAllString(text, " ")
	text = draftHandle.ReplaceAllString(text, "${1} ")
	return strings.Join(strings.Fields(text), " ")
}
