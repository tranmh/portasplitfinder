// Package httpapi implements the PortaSplitFinder HTTP API (chi router) exactly
// as described in docs/api-contract.md.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/tranmh/portasplitfinder/internal/domain"
	"github.com/tranmh/portasplitfinder/internal/geo"
	"github.com/tranmh/portasplitfinder/internal/notify"
)

// DataStore is the subset of the store layer the API needs. Defined here so
// handlers can be tested with fakes.
type DataStore interface {
	Ping(ctx context.Context) error
	ListProducts(ctx context.Context) ([]domain.Product, error)
	ListSources(ctx context.Context) ([]domain.Source, error)
	ListStores(ctx context.Context) ([]domain.Store, error)
	LatestSnapshots(ctx context.Context) ([]domain.AvailabilitySnapshot, error)
	CreateSubscription(ctx context.Context, s domain.Subscription) (string, error)
	ConfirmSubscription(ctx context.Context, token string) (email string, ok bool, err error)
	UnsubSubscription(ctx context.Context, token string) (ok bool, err error)
}

// Server holds API dependencies.
type Server struct {
	store         DataStore
	mailer        notify.Mailer
	publicBaseURL string
	webOrigin     string
	log           *slog.Logger

	// subMu guards subSeen: recent subscribe timestamps per (lowercased)
	// email, used to cap confirmation mails sent to one address.
	subMu   sync.Mutex
	subSeen map[string][]time.Time
}

// Per-address subscribe throttle: at most subLimit requests per subWindow.
const (
	subLimit  = 3
	subWindow = time.Hour
)

// allowSubscribe records a subscribe attempt for email and reports whether it
// is within the per-address limit.
func (s *Server) allowSubscribe(email string) bool {
	key := strings.ToLower(strings.TrimSpace(email))
	now := time.Now()
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if s.subSeen == nil {
		s.subSeen = map[string][]time.Time{}
	}
	recent := s.subSeen[key][:0]
	for _, t := range s.subSeen[key] {
		if now.Sub(t) < subWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) >= subLimit {
		s.subSeen[key] = recent
		return false
	}
	s.subSeen[key] = append(recent, now)
	return true
}

// Config configures the API server.
type Config struct {
	Store         DataStore
	Mailer        notify.Mailer
	PublicBaseURL string
	WebOrigin     string // CORS allowed origin; empty => "*"
	Logger        *slog.Logger
}

// New builds a Server.
func New(cfg Config) *Server {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		store:         cfg.Store,
		mailer:        cfg.Mailer,
		publicBaseURL: cfg.PublicBaseURL,
		webOrigin:     cfg.WebOrigin,
		log:           log,
	}
}

// Router builds the chi router with middleware and all routes.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(s.cors)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/products", s.handleProducts)
		r.Get("/sources", s.handleSources)
		r.Get("/availability", s.handleAvailability)
		r.Post("/subscriptions", s.handleSubscribe)
		r.Get("/subscriptions/confirm", s.handleConfirm)
		r.Get("/subscriptions/unsubscribe", s.handleUnsubscribe)
	})
	return r
}

// --- middleware ---

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"dur_ms", time.Since(start).Milliseconds(),
		)
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	origin := s.webOrigin
	if origin == "" {
		origin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "db": "error", "time": now})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "ok", "time": now})
}

