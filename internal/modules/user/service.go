package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/pkg/firebase"
)

// Service defines the business logic contract for user management.
type Service interface {
	// FindEmailByID resolves a user's email by their ID.
	// Implements middleware.UserResolver.
	FindEmailByID(ctx context.Context, id string) (string, error)
	GetByID(ctx context.Context, id string) (*Entity, error)
	List(ctx context.Context) ([]*Entity, error)
	Register(ctx context.Context, req RegisterRequest) (*Entity, error)
	Login(ctx context.Context, req LoginRequest) (*Entity, error)
	SocialLogin(ctx context.Context, req SocialLoginRequest) (*Entity, error)
	EmailLogin(ctx context.Context, req EmailLoginRequest) (*Entity, error)
	CreateGuest(ctx context.Context) (*Entity, error)
	ConvertGuest(ctx context.Context, userID string, req ConvertGuestRequest) (*Entity, error)
	UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*Entity, error)
	RegisterDeviceToken(ctx context.Context, userID string, req RegisterDeviceTokenRequest) error
	Delete(ctx context.Context, id string) error
}

type service struct {
	repo     Repository
	verifier firebase.TokenVerifier
}

// NewService constructs a Service. Every Firebase login and registration must
// carry a verified Firebase ID token; nothing in the request body is trusted
// as a credential.
func NewService(repo Repository, verifier firebase.TokenVerifier) Service {
	return &service{repo: repo, verifier: verifier}
}

func (s *service) FindEmailByID(ctx context.Context, id string) (string, error) {
	return s.repo.FindEmailByID(ctx, id)
}

func (s *service) GetByID(ctx context.Context, id string) (*Entity, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) List(ctx context.Context) ([]*Entity, error) {
	return s.repo.List(ctx)
}

func (s *service) Register(ctx context.Context, req RegisterRequest) (*Entity, error) {
	var firebaseUID *string
	if req.IDToken != "" {
		vt, err := s.verify(ctx, req.IDToken)
		if err != nil {
			return nil, err
		}
		if vt.Email == "" || !strings.EqualFold(vt.Email, req.Email) {
			return nil, apperr.Unauthorized("token email does not match")
		}
		firebaseUID = &vt.UID
	} else {
		return nil, apperr.Unauthorized("id token required")
	}

	// Prevent duplicate email.
	if _, err := s.repo.FindByEmail(ctx, req.Email); err == nil {
		return nil, apperr.Conflict("email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("user.service.Register: hash password: %w", err)
	}
	hashStr := string(hash)

	u, err := s.repo.Create(ctx, CreateParams{
		Email:        &req.Email,
		PasswordHash: &hashStr,
		Name:         req.Name,
		FirebaseUID:  firebaseUID,
		Avatar:       "❤️",
		MaxCards:     10,
	})
	if err != nil {
		return nil, fmt.Errorf("user.service.Register: %w", err)
	}
	return u, nil
}

func (s *service) Login(ctx context.Context, req LoginRequest) (*Entity, error) {
	u, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperr.Unauthorized("invalid email or password")
	}
	if u.PasswordHash == "" {
		return nil, apperr.Unauthorized("invalid email or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, apperr.Unauthorized("invalid email or password")
	}
	_ = s.repo.UpdateLastLogin(ctx, u.ID)
	return u, nil
}

func (s *service) SocialLogin(ctx context.Context, req SocialLoginRequest) (*Entity, error) {
	if req.IDToken == "" {
		return nil, apperr.Unauthorized("id token required")
	}
	return s.loginWithToken(ctx, req.IDToken, req.Name)
}

func (s *service) EmailLogin(ctx context.Context, req EmailLoginRequest) (*Entity, error) {
	if req.IDToken == "" {
		return nil, apperr.Unauthorized("id token required")
	}
	return s.loginWithToken(ctx, req.IDToken, "")
}

// loginWithToken signs in (or signs up) the owner of a verified Firebase ID
// token. Nothing in the request body is trusted.
func (s *service) loginWithToken(ctx context.Context, idToken, name string) (*Entity, error) {
	vt, err := s.verify(ctx, idToken)
	if err != nil {
		return nil, err
	}

	if u, err := s.repo.FindByFirebaseUID(ctx, vt.UID); err == nil {
		_ = s.repo.UpdateLastLogin(ctx, u.ID)
		return u, nil
	}

	if vt.Email == "" {
		return nil, apperr.BadRequest("token has no email")
	}

	if u, err := s.repo.FindByEmail(ctx, vt.Email); err == nil {
		// Accounts created before UIDs were stored need linking. Link only when
		// the caller provably owns the account: either Firebase verified the
		// email, or the account was registered with this exact UID as its
		// password (the app's historical registration scheme).
		if !vt.EmailVerified && !passwordMatches(u, vt.UID) {
			return nil, apperr.Conflict("an account with this email already exists")
		}
		linked, err := s.repo.LinkFirebaseUID(ctx, u.ID, vt.UID)
		if err != nil {
			return nil, fmt.Errorf("user.service.loginWithToken: link: %w", err)
		}
		_ = s.repo.UpdateLastLogin(ctx, linked.ID)
		return linked, nil
	}

	return s.createFirebaseUser(ctx, vt.Email, vt.UID, name, vt.EmailVerified)
}

// createFirebaseUser stores the UID as both firebase_uid and the password
// hash, matching accounts created by earlier API versions.
func (s *service) createFirebaseUser(ctx context.Context, email, uid, name string, emailVerified bool) (*Entity, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(uid), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("user.service.createFirebaseUser: hash: %w", err)
	}
	hashStr := string(hash)

	u, err := s.repo.Create(ctx, CreateParams{
		Email:         &email,
		Name:          name,
		FirebaseUID:   &uid,
		PasswordHash:  &hashStr,
		EmailVerified: emailVerified,
		Avatar:        "❤️",
		MaxCards:      5,
	})
	if err != nil {
		return nil, fmt.Errorf("user.service.createFirebaseUser: %w", err)
	}
	return u, nil
}

