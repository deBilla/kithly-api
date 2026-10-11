package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kithly/kithly-ws-go/internal/core/pghelper"
	"github.com/kithly/kithly-ws-go/internal/core/sqlcdb"
)

// finalVideoNotifier tells a card's organizer that the final video is ready.
// The final compilation can take minutes, so organizers have usually left the
// app by the time it lands.
type finalVideoNotifier interface {
	NotifyFinalVideoReady(ctx context.Context, cardID string) error
}

// notifyStore is the subset of sqlcdb.Queries the notifier reads.
type notifyStore interface {
	GetCardByID(ctx context.Context, id pgtype.UUID) (sqlcdb.VideoCard, error)
	GetUserByID(ctx context.Context, id pgtype.UUID) (sqlcdb.User, error)
}

// pusher sends a push notification. Satisfied by *firebase.Client.
type pusher interface {
	SendPushNotificationWithData(ctx context.Context, deviceToken, title, body string, data map[string]string) error
}

// organizerNotifier pushes to the organizer's registered device.
type organizerNotifier struct {
	store notifyStore
	push  pusher
}

// NotifyFinalVideoReady returns nil without sending when the organizer has no
// registered device, since that is a normal state rather than a failure.
func (n *organizerNotifier) NotifyFinalVideoReady(ctx context.Context, cardID string) error {
	card, err := n.store.GetCardByID(ctx, pghelper.UUID(cardID))
	if err != nil {
		return fmt.Errorf("organizerNotifier.NotifyFinalVideoReady: card: %w", err)
	}
	organizer, err := n.store.GetUserByID(ctx, card.OrganizerID)
	if err != nil {
		return fmt.Errorf("organizerNotifier.NotifyFinalVideoReady: organizer: %w", err)
	}
	if organizer.FcmToken.String == "" {
		return nil
	}

	title := "Your video is ready! 🎉"
	body := fmt.Sprintf("\"%s\" is ready to watch and share.", card.Title)
	// The app routes a tapped notification to data.screen.
	data := map[string]string{"screen": "app/detail/" + cardID}
	if err := n.push.SendPushNotificationWithData(ctx, organizer.FcmToken.String, title, body, data); err != nil {
		return fmt.Errorf("organizerNotifier.NotifyFinalVideoReady: %w", err)
	}
	return nil
}
