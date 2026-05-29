package config

import (
	"crypto/subtle"
	"encoding/json"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// HashPassword returns a bcrypt hash of password.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// AuthCredentials holds login identity and password material.
type AuthCredentials struct {
	Username     string `json:"username"`
	Password     string `json:"password,omitempty"` // API/file input only; cleared after hash
	PasswordHash string `json:"password_hash,omitempty"`
	plainPassword string `json:"-"` // in-memory for outbound HTTP Basic (peer sync)
}

// SetPassword hashes and stores a new password, keeping plain text for Basic auth.
func (a *AuthCredentials) SetPassword(password string) error {
	pw := strings.TrimSpace(password)
	if pw == "" {
		return nil
	}
	hash, err := HashPassword(pw)
	if err != nil {
		return err
	}
	a.PasswordHash = hash
	a.Password = ""
	a.plainPassword = pw
	return nil
}

// PasswordForBasicAuth returns the cleartext password for outbound HTTP Basic auth.
func (a *AuthCredentials) PasswordForBasicAuth() string {
	if a == nil {
		return ""
	}
	if a.plainPassword != "" {
		return a.plainPassword
	}
	return a.Password
}

// EnsureHashed migrates plaintext passwords to bcrypt and clears Password.
func (a *AuthCredentials) EnsureHashed() error {
	if a == nil {
		return nil
	}
	if a.PasswordHash != "" {
		a.Password = ""
		return nil
	}
	if a.Password == "" {
		return nil
	}
	return a.SetPassword(a.Password)
}

// CheckPassword reports whether password matches the stored hash.
func (a *AuthCredentials) CheckPassword(password string) bool {
	if a == nil {
		return false
	}
	if a.PasswordHash != "" {
		err := bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password))
		return err == nil
	}
	if a.Password != "" {
		return subtle.ConstantTimeCompare([]byte(a.Password), []byte(password)) == 1
	}
	return false
}

// UnmarshalJSON accepts plaintext password on load.
func (a *AuthCredentials) UnmarshalJSON(data []byte) error {
	type authAlias AuthCredentials
	var raw struct {
		authAlias
		Password string `json:"password"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = AuthCredentials(raw.authAlias)
	if raw.Password != "" && a.Password == "" {
		a.Password = raw.Password
	}
	if a.Password != "" && a.plainPassword == "" {
		a.plainPassword = strings.TrimSpace(a.Password)
	}
	return nil
}

// MarshalJSON never writes plaintext password to disk.
func (a AuthCredentials) MarshalJSON() ([]byte, error) {
	type authAlias AuthCredentials
	alias := authAlias(a)
	alias.Password = ""
	alias.plainPassword = ""
	return json.Marshal(alias)
}
