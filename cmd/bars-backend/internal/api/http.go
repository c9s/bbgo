// Package api exposes the Feed's klines and tick/volume bars over a small
// REST + WebSocket HTTP API, independent of bbgo's own pkg/server.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/types"

	"github.com/c9s/bbgo/cmd/bars-backend/internal/config"
	"github.com/c9s/bbgo/cmd/bars-backend/internal/feed"
)

// Server wires the Feed into HTTP handlers.
type Server struct {
	cfg      *config.Config
	feed     *feed.Feed
	upgrader websocket.Upgrader
}

// NewRouter builds the complete HTTP handler for the bars-backend API,
// including CORS for cfg.CORSOrigin.
func NewRouter(cfg *config.Config, f *feed.Feed) http.Handler {
	s := &Server{
		cfg:  cfg,
		feed: f,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "" || origin == cfg.CORSOrigin
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/symbols", s.handleSymbols)
	mux.HandleFunc("/api/klines", s.handleKLinesREST)
	mux.HandleFunc("/api/bars", s.handleBarsREST)
	mux.HandleFunc("/ws/klines", s.handleKLinesWS)
	mux.HandleFunc("/ws/bars", s.handleBarsWS)

	return withCORS(cfg.CORSOrigin, mux)
}

func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleSymbols(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.cfg.Symbols)
}

func (s *Server) handleKLinesREST(w http.ResponseWriter, r *http.Request) {
	symbol, interval, err := s.parseKLineQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, s.feed.KLines.Snapshot(symbol, interval))
}

func (s *Server) handleBarsREST(w http.ResponseWriter, r *http.Request) {
	key, err := s.parseBarQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	kLines, err := s.feed.Bars.Snapshot(r.Context(), key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(w, kLines)
}

func (s *Server) handleKLinesWS(w http.ResponseWriter, r *http.Request) {
	symbol, interval, err := s.parseKLineQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	snapshot := s.feed.KLines.Snapshot(symbol, interval)
	ch, cancel := s.feed.KLines.Subscribe(symbol, interval)
	defer cancel()

	s.serveWS(w, r, snapshot, ch)
}

func (s *Server) handleBarsWS(w http.ResponseWriter, r *http.Request) {
	key, err := s.parseBarQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	snapshot, err := s.feed.Bars.Snapshot(r.Context(), key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	ch, cancel, err := s.feed.Bars.Subscribe(r.Context(), key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer cancel()

	s.serveWS(w, r, snapshot, ch)
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request, snapshot []types.KLine, live <-chan types.KLine) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	defer conn.Close()

	for i := range snapshot {
		if err := conn.WriteJSON(wsMessage{Type: "snapshot", Data: &snapshot[i]}); err != nil {
			return
		}
	}
	if err := conn.WriteJSON(wsMessage{Type: "snapshotEnd"}); err != nil {
		return
	}

	// Drain and discard client messages; this is a push-only feed, but we still
	// need to read so gorilla notices the client disconnecting.
	clientClosed := make(chan struct{})
	go func() {
		defer close(clientClosed)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-clientClosed:
			return
		case k, ok := <-live:
			if !ok {
				return
			}
			if err := conn.WriteJSON(wsMessage{Type: "update", Data: &k}); err != nil {
				return
			}
		}
	}
}

// wsMessage envelopes every WS push so the client can tell a historical
// snapshot entry, the end of the snapshot, and a live update apart without
// guessing from the kline's own fields.
type wsMessage struct {
	Type string       `json:"type"` // "snapshot" | "snapshotEnd" | "update"
	Data *types.KLine `json:"data,omitempty"`
}

func (s *Server) parseKLineQuery(r *http.Request) (string, types.Interval, error) {
	symbol := r.URL.Query().Get("symbol")
	interval := types.Interval(r.URL.Query().Get("interval"))

	if !s.cfg.HasSymbol(symbol) {
		return "", "", fmt.Errorf("unknown symbol %q", symbol)
	}
	if !s.cfg.HasInterval(interval) {
		return "", "", fmt.Errorf("unknown interval %q", interval)
	}

	return symbol, interval, nil
}

func (s *Server) parseBarQuery(r *http.Request) (feed.BarKey, error) {
	symbol := r.URL.Query().Get("symbol")
	barType := feed.BarType(r.URL.Query().Get("type"))
	param := r.URL.Query().Get("param")

	if !s.cfg.HasSymbol(symbol) {
		return feed.BarKey{}, fmt.Errorf("unknown symbol %q", symbol)
	}

	return feed.ParseBarKey(symbol, barType, param)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}
