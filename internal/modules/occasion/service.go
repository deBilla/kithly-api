package occasion

import (
	"context"
	"fmt"
)

// Service defines the business logic contract for occasions.
type Service interface {
	List(ctx context.Context) ([]*Entity, error)
	GetByID(ctx context.Context, id string) (*Entity, error)
	Create(ctx context.Context, req CreateRequest) (*Entity, error)
	Update(ctx context.Context, req UpdateRequest) (*Entity, error)
	Delete(ctx context.Context, id string) error
}

type service struct{ repo Repository }

// NewService constructs a Service backed by the given Repository.
func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List(ctx context.Context) ([]*Entity, error) {
	return s.repo.List(ctx)
}

func (s *service) GetByID(ctx context.Context, id string) (*Entity, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) Create(ctx context.Context, req CreateRequest) (*Entity, error) {
	o, err := s.repo.Create(ctx, CreateParams{
		ID:              req.ID,
		Label:           req.Label,
		Emoji:           req.Emoji,
		ContributorTips: req.ContributorTips,
		MessageTemplate: req.MessageTemplate,
		SortOrder:       req.SortOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("occasion.service.Create: %w", err)
	}
	return o, nil
}

func (s *service) Update(ctx context.Context, req UpdateRequest) (*Entity, error) {
	o, err := s.repo.Update(ctx, req.ID, UpdateParams{
		Label:           req.Label,
		Emoji:           req.Emoji,
		ContributorTips: req.ContributorTips,
		MessageTemplate: req.MessageTemplate,
		SortOrder:       req.SortOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("occasion.service.Update: %w", err)
	}
	return o, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
