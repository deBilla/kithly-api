package musictheme

import (
	"context"
	"fmt"
)

// Service defines the business logic contract for music themes.
type Service interface {
	List(ctx context.Context) ([]*Entity, error)
	GetByID(ctx context.Context, id string) (*Entity, error)
	Create(ctx context.Context, req CreateRequest) (*Entity, error)
	Update(ctx context.Context, req UpdateRequest) (*Entity, error)
	Delete(ctx context.Context, id string) error
}

type service struct {
	repo Repository
}

// NewService constructs a Service backed by the given Repository.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context) ([]*Entity, error) {
	themes, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("musictheme.service.List: %w", err)
	}
	return themes, nil
}

func (s *service) GetByID(ctx context.Context, id string) (*Entity, error) {
	theme, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("musictheme.service.GetByID: %w", err)
	}
	return theme, nil
}

func (s *service) Create(ctx context.Context, req CreateRequest) (*Entity, error) {
	theme, err := s.repo.Create(ctx, CreateParams{
		Label:     req.Label,
		Emoji:     req.Emoji,
		AudioURL:  req.AudioURL,
		Category:  req.Category,
		IsActive:  req.IsActive,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("musictheme.service.Create: %w", err)
	}
	return theme, nil
}

func (s *service) Update(ctx context.Context, req UpdateRequest) (*Entity, error) {
	theme, err := s.repo.Update(ctx, req.ID, UpdateParams{
		Label:     req.Label,
		Emoji:     req.Emoji,
		AudioURL:  req.AudioURL,
		Category:  req.Category,
		IsActive:  req.IsActive,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("musictheme.service.Update: %w", err)
	}
	return theme, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("musictheme.service.Delete: %w", err)
	}
	return nil
}
