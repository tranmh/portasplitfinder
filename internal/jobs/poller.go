// Package jobs implements the polling loop: it fetches each enabled source,
// persists snapshots, detects out->in-stock transitions, and notifies matching
// confirmed subscriptions (deduped).
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/geo"
	"github.com/tranmh/portasplitfinder/internal/notify"
	"github.com/tranmh/portasplitfinder/internal/source"
)

// Store is the persistence surface the poller needs. Defined as an interface so
// the poller can be unit-tested with fakes.
type Store interface {
	ListProducts(ctx context.Context) ([]domain.Product, error)
	UpsertStore(ctx context.Context, s domain.Store) (id string, err error)
	InsertSnapshot(ctx context.Context, snap domain.AvailabilitySnapshot) error
	LatestForKey(ctx context.Context, sourceKey, storeID, productID string) (domain.AvailabilitySnapshot, bool, error)
	UpdateSourceStatus(ctx context.Context, key string, checkedAt time.Time, status domain.LastStatus, lastErr *string) error
	ListConfirmedSubscriptions(ctx context.Context) ([]domain.Subscription, error)
	GetStore(ctx context.Context, id string) (domain.Store, bool, error)
	GetProductName(ctx context.Context, id string) (string, bool, error)
	// RecordNotification atomically claims a (sub, dedupeKey) pair; returns
	// inserted=false if it already existed (already notified).
	RecordNotification(ctx context.Context, subscriptionID, sourceKey, storeID, productID, dedupeKey string) (inserted bool, err error)
}

// Poller runs scrape + notify cycles.
type Poller struct {
	sources []source.Source
	store   Store
	mailer  notify.Mailer
	baseURL string
	log     *slog.Logger
	now     func() time.Time
}

// Config configures a Poller.
type Config struct {
	Sources       []source.Source
	Store         Store
	Mailer        notify.Mailer
	PublicBaseURL string
	Logger        *slog.Logger
	// Now overrides the clock (tests). Defaults to time.Now.
	Now func() time.Time
}

// New builds a Poller.
func New(cfg Config) *Poller {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Poller{
		sources: cfg.Sources,
		store:   cfg.Store,
		mailer:  cfg.Mailer,
		baseURL: cfg.PublicBaseURL,
		log:     log,
		now:     now,
	}
}

// transition records a listing that just became in-stock.
type transition struct {
	listing domain.Listing
}

// PollOnce runs a single scrape + notify cycle across all sources. It returns
// the number of emails sent and any aggregated error (per-source errors are
// recorded against the source and do not abort the cycle).
func (p *Poller) PollOnce(ctx context.Context) (mailsSent int, err error) {
	var transitions []transition

	// Load the tracked products once per cycle and pass them to every source so
	// real adapters know which items to look up and which ProductID to tag.
	products, perr := p.store.ListProducts(ctx)
	if perr != nil {
		return 0, fmt.Errorf("list products: %w", perr)
	}

	for _, src := range p.sources {
		ts, srcErr := p.pollSource(ctx, src, products)
		if srcErr != nil {
			p.log.Warn("source poll failed", "source", src.Key(), "err", srcErr)
			continue
		}
		transitions = append(transitions, ts...)
	}

	sent, notifyErr := p.notify(ctx, transitions)
	mailsSent += sent
	return mailsSent, notifyErr
}

// pollSource fetches one source, writes snapshots, updates status, and returns
// the listings that transitioned out->in-stock.
func (p *Poller) pollSource(ctx context.Context, src source.Source, products []domain.Product) ([]transition, error) {
	now := p.now()
	listings, stores, err := src.Fetch(ctx, products)
	if err != nil {
		if errors.Is(err, source.ErrNotImplemented) {
			// Stub source: record as ok-but-empty so the loop is tolerant.
			_ = p.store.UpdateSourceStatus(ctx, src.Key(), now, domain.StatusOK, nil)
			return nil, nil
		}
		msg := err.Error()
		_ = p.store.UpdateSourceStatus(ctx, src.Key(), now, domain.StatusError, &msg)
		return nil, err
	}

	// Upsert stores so snapshots can reference them.
	for _, st := range stores {
		if _, uerr := p.store.UpsertStore(ctx, st); uerr != nil {
			return nil, fmt.Errorf("upsert store %s: %w", st.Name, uerr)
		}
	}

	var transitions []transition
	for _, l := range listings {
		storeID := ""
		if l.StoreID != nil {
			storeID = *l.StoreID
		}

		prev, had, gerr := p.store.LatestForKey(ctx, l.SourceKey, storeID, l.ProductID)
		if gerr != nil {
			return nil, fmt.Errorf("latest for key: %w", gerr)
		}

		snap := domain.AvailabilitySnapshot{
			SourceKey:  l.SourceKey,
			StoreID:    l.StoreID,
			ProductID:  l.ProductID,
			InStock:    l.InStock,
			Quantity:   l.Quantity,
			PriceCents: l.PriceCents,
			Currency:   l.Currency,
			URL:        l.URL,
			CheckedAt:  now,
		}
		if ierr := p.store.InsertSnapshot(ctx, snap); ierr != nil {
			return nil, fmt.Errorf("insert snapshot: %w", ierr)
		}

		// Transition: previously known and out-of-stock, now in-stock.
		if l.InStock && had && !prev.InStock {
			ll := l
			ll.CheckedAt = now
			transitions = append(transitions, transition{listing: ll})
		}
	}

	_ = p.store.UpdateSourceStatus(ctx, src.Key(), now, domain.StatusOK, nil)
	return transitions, nil
}

