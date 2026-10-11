package user

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/kithly/kithly-ws-go/internal/middleware"
)

func TestSelfOnlyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{users: []*Entity{{ID: "me"}, {ID: "someone-else"}}}
	h := NewHandler(NewService(repo, fakeVerifier{}))

	asUser := func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "me"); c.Next() }
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	RegisterRoutes(r.Group("/v1"), h, asUser, middleware.AdminKey("k"))

	tests := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/v1/user/me", http.StatusOK},
		{http.MethodGet, "/v1/user/someone-else", http.StatusForbidden},
		{http.MethodDelete, "/v1/user/me", http.StatusNoContent},
		{http.MethodDelete, "/v1/user/someone-else", http.StatusForbidden},
		{http.MethodGet, "/v1/user", http.StatusForbidden}, // listing needs the admin key
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil).WithContext(context.Background()))
			assert.Equal(t, tt.want, w.Code)
		})
	}
}
