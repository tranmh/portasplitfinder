// Package source defines the pluggable data-source interface and a registry that
// builds the enabled set from configuration. The mock source provides
// deterministic seed data that drives the whole app and the e2e tests; the real
// sources are stubs behind the same interface.
package source

import (
	"context"
	"errors"
	"fmt"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

// ErrNotImplemented is returned by stub sources that have no real scraper yet.
var ErrNotImplemented = errors.New("source: not implemented")

// Source is a pluggable retailer data source.
type Source interface {
	// Key is the stable source key ("obi", "amazon", "mock", ...).
	Key() string
	// Meta returns the source's catalogue metadata.
	Meta() domain.Source
	// Fetch returns the current listings and (for store sources) the stores they
	// belong to, for the given tracked products. Implementations may use the
	// products to resolve which items to look up and which ProductID to tag on
	// each listing; the mock source ignores them and uses its own catalogue.
	// Implementations must be safe to call concurrently with respect to other
	// sources.
	Fetch(ctx context.Context, products []domain.Product) ([]domain.Listing, []domain.Store, error)
}

// unblockerTemplate is the process-wide web-unblocker URL template (see
// WithUnblocker) applied to real adapters' HTTP clients. Empty = direct fetch.
// Set once at startup via SetUnblocker before Build.
var unblockerTemplate string

// SetUnblocker configures the web-unblocker URL template that real source
// adapters (obi, bauhaus) route their requests through. Call once at startup
// (e.g. from the poller) before building sources. An empty template means the
// adapters fetch directly and degrade gracefully when bot-blocked.
func SetUnblocker(template string) { unblockerTemplate = template }

// builder constructs a source instance.
type builder func() Source

// builders is the registry of all known source keys.
var builders = map[string]builder{
	"mock":       func() Source { return NewMock() },
	"obi":        func() Source { return newOBISource() },
	"bauhaus":    func() Source { return newBauhausSource() },
	"ebay":       func() Source { return &ebaySource{} },
	"amazon":     func() Source { return &amazonSource{} },
	"guenstiger": func() Source { return &guenstigerSource{} },
	"idealo":     func() Source { return &idealoSource{} },
}

// Build returns the enabled sources for the given keys, in the order given.
// Unknown keys produce an error.
func Build(keys []string) ([]Source, error) {
	out := make([]Source, 0, len(keys))
	for _, k := range keys {
		b, ok := builders[k]
		if !ok {
			return nil, fmt.Errorf("source: unknown source key %q", k)
		}
		out = append(out, b())
	}
	return out, nil
}

// Known reports whether key names a registered source.
func Known(key string) bool {
	_, ok := builders[key]
	return ok
}
