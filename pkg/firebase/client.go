// Package firebase wraps the Firebase Admin SDK for token verification
// and FCM push notification delivery.
package firebase

import (
	"context"
	"encoding/base64"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// Client holds initialised Firebase Auth and Messaging clients.
type Client struct {
	auth      *auth.Client
	messaging *messaging.Client
}

// New initialises a Firebase Admin client from a base64-encoded
// service account JSON string (as stored in FIREBASE_SERVICE_ACCOUNT_BASE64).
func New(ctx context.Context, serviceAccountBase64 string) (*Client, error) {
	decoded, err := base64.StdEncoding.DecodeString(serviceAccountBase64)
	if err != nil {
		return nil, fmt.Errorf("firebase.New: decode service account: %w", err)
	}

	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON(decoded))
	if err != nil {
		return nil, fmt.Errorf("firebase.New: init app: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase.New: init auth: %w", err)
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase.New: init messaging: %w", err)
	}

	return &Client{auth: authClient, messaging: msgClient}, nil
}

// VerifyIDToken verifies a Firebase ID token and returns the decoded token.
// Returns an error if the token is invalid, expired, or revoked.
func (c *Client) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	token, err := c.auth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("firebase.VerifyIDToken: %w", err)
	}
	return token, nil
}

// VerifiedToken holds the identity claims the API relies on from a verified
// Firebase ID token. Email is empty for anonymous or phone sign-ins.
type VerifiedToken struct {
	UID           string
	Email         string
	EmailVerified bool
}

// TokenVerifier verifies Firebase ID tokens. Declared here so the auth
// middleware and user service can be tested without a live Firebase project.
type TokenVerifier interface {
	Verify(ctx context.Context, idToken string) (*VerifiedToken, error)
}

// Verify verifies a Firebase ID token and extracts the identity claims.
// It satisfies TokenVerifier.
func (c *Client) Verify(ctx context.Context, idToken string) (*VerifiedToken, error) {
	token, err := c.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, err
	}
	vt := &VerifiedToken{UID: token.UID}
	if email, ok := token.Claims["email"].(string); ok {
		vt.Email = email
	}
	if verified, ok := token.Claims["email_verified"].(bool); ok {
		vt.EmailVerified = verified
	}
	return vt, nil
}

// SendPushNotification sends a single FCM notification to the given device token.
// Silently returns nil if the device token is empty (unregistered device).
func (c *Client) SendPushNotification(ctx context.Context, deviceToken, title, body string) error {
	return c.SendPushNotificationWithData(ctx, deviceToken, title, body, nil)
}

// SendPushNotificationWithData is SendPushNotification with a data payload,
// which the app reads when the notification is tapped (e.g. "screen" to
// deep-link). Silently returns nil if the device token is empty.
func (c *Client) SendPushNotificationWithData(ctx context.Context, deviceToken, title, body string, data map[string]string) error {
	if deviceToken == "" {
		return nil
	}
	_, err := c.messaging.Send(ctx, &messaging.Message{
		Token: deviceToken,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	})
	if err != nil {
		return fmt.Errorf("firebase.SendPushNotification: %w", err)
	}
	return nil
}
