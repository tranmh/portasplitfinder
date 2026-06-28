package source

import (
	"context"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// The stubs below implement the Source interface but have no real scraper/API
// integration yet. They return empty data and ErrNotImplemented so the registry
// and poller can treat them uniformly. Replace each Fetch with a real
// implementation when the scraper/API is available.

type obiSource struct{}

func (s *obiSource) Key() string { return "obi" }
func (s *obiSource) Meta() domain.Source {
	return domain.Source{Key: "obi", Name: "OBI", Type: domain.SourceStore, BaseURL: "https://www.obi.de", LastStatus: domain.StatusNever}
}
func (s *obiSource) Fetch(_ context.Context) ([]domain.Listing, []domain.Store, error) {
	// TODO real scraper/API: fetch OBI store stock for the tracked products.
	return nil, nil, ErrNotImplemented
}

type ebaySource struct{}

func (s *ebaySource) Key() string { return "ebay" }
func (s *ebaySource) Meta() domain.Source {
	return domain.Source{Key: "ebay", Name: "eBay", Type: domain.SourceOnline, BaseURL: "https://www.ebay.de", LastStatus: domain.StatusNever}
}
func (s *ebaySource) Fetch(_ context.Context) ([]domain.Listing, []domain.Store, error) {
	// TODO real scraper/API: query the eBay Browse API for the tracked products.
	return nil, nil, ErrNotImplemented
}

type amazonSource struct{}

func (s *amazonSource) Key() string { return "amazon" }
func (s *amazonSource) Meta() domain.Source {
	return domain.Source{Key: "amazon", Name: "Amazon", Type: domain.SourceOnline, BaseURL: "https://www.amazon.de", LastStatus: domain.StatusNever}
}
func (s *amazonSource) Fetch(_ context.Context) ([]domain.Listing, []domain.Store, error) {
	// TODO real scraper/API: query the Amazon Product Advertising API.
	return nil, nil, ErrNotImplemented
}

type guenstigerSource struct{}

func (s *guenstigerSource) Key() string { return "guenstiger" }
func (s *guenstigerSource) Meta() domain.Source {
	return domain.Source{Key: "guenstiger", Name: "guenstiger.de", Type: domain.SourceOnline, BaseURL: "https://www.guenstiger.de", LastStatus: domain.StatusNever}
}
func (s *guenstigerSource) Fetch(_ context.Context) ([]domain.Listing, []domain.Store, error) {
	// TODO real scraper/API: scrape guenstiger.de price/availability.
	return nil, nil, ErrNotImplemented
}

type idealoSource struct{}

func (s *idealoSource) Key() string { return "idealo" }
func (s *idealoSource) Meta() domain.Source {
	return domain.Source{Key: "idealo", Name: "idealo", Type: domain.SourceOnline, BaseURL: "https://www.idealo.de", LastStatus: domain.StatusNever}
}
func (s *idealoSource) Fetch(_ context.Context) ([]domain.Listing, []domain.Store, error) {
	// TODO real scraper/API: scrape idealo.de price/availability.
	return nil, nil, ErrNotImplemented
}
