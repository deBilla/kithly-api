package user

// RegisterRequest is the request body for POST /user/.
type RegisterRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name"     binding:"required"`
	// IDToken is the Firebase ID token from the Authorization header, set by
	// the handler; never read from the body.
	IDToken string `json:"-"`
}

// LoginRequest is the request body for POST /user/login.
type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// SocialLoginRequest is the request body for POST /user/social-login.
// The caller is identified by the Firebase ID token in the Authorization header.
type SocialLoginRequest struct {
	Email       string `json:"email"       binding:"required,email"`
	Name        string `json:"name"`
	GuestUserID string `json:"guestUserId"`
	// IDToken is the Firebase ID token from the Authorization header, set by
	// the handler; never read from the body.
	IDToken string `json:"-"`
}

// EmailLoginRequest is the request body for POST /user/email-login.
// The caller is identified by the Firebase ID token in the Authorization header.
type EmailLoginRequest struct {
	Email       string `json:"email"       binding:"required,email"`
	GuestUserID string `json:"guestUserId"`
	// IDToken is the Firebase ID token from the Authorization header, set by
	// the handler; never read from the body.
	IDToken string `json:"-"`
}

// ConvertGuestRequest is the request body for POST /user/convert-guest.
type ConvertGuestRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name"     binding:"required"`
	// IDToken is the Firebase ID token from the Authorization header, set by
	// the handler; never read from the body.
	IDToken string `json:"-"`
}

// UpdateProfileRequest is the request body for PUT /user/me.
type UpdateProfileRequest struct {
	Name   *string `json:"name"`
	Avatar *string `json:"avatar"`
}

// RegisterDeviceTokenRequest is the request body for POST /user/device-token.
type RegisterDeviceTokenRequest struct {
	Token string `json:"fcm_token" binding:"required"`
}

// AuthResponse wraps the user view with an authentication token.
type AuthResponse struct {
	User  View   `json:"user"`
	Token string `json:"token,omitempty"`
}
