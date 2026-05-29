package config

import (
	"crypto/subtle"
	"encoding/json"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// EnsureAuthHashed migrates plaintext passwords to bcrypt and clears Password.
func (a *AuthConfig) EnsureAuthHashed() error {
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
	hash, err := HashPassword(a.Password)
	if err != nil {
		return err
	}
	a.PasswordHash = hash
	a.Password = ""
	return nil
}

// HashPassword returns a bcrypt hash of password.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword reports whether password matches the stored hash (or legacy plaintext during migration).
func (a *AuthConfig) CheckPassword(password string) bool {
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

// UnmarshalJSON accepts legacy plaintext password fields.
func (a *AuthConfig) UnmarshalJSON(data []byte) error {
	type authAlias AuthConfig
	var raw struct {
		authAlias
		Password string `json:"password"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = AuthConfig(raw.authAlias)
	if raw.Password != "" && a.Password == "" {
		a.Password = raw.Password
	}
	return nil
}

// MarshalJSON never writes plaintext password to disk.
func (a AuthConfig) MarshalJSON() ([]byte, error) {
	type authAlias AuthConfig
	alias := authAlias(a)
	alias.Password = ""
	return json.Marshal(alias)
}
