package importer

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bobmcallan/vire-portal/internal/models"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
	"github.com/timshannon/badgerhold/v4"
	"golang.org/x/crypto/bcrypt"
)

// usersFile represents the JSON structure of the users import file.
type usersFile struct {
	Users []models.User `json:"users"`
}

// ImportUsers reads users from a JSON file and imports them into BadgerDB.
// Existing users (matched by username key) are skipped.
// Passwords are bcrypt-hashed before storage.
func ImportUsers(db interface{}, logger *common.Logger, jsonPath string) error {
	store, ok := db.(*badgerhold.Store)
	if !ok {
		return fmt.Errorf("db is not a *badgerhold.Store")
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read users file %s: %w", jsonPath, err)
	}

	var file usersFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("failed to parse users file %s: %w", jsonPath, err)
	}

	for _, u := range file.Users {
		// Check if user already exists
		var existing models.User
		err := store.Get(u.Username, &existing)
		if err == nil {
			logger.Debug().Str("username", u.Username).Msg("user already exists, skipping")
			continue
		}
		if err != badgerhold.ErrNotFound {
			return fmt.Errorf("failed to check user %s: %w", u.Username, err)
		}

		// bcrypt has a 72-byte input limit; truncate to avoid errors
		pwd := []byte(u.Password)
		if len(pwd) > 72 {
			pwd = pwd[:72]
		}
		hash, err := bcrypt.GenerateFromPassword(pwd, bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password for user %s: %w", u.Username, err)
		}
		u.Password = string(hash)

		// Insert user
		if err := store.Insert(u.Username, &u); err != nil {
			return fmt.Errorf("failed to insert user %s: %w", u.Username, err)
		}
		logger.Info().Str("username", u.Username).Str("role", u.Role).Msg("user imported")
	}

	return nil
}
