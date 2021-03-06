package models

import (
	"github.com/google/logger"
	"golang.org/x/crypto/bcrypt"

	"github.com/jsannemo/omogenjudge/frontend/paths"
)

// An Account represents a user account from the `account` table.
type Account struct {
	// Internal account identifier. This should not be exposed externally.
	AccountID int32 `db:"account_id"`
	// External account identifier. This can be exposed externally and used in URLs.
	Username string `db:"username"`
	// A bcrypt hash of the user's password.
	PasswordHash string `db:"password_hash"`
	FullName     string `db:"full_name"`
	Email        string `db:"email"`
}

// SetPassword sets the account password hash to the hash of the given password.
func (a *Account) SetPassword(password string) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		logger.Fatalf("Could not hash password: %v", hash)
	}
	a.PasswordHash = string(hash)
}

// Matches returns whether the given password matches the account password hash.
func (a *Account) MatchesPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false
	} else if err != nil {
		logger.Fatalf("Could not match password: %v", err)
	}
	return true
}

// Link returns a link to the account's user page.
func (a *Account) Link() string {
	return paths.Route(paths.User, paths.UserNameArg, a.Username)
}
