package ws

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pokerroom/internal/engine"
	"pokerroom/internal/history"
	"pokerroom/internal/httpapi"
	"pokerroom/internal/protocol"
	"pokerroom/internal/room"
)

func TestHistoryDownload(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := room.NewManager(room.Options{Variants: map[string]engine.Variant{"holdem": engine.Holdem}}, log)
	store, err := history.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.KeepHistory(store)
	s := &Server{Rooms: m, Log: log}
	srv := httptest.NewServer(httpapi.NewHandler(httpapi.Config{Routes: s.Register}, log))
	t.Cleanup(srv.Close)

	code, token, err := m.Create("Ann")
	if err != nil {
		t.Fatal(err)
	}
	a := m.Get(code)
	a.Inspect(func(r *room.Room) {
		bob, _ := r.Join("Bob")
		ann := r.PlayerByToken(token)
		must := func(p *room.Player, msg protocol.ClientMsg) {
			if err := r.Handle(p.ID, msg); err != nil {
				t.Fatal(err)
			}
		}
		must(ann, protocol.ClientMsg{Type: protocol.MsgStart})
		// Fold every hand until two are done.
		for n := 0; n < 2; {
			v := r.View(ann.ID)
			switch {
			case v.Stage == protocol.StageResult:
				must(ann, protocol.ClientMsg{Type: protocol.MsgNextHand})
				n++
			case v.Hand != nil && v.Hand.ToAct == ann.ID:
				must(ann, protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "fold"}})
			default:
				must(bob, protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "fold"}})
			}
		}
	})

	get := func(auth string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/rooms/"+code+"/history", nil)
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		return resp
	}

	if r := get(""); r.StatusCode != http.StatusForbidden {
		t.Fatalf("no token: %d", r.StatusCode)
	}
	if r := get(token); r.StatusCode != http.StatusConflict {
		t.Fatalf("during the game: %d", r.StatusCode)
	}
	a.Inspect(func(r *room.Room) {
		// Ending during a hand waits for it: fold it out.
		_ = r.Handle(r.PlayerByToken(token).ID, protocol.ClientMsg{Type: protocol.MsgEndGame})
		for v := r.View(""); v.Phase != protocol.PhaseEnded; v = r.View("") {
			_ = r.Handle(v.Hand.ToAct, protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "fold"}})
		}
	})
	r := get(token)
	if r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("after the game: %d %s", r.StatusCode, b)
	}
	if cd := r.Header.Get("Content-Disposition"); !strings.Contains(cd, "poker-room-"+code+".json") {
		t.Fatalf("Content-Disposition %q", cd)
	}
	var export room.Export
	if err := json.NewDecoder(r.Body).Decode(&export); err != nil {
		t.Fatal(err)
	}
	if export.Room != code || len(export.Hands) != 3 || export.Hands[2].Number != 3 {
		t.Fatalf("export: %s, %d hands", export.Room, len(export.Hands))
	}
	if resp := get("someone-elses-token"); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong token: %d", resp.StatusCode)
	}
}
