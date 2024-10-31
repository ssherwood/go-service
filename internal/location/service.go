package location

import (
	"context"
	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateLocation(ctx context.Context, location *Location) (*Location, error) {
	// TODO validity checks for required fields, etc
	return s.repo.CreateLocation(ctx, location)
}

func (s *Service) GetLocationById(ctx context.Context, locationId uuid.UUID) (*Location, error) {
	return s.repo.GetLocationById(ctx, locationId)
}
