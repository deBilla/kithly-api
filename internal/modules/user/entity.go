// Package user implements user authentication, registration, and profile management.
package user

import "time"

// Entity represents an application user (registered or guest).
// PasswordHash is kept in the entity for internal login verification only;
// it is never included in API responses.
type Entity struct {
	ID             string
	Email          string
	PasswordHash   string
	Name           string
	Avatar         string
	EmailVerified  bool
	IsGuest        bool
	GuestToken     string
	FirebaseUID    string
	FCMToken       string
	MaxCards       int
	LastLoginAt    *time.Time
	GuestExpiresAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// View is the public representation of a user returned in API responses.
type View struct {
	ID            string  `json:"id"`
	Email         string  `json:"email"`
	Name          string  `json:"name"`
	Avatar        string  `json:"avatar"`
	EmailVerified bool    `json:"email_verified"`
	IsGuest       bool    `json:"is_guest"`
	MaxCards      int     `json:"max_cards"`
	CreatedAt     string  `json:"created_at"`
}

// ToView converts the entity to its API representation.
func (e *Entity) ToView() View {
	return View{
		ID:            e.ID,
		Email:         e.Email,
		Name:          e.Name,
		Avatar:        e.Avatar,
		EmailVerified: e.EmailVerified,
		IsGuest:       e.IsGuest,
		MaxCards:      e.MaxCards,
		CreatedAt:     e.CreatedAt.Format(time.RFC3339),
	}
}
