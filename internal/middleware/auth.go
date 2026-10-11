package middleware

import (
	"context"
	"crypto/subtle"
	"strings"

	"github.com/gin-gonic/gin"

	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/pkg/firebase"
)

const (
	// ContextKeyUserID is the Gin context key for the authenticated user's ID.
	ContextKeyUserID = "userID"
	// ContextKeyUserEmail is the Gin context key for the authenticated user's email.
	ContextKeyUserEmail = "userEmail"
	// ContextKeyAuthMethod records which credential authenticated the request
	// ("firebase" or "guest"), for logging.
	ContextKeyAuthMethod = "authMethod"

	// HeaderGuestToken carries the opaque token issued by POST /user/guest.
	HeaderGuestToken = "X-Guest-Token"
	// HeaderAdminKey carries the shared secret for reference-data admin routes.
	HeaderAdminKey = "X-Admin-Key"
)

// Identity is the authenticated principal attached to a request.
type Identity struct {
	UserID  string
	Email   string
	IsGuest bool
}

// IdentityResolver maps each accepted credential to a Kithly user.
// Implemented in cmd/api/main.go over the user repository, which avoids an
// import cycle (the user package imports this one for its context keys).
type IdentityResolver interface {
	ByFirebaseUID(ctx context.Context, uid string) (*Identity, error)
	ByGuestToken(ctx context.Context, token string) (*Identity, error)
}

// Auth returns middleware that rejects requests without a valid credential.
//
// Accepted, in order of precedence:
//  1. Authorization: Bearer <Firebase ID token>
//  2. X-Guest-Token: <token from POST /user/guest>
func Auth(verifier firebase.TokenVerifier, resolver IdentityResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, present, err := authenticate(c, verifier, resolver)
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}
		if !present {
			_ = c.Error(apperr.Unauthorized("missing credentials"))
			c.Abort()
			return
		}
		setIdentity(c, id)
		c.Next()
	}
}

// OptionalAuth is Auth for endpoints that anonymous contributors may call.
// A request with no credentials passes through anonymously; a request with
// credentials that fail verification is rejected rather than silently
// downgraded, so a client bug never looks like an anonymous upload.
func OptionalAuth(verifier firebase.TokenVerifier, resolver IdentityResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, present, err := authenticate(c, verifier, resolver)
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}
		if present {
			setIdentity(c, id)
		}
		c.Next()
	}
}

// AdminKey guards reference-data write routes with a shared secret. An empty
// key disables those routes entirely instead of leaving them open.
func AdminKey(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got := c.GetHeader(HeaderAdminKey)
		if key == "" || subtle.ConstantTimeCompare([]byte(got), []byte(key)) != 1 {
			_ = c.Error(apperr.Forbidden("admin access required"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// BearerToken returns the token from an "Authorization: Bearer" header, or "".
func BearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// authenticate resolves the request's identity. present reports whether any
// credential was supplied at all, so callers can tell anonymous from invalid.
func authenticate(c *gin.Context, verifier firebase.TokenVerifier, resolver IdentityResolver) (*Identity, bool, error) {
	ctx := c.Request.Context()

	if token := BearerToken(c); token != "" {
		vt, err := verifier.Verify(ctx, token)
		if err != nil {
			return nil, true, apperr.Unauthorized("invalid or expired token")
		}
		id, err := resolver.ByFirebaseUID(ctx, vt.UID)
		if err == nil {
			c.Set(ContextKeyAuthMethod, "firebase")
			return id, true, nil
		}
		// Signed in with Firebase but not yet registered with the API. That is
		// expected mid-way through converting a guest, when the client sends
		// both the new ID token and the guest token; otherwise reject.
		if c.GetHeader(HeaderGuestToken) == "" {
			return nil, true, apperr.Unauthorized("user not registered")
		}
	}

	if token := c.GetHeader(HeaderGuestToken); token != "" {
		id, err := resolver.ByGuestToken(ctx, token)
		if err != nil || !id.IsGuest {
			return nil, true, apperr.Unauthorized("invalid guest token")
		}
		c.Set(ContextKeyAuthMethod, "guest")
		return id, true, nil
	}

	return nil, false, nil
}

func setIdentity(c *gin.Context, id *Identity) {
	c.Set(ContextKeyUserID, id.UserID)
	c.Set(ContextKeyUserEmail, id.Email)
}
