package models

import "time"

// User represents a portal user.
type User struct {
	ID        string    `json:"id" badgerhold:"key"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"` // "google" or "github"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
