package room

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"pokerroom/internal/protocol"
)

// drain returns the messages already queued for c. Call it after an
// Inspect, which waits until everything submitted before it has run.
func drain(t *testing.T, c *Client) []protocol.ServerMsg {
	t.Helper()
	var out []protocol.ServerMsg
	for {
		select {
		case b := <-c.Send():
			var m protocol.ServerMsg
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			out = append(out, m)
		default:
			return out
		}
	}
}

func lastError(t *testing.T, c *Client) string {
	t.Helper()
	msgs := drain(t, c)
	for _, m := range slices.Backward(msgs) {
		if m.Type == protocol.MsgError {
			return m.Error
		}
	}
	return ""
}

// rematchSetup makes a room of Ann (host), Bob and Cy with non-default
// settings, plays nothing, and connects Ann and Bob. Cy stays offline.
func rematchSetup(t *testing.T, m *Manager) (a *Actor, ann, bob *Client, ids map[string]string) {
	t.Helper()
	code, token, err := m.Create("Ann")
	if err != nil {
		t.Fatal(err)
	}
	a = m.Get(code)
	ids = map[string]string{}
	var bobToken string
	a.Inspect(func(r *Room) {
		r.settings.StartingStack = 5000
		r.settings.BigBlind, r.settings.SmallBlind = 50, 25
		r.settings.Variants = []string{"holdem", "omaha"}
		r.settings.Hints = false
		for _, n := range []string{"Bob", "Cy"} {
			p, err := r.Join(n)
			if err != nil {
				t.Fatal(err)
			}
			if n == "Bob" {
				bobToken = p.Token
			}
		}
		for _, p := range r.players {
			ids[p.Name] = p.ID
		}
	})
	ann, bob = NewClient(), NewClient()
	for c, tok := range map[*Client]string{ann: token, bob: bobToken} {
		if err := a.Connect(c, protocol.ClientMsg{Type: protocol.MsgResume, Token: tok}); err != nil {
			t.Fatal(err)
		}
	}
	return a, ann, bob, ids
}

func TestRematch(t *testing.T) {
	m := newTestManager()
	a, ann, bob, ids := rematchSetup(t, m)
	rematch := protocol.ClientMsg{Type: protocol.MsgRematch}

	// Only after the game has ended.
	a.Submit(ann, rematch)
	a.Inspect(func(*Room) {})
	if e := lastError(t, ann); !strings.Contains(e, "end the game") {
		t.Fatalf("rematch in the lobby: error %q", e)
	}
	a.Submit(ann, protocol.ClientMsg{Type: protocol.MsgStart})
	a.Submit(ann, protocol.ClientMsg{Type: protocol.MsgEndGame})
	a.Inspect(func(r *Room) {
		if r.phase != protocol.PhaseEnded {
			t.Fatalf("phase %q, want ended", r.phase)
		}
	})

	// Only the host.
	a.Submit(bob, rematch)
	a.Inspect(func(*Room) {})
	if e := lastError(t, bob); e != ErrNotHost.Error() {
		t.Fatalf("rematch by Bob: error %q", e)
	}

	a.Submit(ann, rematch)
	var old Snapshot
	views := map[string]protocol.State{}
	a.Inspect(func(r *Room) {
		old = r.Snapshot()
		for name, id := range ids {
			views[name] = r.View(id)
		}
	})
	if e := lastError(t, ann); e != "" {
		t.Fatalf("rematch failed: %s", e)
	}

	// Everyone, offline Cy included, is pointed at the same new room, with
	// their own token: nobody can see anyone else's.
	next := views["Ann"].NextRoom
	if next == nil || next.Code == "" || next.Code == old.Code {
		t.Fatalf("next room = %+v", next)
	}
	tokens := map[string]bool{}
	for name, v := range views {
		if v.NextRoom == nil || v.NextRoom.Code != next.Code || v.NextRoom.Token == "" {
			t.Fatalf("%s's view: next room %+v", name, v.NextRoom)
		}
		tokens[v.NextRoom.Token] = true
		b, _ := json.Marshal(v)
		for other, ov := range views {
			if other != name && strings.Contains(string(b), ov.NextRoom.Token) {
				t.Fatalf("%s's view contains %s's token", name, other)
			}
		}
	}
	if len(tokens) != 3 {
		t.Fatal("tokens must differ per player")
	}

	// The new room: lobby, same settings, same players, same host, fresh
	// stacks. Each token resumes the right player.
	na := m.Get(next.Code)
	if na == nil {
		t.Fatal("new room not found")
	}
	na.Inspect(func(r *Room) {
		if r.phase != protocol.PhaseLobby {
			t.Fatalf("new room phase %q", r.phase)
		}
		got, want := r.settings, old.Settings
		if got.StartingStack != want.StartingStack || got.BigBlind != want.BigBlind || got.Hints != want.Hints || !slices.Equal(got.Variants, want.Variants) {
			t.Fatalf("settings %+v, want %+v", got, want)
		}
		var names []string
		for _, p := range r.players {
			names = append(names, p.Name)
			if p.Stack != 5000 || p.Debt != 0 {
				t.Fatalf("%s starts with stack %d debt %d", p.Name, p.Stack, p.Debt)
			}
		}
		if !slices.Equal(names, []string{"Ann", "Bob", "Cy"}) {
			t.Fatalf("players %v", names)
		}
		if host := r.player(r.hostID); host.Name != "Ann" {
			t.Fatalf("host is %s", host.Name)
		}
		for name, v := range views {
			if p := r.PlayerByToken(v.NextRoom.Token); p == nil || p.Name != name {
				t.Fatalf("%s's token resumes %v", name, p)
			}
		}
	})

	// Once is enough.
	a.Submit(ann, rematch)
	a.Inspect(func(*Room) {})
	if e := lastError(t, ann); !strings.Contains(e, "already") {
		t.Fatalf("second rematch: error %q", e)
	}
}

func TestRematchWhenServerIsFull(t *testing.T) {
	m := newTestManager()
	a, ann, _, _ := rematchSetup(t, m)
	a.Submit(ann, protocol.ClientMsg{Type: protocol.MsgStart})
	a.Submit(ann, protocol.ClientMsg{Type: protocol.MsgEndGame})
	m.MaxRooms = 1
	a.Submit(ann, protocol.ClientMsg{Type: protocol.MsgRematch})
	a.Inspect(func(r *Room) {
		if r.nextCode != "" {
			t.Fatal("no new room should be recorded")
		}
	})
	if e := lastError(t, ann); e != ErrTooManyRooms.Error() {
		t.Fatalf("error %q", e)
	}
	// It can be retried once there's room.
	m.MaxRooms = 10
	a.Submit(ann, protocol.ClientMsg{Type: protocol.MsgRematch})
	a.Inspect(func(r *Room) {
		if r.nextCode == "" {
			t.Fatal("retry should work")
		}
	})
}

func TestRematchSurvivesRestart(t *testing.T) {
	m := newTestManager()
	a, ann, _, ids := rematchSetup(t, m)
	for _, typ := range []string{protocol.MsgStart, protocol.MsgEndGame, protocol.MsgRematch} {
		a.Submit(ann, protocol.ClientMsg{Type: typ})
	}
	a.Inspect(func(r *Room) {
		back := roundTrip(t, r)
		for name, id := range ids {
			if got, want := back.View(id).NextRoom, r.View(id).NextRoom; got == nil || *got != *want {
				t.Fatalf("%s: next room after restore %+v, want %+v", name, got, want)
			}
		}
	})
}
