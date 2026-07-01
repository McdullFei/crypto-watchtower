package auth

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// ValidateStrongPassword enforces the account password complexity policy.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param password Raw password to validate.
// @returns Error when the password is weak.
func ValidateStrongPassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return errors.New("password must include uppercase, lowercase, digit, and special characters")
	}
	if strings.ContainsAny(password, " \t\r\n") {
		return errors.New("password must not contain whitespace")
	}
	return nil
}

// HashPassword hashes a validated raw password with bcrypt.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param password Raw password.
// @returns Bcrypt password hash.
func HashPassword(password string) (string, error) {
	if err := ValidateStrongPassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a raw password against a bcrypt hash.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param hash Stored bcrypt hash.
// @param password Raw password.
// @returns Error when the password does not match.
func CheckPassword(hash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