// notify matches transitions to confirmed subscriptions and sends emails,
// deduping via the notifications table. Returns count of emails sent.
func (p *Poller) notify(ctx context.Context, transitions []transition) (int, error) {
	if len(transitions) == 0 {
		return 0, nil
	}
	subs, err := p.store.ListConfirmedSubscriptions(ctx)
	if err != nil {
		return 0, fmt.Errorf("list confirmed subs: %w", err)
	}

	sent := 0
	day := p.now().Format("2006-01-02")

	for _, tr := range transitions {
		l := tr.listing
		for _, sub := range subs {
			if !p.matches(ctx, sub, l) {
				continue
			}

			storeID := ""
			if l.StoreID != nil {
				storeID = *l.StoreID
			}
			dedupeKey := fmt.Sprintf("%s|%s|%s|%s", l.SourceKey, storeID, l.ProductID, day)

			inserted, rerr := p.store.RecordNotification(ctx, sub.ID, l.SourceKey, storeID, l.ProductID, dedupeKey)
			if rerr != nil {
				return sent, fmt.Errorf("record notification: %w", rerr)
			}
			if !inserted {
				continue // already notified today for this listing
			}

			if serr := p.sendAvailability(ctx, sub, l); serr != nil {
				p.log.Error("send availability email", "sub", sub.ID, "err", serr)
				continue
			}
			sent++
		}
	}
	return sent, nil
}

// matches reports whether a subscription should be notified about a listing.
func (p *Poller) matches(ctx context.Context, sub domain.Subscription, l domain.Listing) bool {
	// Product filter: a sub with a specific product only matches that product.
	if sub.ProductID != nil && *sub.ProductID != "" && *sub.ProductID != l.ProductID {
		return false
	}

	// Online listing => relevant to everyone (nationwide).
	if l.StoreID == nil || *l.StoreID == "" {
		return true
	}

	// Store listing => only if the store is within the sub's radius of its PLZ.
	center, ok := geo.Lookup(sub.PLZ)
	if !ok {
		return false
	}
	st, found, err := p.store.GetStore(ctx, *l.StoreID)
	if err != nil || !found {
		return false
	}
	return geo.WithinRadius(center, geo.Point{Lat: st.Lat, Lon: st.Lon}, float64(sub.RadiusKm))
}

func (p *Poller) sendAvailability(ctx context.Context, sub domain.Subscription, l domain.Listing) error {
	if p.mailer == nil {
		return nil
	}
	productName, _, _ := p.store.GetProductName(ctx, l.ProductID)
	if productName == "" {
		productName = "Portasplit"
	}

	where := l.SourceKey
	if l.StoreID != nil && *l.StoreID != "" {
		if st, ok, _ := p.store.GetStore(ctx, *l.StoreID); ok {
			where = fmt.Sprintf("%s, %s", st.Name, st.City)
		}
	}

	msg, err := notify.RenderAvailability(notify.AvailabilityData{
		Email:       sub.Email,
		ProductName: productName,
		Where:       where,
		PriceEUR:    notify.FormatPriceEUR(l.PriceCents),
		URL:         l.URL,
		UnsubURL:    notify.BuildURL(p.baseURL, notify.PathUnsub, sub.UnsubToken),
	})
	if err != nil {
		return err
	}
	return p.mailer.Send(msg)
}

// Run executes PollOnce immediately and then on every tick of interval until the
// context is cancelled. lastPoll is updated after each cycle.
func (p *Poller) Run(ctx context.Context, interval time.Duration, lastPoll *LastPoll) error {
	runOnce := func() {
		sent, err := p.PollOnce(ctx)
		if err != nil {
			p.log.Error("poll cycle error", "err", err)
		}
		if lastPoll != nil {
			lastPoll.Set(p.now())
		}
		p.log.Info("poll cycle complete", "mails_sent", sent)
	}

	runOnce()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			runOnce()
		}
	}
}
