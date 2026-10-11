package videooverlay

import (
	"context"
	"fmt"
)

// Service defines the business logic contract for video overlays.
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
		Label:        req.Label,
		ResourceURL:  req.ResourceURL,
		ThumbnailURL: req.ThumbnailURL,
		OverlayType:  req.OverlayType,
		Category:     req.Category,
		Config:       req.Config,
		IsActive:     req.IsActive,
		SortOrder:    req.SortOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("videooverlay.service.Create: %w", err)
	}
	return o, nil
}

func (s *service) Update(ctx context.Context, req UpdateRequest) (*Entity, error) {
	o, err := s.repo.Update(ctx, req.ID, UpdateParams{
		Label:        req.Label,
		ResourceURL:  req.ResourceURL,
		ThumbnailURL: req.ThumbnailURL,
		OverlayType:  req.OverlayType,
		Category:     req.Category,
		Config:       req.Config,
		IsActive:     req.IsActive,
		SortOrder:    req.SortOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("videooverlay.service.Update: %w", err)
	}
	return o, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
