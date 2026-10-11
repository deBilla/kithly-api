package user

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/pkg/firebase"
)

// fakeRepo is an in-memory Repository covering the methods login touches.
type fakeRepo struct {
	users  []*Entity
	nextID int
}

func (r *fakeRepo) find(match func(*Entity) bool) (*Entity, error) {
	for _, u := range r.users {
		if match(u) {
			return u, nil
		}
	}
	return nil, apperr.NotFound("user")
}

func (r *fakeRepo) FindByID(_ context.Context, id string) (*Entity, error) {
	return r.find(func(u *Entity) bool { return u.ID == id })
}

func (r *fakeRepo) FindByEmail(_ context.Context, email string) (*Entity, error) {
	return r.find(func(u *Entity) bool { return u.Email != "" && u.Email == email })
}

func (r *fakeRepo) FindByFirebaseUID(_ context.Context, uid string) (*Entity, error) {
	return r.find(func(u *Entity) bool { return u.FirebaseUID != "" && u.FirebaseUID == uid })
}

func (r *fakeRepo) FindByGuestToken(_ context.Context, token string) (*Entity, error) {
	return r.find(func(u *Entity) bool { return u.GuestToken == token })
}

func (r *fakeRepo) LinkFirebaseUID(ctx context.Context, id, uid string) (*Entity, error) {
	u, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	u.FirebaseUID = uid
	return u, nil
}

func (r *fakeRepo) Create(_ context.Context, p CreateParams) (*Entity, error) {
	r.nextID++
	u := &Entity{ID: "new-" + string(rune('0'+r.nextID)), Name: p.Name, IsGuest: p.IsGuest, EmailVerified: p.EmailVerified}
	if p.Email != nil {
		u.Email = *p.Email
	}
	if p.PasswordHash != nil {
		u.PasswordHash = *p.PasswordHash
	}
	if p.FirebaseUID != nil {
		u.FirebaseUID = *p.FirebaseUID
	}
	if p.GuestToken != nil {
		u.GuestToken = *p.GuestToken
	}
	r.users = append(r.users, u)
	return u, nil
}

func (r *fakeRepo) FindEmailByID(context.Context, string) (string, error) { return "", nil }
func (r *fakeRepo) List(context.Context) ([]*Entity, error)               { return r.users, nil }
func (r *fakeRepo) Update(context.Context, string, UpdateParams) (*Entity, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeRepo) UpdateFCMToken(context.Context, string, string) error { return nil }
func (r *fakeRepo) UpdateLastLogin(context.Context, string) error        { return nil }
func (r *fakeRepo) ConvertGuest(ctx context.Context, id string, p ConvertGuestParams) (*Entity, error) {
	u, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	u.IsGuest, u.Email, u.Name, u.PasswordHash = false, p.Email, p.Name, p.PasswordHash
	return u, nil
}
func (r *fakeRepo) Delete(context.Context, string) error              { return nil }
func (r *fakeRepo) CountCards(context.Context, string) (int64, error) { return 0, nil }

type fakeVerifier map[string]*firebase.VerifiedToken

func (f fakeVerifier) Verify(_ context.Context, token string) (*firebase.VerifiedToken, error) {
	if vt, ok := f[token]; ok {
		return vt, nil
	}
	return nil, errors.New("bad token")
}

