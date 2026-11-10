// Package ws exposes rooms over HTTP: a small REST API to create and look up
// rooms, and the WebSocket each player's browser keeps open.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"pokerroom/internal/protocol"
	"pokerroom/internal/room"
)

// Server serves the room API.
type Server struct {
	Rooms *room.Manager
	// OriginPatterns are extra allowed browser origins (host patterns, e.g.
	// "localhost:5173"). Same-origin requests are always allowed.
	OriginPatterns []string
	Log            *slog.Logger
	// TrustProxy reads the client IP from Fly-Client-IP (set it only behind
	// Fly.io's proxy, which overwrites the header).
	TrustProxy bool
	// RoomsPerMinute limits room creation per IP (default 10).
	RoomsPerMinute float64

	createOnce    sync.Once
	createLimiter *ipLimiter
}

const (
	helloTimeout = 10 * time.Second
	writeTimeout = 10 * time.Second
	maxMessage   = 8 << 10
	// pingEvery keeps connections alive through proxies and notices phones
	// that went to sleep, so their seat shows as offline.
	pingEvery = 25 * time.Second

	// Per connection: a short burst is fine; sustained flooding is dropped
	// and, if it keeps up, disconnected.
	msgRate            = 10.0 // messages per second
	msgBurst           = 30.0
	maxDropped         = 200
	defaultRoomsPerMin = 10.0 // room creations per IP
)

// Register adds the routes to mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/rooms", s.createRoom)
	mux.HandleFunc("GET /api/rooms/{code}", s.getRoom)
	mux.HandleFunc("GET /api/rooms/{code}/history", s.getHistory)
	mux.HandleFunc("GET /ws/{code}", s.serveWS)
}

type createReq struct {
	Name string `json:"name"`
}

type createResp struct {
	Code  string `json:"code"`
	Token string `json:"token"`
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	s.createOnce.Do(func() {
		n := s.RoomsPerMinute
		if n <= 0 {
			n = defaultRoomsPerMin
		}
		s.createLimiter = newIPLimiter(n, n)
	})
	if !s.createLimiter.allow(s.clientIP(r), time.Now()) {
		httpError(w, http.StatusTooManyRequests, "too many new rooms, try again in a minute")
		return
	}
	var req createReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}
	code, token, err := s.Rooms.Create(req.Name)
	if errors.Is(err, room.ErrTooManyRooms) {
		httpError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, createResp{Code: code, Token: token})
}

func (s *Server) getRoom(w http.ResponseWriter, r *http.Request) {
	a := s.Rooms.Get(r.PathValue("code"))
	if a == nil {
		httpError(w, http.StatusNotFound, "room not found")
		return
	}
	writeJSON(w, http.StatusOK, a.Summary())
}

// getHistory downloads a finished game's every hand. The player proves they
// were in the room with their session token (Authorization: Bearer), kept
// out of the URL so it doesn't end up in logs or browser history.
func (s *Server) getHistory(w http.ResponseWriter, r *http.Request) {
	a := s.Rooms.Get(r.PathValue("code"))
	if a == nil {
		httpError(w, http.StatusNotFound, "room not found")
		return
	}
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	export, err := a.HandHistory(token)
	switch {
	case errors.Is(err, room.ErrNotYourRoom):
		httpError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, room.ErrNotOver):
		httpError(w, http.StatusConflict, err.Error())
	case errors.Is(err, room.ErrNoArchive), errors.Is(err, room.ErrRoomClosed):
		httpError(w, http.StatusNotFound, err.Error())
	case err != nil:
		s.Log.Error("hand history", "room", r.PathValue("code"), "err", err)
		httpError(w, http.StatusInternalServerError, "couldn't read the hand history")
	default:
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="poker-room-%s.json"`, export.Room))
		writeJSON(w, http.StatusOK, export)
	}
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	a := s.Rooms.Get(r.PathValue("code"))
	if a == nil {
		httpError(w, http.StatusNotFound, "room not found")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.OriginPatterns})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(maxMessage)
	ctx := r.Context()

	// The first message identifies the player.
	var hello protocol.ClientMsg
	helloCtx, cancel := context.WithTimeout(ctx, helloTimeout)
	err = wsjson.Read(helloCtx, conn, &hello)
	cancel()
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "expected join or resume")
		return
	}
	c := room.NewClient()
	if err := a.Connect(c, hello); err != nil {
		typ := protocol.MsgError
		if errors.Is(err, room.ErrKicked) {
			typ = protocol.MsgRemoved
		}
		_ = wsjson.Write(ctx, conn, protocol.ServerMsg{Type: typ, Error: err.Error()})
		_ = conn.Close(websocket.StatusPolicyViolation, "could not join")
		return
	}
	defer a.Disconnect(c)

	go s.writeLoop(ctx, conn, c)

	var limit bucket
	dropped := 0
	for {
		var m protocol.ClientMsg
		if err := wsjson.Read(ctx, conn, &m); err != nil {
			var syntaxErr *json.SyntaxError
			if errors.As(err, &syntaxErr) {
				_ = conn.Close(websocket.StatusUnsupportedData, "invalid JSON")
			}
			return
		}
		if m.Type == protocol.MsgJoin || m.Type == protocol.MsgResume {
			continue // already identified
		}
		if !limit.allow(time.Now(), msgRate, msgBurst) {
			if dropped++; dropped > maxDropped {
				_ = conn.Close(websocket.StatusPolicyViolation, "too many messages")
				return
			}
			continue
		}
		a.Submit(c, m)
	}
}

func (s *Server) writeLoop(ctx context.Context, conn *websocket.Conn, c *room.Client) {
	ping := time.NewTicker(pingEvery)
	defer ping.Stop()
	for {
		select {
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				_ = conn.CloseNow() // the reader then fails and the seat goes offline
				return
			}
		case b := <-c.Send():
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Write(wctx, websocket.MessageText, b)
			cancel()
			if err != nil {
				_ = conn.CloseNow()
				return
			}
		case <-c.Done():
			_ = conn.Close(websocket.StatusTryAgainLater, "connection too slow")
			return
		case <-ctx.Done():
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
