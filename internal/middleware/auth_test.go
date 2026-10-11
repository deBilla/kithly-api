package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/kithly/kithly-ws-go/internal/middleware"
	"github.com/kithly/kithly-ws-go/pkg/firebase"
)

type fakeVerifier struct {
	tokens map[string]*firebase.VerifiedToken
}

func (f *fakeVerifier) Verify(_ context.Context, token string) (*firebase.VerifiedToken, error) {
	if vt, ok := f.tokens[token]; ok {
		return vt, nil
	}
	return nil, errors.New("bad token")
}

type fakeResolver struct {
	byUID   map[string]*middleware.Identity
	byGuest map[string]*middleware.Identity
}

func lookup(m map[string]*middleware.Identity, k string) (*middleware.Identity, error) {
	if id, ok := m[k]; ok {
		return id, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeResolver) ByFirebaseUID(_ context.Context, uid string) (*middleware.Identity, error) {
	return lookup(f.byUID, uid)
}

func (f *fakeResolver) ByGuestToken(_ context.Context, t string) (*middleware.Identity, error) {
	return lookup(f.byGuest, t)
}

var (
	alice = &middleware.Identity{UserID: "u-alice", Email: "alice@example.com"}
	guest = &middleware.Identity{UserID: "u-guest", IsGuest: true}
)

func newFakes() (*fakeVerifier, *fakeResolver) {
	v := &fakeVerifier{tokens: map[string]*firebase.VerifiedToken{
		"good":         {UID: "fb-alice", Email: "alice@example.com", EmailVerified: true},
		"unregistered": {UID: "fb-new", Email: "new@example.com"},
	}}
	r := &fakeResolver{
		byUID:   map[string]*middleware.Identity{"fb-alice": alice},
		byGuest: map[string]*middleware.Identity{"g-token": guest, "not-a-guest": alice},
	}
	return v, r
}

// serve runs one request through the given middleware and reports the status
// and the user ID the handler saw.
func serve(mw gin.HandlerFunc, headers map[string]string) (int, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.GET("/", mw, func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(middleware.ContextKeyUserID))
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		return w.Code, ""
	}
	return w.Code, w.Body.String()
}

func TestAuth(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		wantCode int
		wantUser string
	}{
		{"no credentials", nil, http.StatusUnauthorized, ""},
		{"valid firebase token", map[string]string{"Authorization": "Bearer good"}, http.StatusOK, "u-alice"},
		{"bearer scheme is case-insensitive", map[string]string{"Authorization": "bearer good"}, http.StatusOK, "u-alice"},
		{"invalid firebase token", map[string]string{"Authorization": "Bearer forged"}, http.StatusUnauthorized, ""},
		{"valid token for unregistered user", map[string]string{"Authorization": "Bearer unregistered"}, http.StatusUnauthorized, ""},
		{"unregistered token plus guest token uses guest (conversion)", map[string]string{"Authorization": "Bearer unregistered", "X-Guest-Token": "g-token"}, http.StatusOK, "u-guest"},
		{"valid guest token", map[string]string{"X-Guest-Token": "g-token"}, http.StatusOK, "u-guest"},
		{"unknown guest token", map[string]string{"X-Guest-Token": "nope"}, http.StatusUnauthorized, ""},
		{"guest token of a non-guest account", map[string]string{"X-Guest-Token": "not-a-guest"}, http.StatusUnauthorized, ""},
		{"a bare X-User-ID header is not a credential", map[string]string{"X-User-ID": "u-alice"}, http.StatusUnauthorized, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, r := newFakes()
			mw := middleware.Auth(v, r)
			code, user := serve(mw, tt.headers)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantUser, user)
		})
	}
}

func TestOptionalAuth(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		wantCode int
		wantUser string
	}{
		{"anonymous passes through", nil, http.StatusOK, ""},
		{"valid token identifies the caller", map[string]string{"Authorization": "Bearer good"}, http.StatusOK, "u-alice"},
		{"invalid token is rejected, not downgraded", map[string]string{"Authorization": "Bearer forged"}, http.StatusUnauthorized, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, r := newFakes()
			mw := middleware.OptionalAuth(v, r)
			code, user := serve(mw, tt.headers)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantUser, user)
		})
	}
}

func TestAdminKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		header   string
		wantCode int
	}{
		{"correct key", "s3cret", "s3cret", http.StatusOK},
		{"wrong key", "s3cret", "guess", http.StatusForbidden},
		{"missing header", "s3cret", "", http.StatusForbidden},
		{"unconfigured key disables the route", "", "", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{}
			if tt.header != "" {
				headers["X-Admin-Key"] = tt.header
			}
			code, _ := serve(middleware.AdminKey(tt.key), headers)
			assert.Equal(t, tt.wantCode, code)
		})
	}
}
