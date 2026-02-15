package models

// User represents a user in the system.
type User struct {
	Username  string `json:"username" badgerhold:"key"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Role      string `json:"role"`
	NavexaKey string `json:"navexa_key,omitempty"`
}
