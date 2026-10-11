package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kithly/kithly-ws-go/internal/core/pghelper"
	"github.com/kithly/kithly-ws-go/internal/core/sqlcdb"
)

const (
	testCardID      = "11111111-1111-1111-1111-111111111111"
	testOrganizerID = "22222222-2222-2222-2222-222222222222"
)

type fakeNotifyStore struct {
	card    *sqlcdb.VideoCard
	user    *sqlcdb.User
	cardErr error
}

func (f fakeNotifyStore) GetCardByID(_ context.Context, _ pgtype.UUID) (sqlcdb.VideoCard, error) {
	if f.cardErr != nil {
		return sqlcdb.VideoCard{}, f.cardErr
	}
	return *f.card, nil
}

func (f fakeNotifyStore) GetUserByID(_ context.Context, id pgtype.UUID) (sqlcdb.User, error) {
	if f.user == nil || f.user.ID != id {
		return sqlcdb.User{}, errors.New("no rows")
	}
	return *f.user, nil
}

type sentPush struct {
	token, title, body string
	data               map[string]string
}

type fakePusher struct{ sent []sentPush }

func (f *fakePusher) SendPushNotificationWithData(_ context.Context, token, title, body string, data map[string]string) error {
	f.sent = append(f.sent, sentPush{token, title, body, data})
	return nil
}

func testCard() *sqlcdb.VideoCard {
	return &sqlcdb.VideoCard{
		ID:          pghelper.UUID(testCardID),
		OrganizerID: pghelper.UUID(testOrganizerID),
		Title:       "Grandma's 80th",
	}
}

func TestNotifyFinalVideoReady_PushesToOrganizer(t *testing.T) {
	push := &fakePusher{}
	n := &organizerNotifier{
		store: fakeNotifyStore{card: testCard(), user: &sqlcdb.User{
			ID:       pghelper.UUID(testOrganizerID),
			FcmToken: pghelper.TextVal("device-token"),
		}},
		push: push,
	}

	require.NoError(t, n.NotifyFinalVideoReady(context.Background(), testCardID))

	require.Len(t, push.sent, 1)
	got := push.sent[0]
	assert.Equal(t, "device-token", got.token)
	assert.Contains(t, got.body, "Grandma's 80th")
	assert.Equal(t, "app/detail/"+testCardID, got.data["screen"], "tapping must open the card")
}

func TestNotifyFinalVideoReady_SkipsWithoutDeviceToken(t *testing.T) {
	push := &fakePusher{}
	n := &organizerNotifier{
		store: fakeNotifyStore{card: testCard(), user: &sqlcdb.User{ID: pghelper.UUID(testOrganizerID)}},
		push:  push,
	}

	require.NoError(t, n.NotifyFinalVideoReady(context.Background(), testCardID))
	assert.Empty(t, push.sent)
}

func TestNotifyFinalVideoReady_ReportsLookupFailures(t *testing.T) {
	tests := []struct {
		name  string
		store fakeNotifyStore
	}{
		{name: "card lookup fails", store: fakeNotifyStore{cardErr: errors.New("db down")}},
		{name: "organizer missing", store: fakeNotifyStore{card: testCard()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			push := &fakePusher{}
			n := &organizerNotifier{store: tt.store, push: push}

			assert.Error(t, n.NotifyFinalVideoReady(context.Background(), testCardID))
			assert.Empty(t, push.sent)
		})
	}
}