func (s *service) verify(ctx context.Context, idToken string) (*firebase.VerifiedToken, error) {
	vt, err := s.verifier.Verify(ctx, idToken)
	if err != nil {
		return nil, apperr.Unauthorized("invalid or expired token")
	}
	return vt, nil
}

func passwordMatches(u *Entity, secret string) bool {
	if u.PasswordHash == "" || secret == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(secret)) == nil
}

func (s *service) CreateGuest(ctx context.Context) (*Entity, error) {
	guestToken := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	_ = expiresAt // stored via GuestExpiresAt — repo Create handles nil for now

	u, err := s.repo.Create(ctx, CreateParams{
		Name:       "Guest",
		Avatar:     "❤️",
		MaxCards:   1,
		IsGuest:    true,
		GuestToken: &guestToken,
	})
	if err != nil {
		return nil, fmt.Errorf("user.service.CreateGuest: %w", err)
	}
	return u, nil
}

func (s *service) ConvertGuest(ctx context.Context, userID string, req ConvertGuestRequest) (*Entity, error) {
	guest, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !guest.IsGuest {
		return nil, apperr.BadRequest("user is not a guest account")
	}

	var firebaseUID string
	if req.IDToken != "" {
		vt, err := s.verify(ctx, req.IDToken)
		if err != nil {
			return nil, err
		}
		if vt.Email == "" || !strings.EqualFold(vt.Email, req.Email) {
			return nil, apperr.Unauthorized("token email does not match")
		}
		firebaseUID = vt.UID
	} else {
		return nil, apperr.Unauthorized("id token required")
	}

	// Prevent duplicate email.
	if _, err := s.repo.FindByEmail(ctx, req.Email); err == nil {
		return nil, apperr.Conflict("email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("user.service.ConvertGuest: hash: %w", err)
	}

	u, err := s.repo.ConvertGuest(ctx, userID, ConvertGuestParams{
		Email:        req.Email,
		PasswordHash: string(hash),
		Name:         req.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("user.service.ConvertGuest: %w", err)
	}
	if firebaseUID != "" {
		if u, err = s.repo.LinkFirebaseUID(ctx, u.ID, firebaseUID); err != nil {
			return nil, fmt.Errorf("user.service.ConvertGuest: link: %w", err)
		}
	}
	return u, nil
}

func (s *service) UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*Entity, error) {
	u, err := s.repo.Update(ctx, userID, UpdateParams{
		Name:   req.Name,
		Avatar: req.Avatar,
	})
	if err != nil {
		return nil, fmt.Errorf("user.service.UpdateProfile: %w", err)
	}
	return u, nil
}

func (s *service) RegisterDeviceToken(ctx context.Context, userID string, req RegisterDeviceTokenRequest) error {
	if err := s.repo.UpdateFCMToken(ctx, userID, req.Token); err != nil {
		return fmt.Errorf("user.service.RegisterDeviceToken: %w", err)
	}
	return nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