func (s *Server) handleProducts(w http.ResponseWriter, r *http.Request) {
	products, err := s.store.ListProducts(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	if products == nil {
		products = []domain.Product{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": products})
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.store.ListSources(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	if sources == nil {
		sources = []domain.Source{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

func (s *Server) handleAvailability(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var center *Center
	plz := q.Get("plz")
	if plz != "" {
		if !validPLZ(plz) {
			writeError(w, http.StatusBadRequest, codeValidation, "plz must be 5 digits", fieldPtr("plz"))
			return
		}
		pt, ok := geo.Lookup(plz)
		if !ok {
			writeError(w, http.StatusBadRequest, codeValidation, "unknown plz", fieldPtr("plz"))
			return
		}
		center = &Center{PLZ: plz, Lat: pt.Lat, Lon: pt.Lon}
	}

	radiusKm := defaultRadiusKm
	if rs := q.Get("radius"); rs != "" {
		n, err := strconv.Atoi(rs)
		if err != nil || n < 1 || n > maxRadiusKm {
			writeError(w, http.StatusBadRequest, codeValidation, "radius must be an integer in 1..500", fieldPtr("radius"))
			return
		}
		radiusKm = n
	}

	productID := q.Get("productId")

	snapshots, err := s.store.LatestSnapshots(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	stores, err := s.store.ListStores(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	sources, err := s.store.ListSources(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}

	resp := assembleAvailability(availabilityInputs{snapshots: snapshots, stores: stores, sources: sources}, center, radiusKm, productID)
	if resp.Stores == nil {
		resp.Stores = []StoreAvailability{}
	}
	if resp.Online == nil {
		resp.Online = []OnlineAvailability{}
	}
	writeJSON(w, http.StatusOK, resp)
}

type subscribeRequest struct {
	Email     string  `json:"email"`
	PLZ       string  `json:"plz"`
	RadiusKm  *int    `json:"radiusKm"`
	ProductID *string `json:"productId"`
}

// maxSubscribeBody caps the subscribe request body; a valid one is well under 1 KiB.
const maxSubscribeBody = 64 << 10

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	var req subscribeRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxSubscribeBody)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, codeValidation, "request body too large", nil)
			return
		}
		writeError(w, http.StatusBadRequest, codeValidation, "invalid JSON body", nil)
		return
	}
	if !validEmail(req.Email) {
		writeError(w, http.StatusBadRequest, codeValidation, "invalid email", fieldPtr("email"))
		return
	}
	if !validPLZ(req.PLZ) {
		writeError(w, http.StatusBadRequest, codeValidation, "plz must be 5 digits", fieldPtr("plz"))
		return
	}
	radius := defaultRadiusKm
	if req.RadiusKm != nil {
		radius = *req.RadiusKm
	}
	if !validRadius(radius) {
		writeError(w, http.StatusBadRequest, codeValidation, "radiusKm must be in 1..500", fieldPtr("radiusKm"))
		return
	}
	if !s.allowSubscribe(req.Email) {
		writeError(w, http.StatusTooManyRequests, codeRateLimit, "too many subscription requests for this email", nil)
		return
	}

	confirmTok, err := notify.NewToken()
	if err != nil {
		s.internal(w, err)
		return
	}
	unsubTok, err := notify.NewToken()
	if err != nil {
		s.internal(w, err)
		return
	}

	sub := domain.Subscription{
		Email:        req.Email,
		PLZ:          req.PLZ,
		RadiusKm:     radius,
		ProductID:    req.ProductID,
		ConfirmToken: confirmTok,
		UnsubToken:   unsubTok,
	}
	if _, err := s.store.CreateSubscription(r.Context(), sub); err != nil {
		s.internal(w, err)
		return
	}

	// Send the double opt-in confirmation email. Failure to send is logged but
	// the response is still 202 (avoids enumeration / transient SMTP issues).
	msg, err := notify.RenderConfirm(notify.ConfirmData{
		Email:      req.Email,
		ConfirmURL: notify.BuildURL(s.publicBaseURL, notify.PathConfirm, confirmTok),
		UnsubURL:   notify.BuildURL(s.publicBaseURL, notify.PathUnsub, unsubTok),
	})
	if err != nil {
		s.log.Error("render confirm email", "err", err)
	} else if s.mailer != nil {
		if err := s.mailer.Send(msg); err != nil {
			s.log.Error("send confirm email", "err", err)
		}
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "pending_confirmation"})
}

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, codeInvalidTok, "missing token", fieldPtr("token"))
		return
	}
	email, ok, err := s.store.ConfirmSubscription(r.Context(), token)
	if err != nil {
		s.internal(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, codeInvalidTok, "unknown or expired token", fieldPtr("token"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "confirmed", "email": email})
}

func (s *Server) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, codeInvalidTok, "missing token", fieldPtr("token"))
		return
	}
	ok, err := s.store.UnsubSubscription(r.Context(), token)
	if err != nil {
		s.internal(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, codeInvalidTok, "unknown token", fieldPtr("token"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unsubscribed"})
}

func (s *Server) internal(w http.ResponseWriter, err error) {
	s.log.Error("internal error", "err", err)
	writeError(w, http.StatusInternalServerError, codeInternal, "internal server error", nil)
}
