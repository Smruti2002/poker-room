// Package ws exposes rooms over HTTP: a small REST API to create and look up
// rooms, and the WebSocket each player's browser keeps open.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
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
}

const (
	helloTimeout = 10 * time.Second
	writeTimeout = 10 * time.Second
	maxMessage   = 8 << 10
)

// Register adds the routes to mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/rooms", s.createRoom)
	mux.HandleFunc("GET /api/rooms/{code}", s.getRoom)
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
	var req createReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}
	code, token, err := s.Rooms.Create(req.Name)
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
		_ = wsjson.Write(ctx, conn, protocol.ServerMsg{Type: protocol.MsgError, Error: err.Error()})
		_ = conn.Close(websocket.StatusPolicyViolation, "could not join")
		return
	}
	defer a.Disconnect(c)

	go s.writeLoop(ctx, conn, c)

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
		a.Submit(c, m)
	}
}

func (s *Server) writeLoop(ctx context.Context, conn *websocket.Conn, c *room.Client) {
	for {
		select {
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
