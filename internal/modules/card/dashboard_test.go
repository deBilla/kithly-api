package card

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dashboardRepo struct {
	Repository
	owned, shared []*Entity
	counts        map[string]int
	askedFor      []string
}

func (r *dashboardRepo) ListByOrganizer(context.Context, string, int, int) ([]*Entity, error) {
	return r.owned, nil
}

func (r *dashboardRepo) ListShared(context.Context, string, int, int) ([]*Entity, error) {
	return r.shared, nil
}

func (r *dashboardRepo) CountIncludedContributions(_ context.Context, ids []string) (map[string]int, error) {
	r.askedFor = ids
	return r.counts, nil
}

func TestDashboard_FillsContributionCounts(t *testing.T) {
	repo := &dashboardRepo{
		owned:  []*Entity{{ID: "a"}, {ID: "b"}},
		shared: []*Entity{{ID: "c"}},
		counts: map[string]int{"a": 3, "c": 1}, // b has none
	}
	svc := &service{repo: repo}

	cards, err := svc.Dashboard(context.Background(), "u1", "u1@example.com", 1, 20)
	require.NoError(t, err)
	require.Len(t, cards, 3)
	assert.Equal(t, []string{"a", "b", "c"}, repo.askedFor, "one grouped lookup for owned and shared cards")

	got := map[string]int{}
	for _, c := range cards {
		got[c.ID] = c.ToOwnerView().ContributionCount
	}
	assert.Equal(t, map[string]int{"a": 3, "b": 0, "c": 1}, got)
	assert.True(t, cards[2].IsCollaborator)
}

func TestDashboard_NoEmailSkipsSharedButStillCounts(t *testing.T) {
	repo := &dashboardRepo{owned: []*Entity{{ID: "a"}}, counts: map[string]int{"a": 2}}
	cards, err := (&service{repo: repo}).Dashboard(context.Background(), "u1", "", 1, 20)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	assert.Equal(t, 2, cards[0].ContributionCount)
}