func hashOf(t *testing.T, secret string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

// victim is an account registered through POST /user by an older app build:
// password = Firebase UID, firebase_uid never stored.
func newRepoWithVictim(t *testing.T) *fakeRepo {
	return &fakeRepo{users: []*Entity{{
		ID:           "victim",
		Email:        "victim@example.com",
		PasswordHash: hashOf(t, "fb-victim"),
	}}}
}

func isStatus(err error, sentinel error) bool {
	var ae *apperr.AppError
	return errors.As(err, &ae) && errors.Is(ae.Err, sentinel)
}

func TestSocialLogin_RequiresToken(t *testing.T) {
	svc := NewService(newRepoWithVictim(t), fakeVerifier{})
	_, err := svc.SocialLogin(context.Background(), SocialLoginRequest{Email: "victim@example.com"})
	assert.True(t, isStatus(err, apperr.ErrUnauthorized), "got %v", err)
}

func TestSocialLogin_WithToken(t *testing.T) {
	verifier := fakeVerifier{
		"victim-verified":     {UID: "fb-victim-google", Email: "victim@example.com", EmailVerified: true},
		"victim-same-uid":     {UID: "fb-victim", Email: "victim@example.com"},
		"attacker-unverified": {UID: "fb-attacker", Email: "victim@example.com"},
		"fresh":               {UID: "fb-fresh", Email: "fresh@example.com", EmailVerified: true},
		"no-email":            {UID: "fb-anon"},
	}

	tests := []struct {
		name      string
		token     string
		bodyEmail string
		wantID    string
		wantLink  string
		wantErr   error
		wantNew   bool
	}{
		{name: "verified email links the existing account", token: "victim-verified", wantID: "victim", wantLink: "fb-victim-google"},
		{name: "unverified email links when the UID matches the password", token: "victim-same-uid", wantID: "victim", wantLink: "fb-victim"},
		{name: "unverified email with a different UID is refused", token: "attacker-unverified", wantErr: apperr.ErrConflict},
		{name: "body email is ignored in favour of the token", token: "fresh", bodyEmail: "victim@example.com", wantNew: true},
		{name: "token without email cannot create an account", token: "no-email", wantErr: apperr.ErrBadRequest},
		{name: "forged token", token: "forged", wantErr: apperr.ErrUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepoWithVictim(t)
			svc := NewService(repo, verifier)
			u, err := svc.SocialLogin(context.Background(), SocialLoginRequest{
				Email: tt.bodyEmail, IDToken: tt.token,
			})
			if tt.wantErr != nil {
				assert.True(t, isStatus(err, tt.wantErr), "got %v", err)
				return
			}
			require.NoError(t, err)
			if tt.wantNew {
				assert.Equal(t, "fresh@example.com", u.Email)
				assert.NotEqual(t, "victim", u.ID)
				return
			}
			assert.Equal(t, tt.wantID, u.ID)
			assert.Equal(t, tt.wantLink, u.FirebaseUID)
		})
	}
}

func TestLoginWithToken_ReturningUserByUID(t *testing.T) {
	repo := &fakeRepo{users: []*Entity{{ID: "u1", Email: "a@example.com", FirebaseUID: "fb-1"}}}
	svc := NewService(repo, fakeVerifier{"t": {UID: "fb-1", Email: "a@example.com"}})
	u, err := svc.EmailLogin(context.Background(), EmailLoginRequest{Email: "a@example.com", IDToken: "t"})
	require.NoError(t, err)
	assert.Equal(t, "u1", u.ID)
}

func TestRegister(t *testing.T) {
	verifier := fakeVerifier{"t": {UID: "fb-new", Email: "new@example.com"}}
	ctx := context.Background()

	t.Run("token stores the Firebase UID", func(t *testing.T) {
		svc := NewService(&fakeRepo{}, verifier)
		u, err := svc.Register(ctx, RegisterRequest{Email: "new@example.com", Password: "fb-new", Name: "N", IDToken: "t"})
		require.NoError(t, err)
		assert.Equal(t, "fb-new", u.FirebaseUID)
	})

	t.Run("token for a different email is refused", func(t *testing.T) {
		svc := NewService(&fakeRepo{}, verifier)
		_, err := svc.Register(ctx, RegisterRequest{Email: "other@example.com", Password: "fb-new", Name: "N", IDToken: "t"})
		assert.True(t, isStatus(err, apperr.ErrUnauthorized), "got %v", err)
	})

	t.Run("no token", func(t *testing.T) {
		svc := NewService(&fakeRepo{}, verifier)
		_, err := svc.Register(ctx, RegisterRequest{Email: "new@example.com", Password: "fb-new", Name: "N"})
		assert.True(t, isStatus(err, apperr.ErrUnauthorized), "got %v", err)
	})
}

func TestConvertGuest_LinksUID(t *testing.T) {
	repo := &fakeRepo{users: []*Entity{{ID: "g1", IsGuest: true, GuestToken: "gt"}}}
	svc := NewService(repo, fakeVerifier{"t": {UID: "fb-g", Email: "g@example.com"}})
	u, err := svc.ConvertGuest(context.Background(), "g1", ConvertGuestRequest{Email: "g@example.com", Password: "fb-g", Name: "G", IDToken: "t"})
	require.NoError(t, err)
	assert.False(t, u.IsGuest)
	assert.Equal(t, "fb-g", u.FirebaseUID)
}
