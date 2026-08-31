package store

import "strings"

// Fold returns the normalised form of a value matched case-insensitively.
//
// The database never folds case (SPEC.md §13.2 rule 4). Every value compared
// without regard to case — usernames, emails, labels, team codes, state and
// view names — is stored twice: as typed, and folded by this function into an
// explicit column carrying the unique index.
func Fold(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
