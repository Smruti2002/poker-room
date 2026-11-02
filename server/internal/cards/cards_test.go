package cards

import (
	"encoding/json"
	"testing"
)

func TestNewRoundTrip(t *testing.T) {
	for r := Two; r <= Ace; r++ {
		for s := Clubs; s <= Spades; s++ {
			c := New(r, s)
			if !c.Valid() || c.Rank() != r || c.Suit() != s {
				t.Fatalf("New(%d,%d) = %d -> rank %d suit %d", r, s, c, c.Rank(), c.Suit())
			}
		}
	}
	if Card(52).Valid() {
		t.Fatal("card 52 should be invalid")
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Card
	}{
		{"As", New(Ace, Spades)},
		{"as", New(Ace, Spades)},
		{"Td", New(Ten, Diamonds)},
		{"10d", New(Ten, Diamonds)},
		{"2c", New(Two, Clubs)},
		{"K♥", New(King, Hearts)},
		{"Q♧", New(Queen, Clubs)},
		{" 9H ", New(Nine, Hearts)},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("Parse(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
	}

	for _, bad := range []string{"", "A", "1s", "Ax", "Zs", "As2", "10"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
}

func TestStringAndPretty(t *testing.T) {
	c := New(Ten, Hearts)
	if c.String() != "Th" || c.Pretty() != "T♥" {
		t.Fatalf("got %q / %q", c.String(), c.Pretty())
	}
	if Card(60).String() != "??" || Card(60).Pretty() != "??" {
		t.Fatal("invalid card should format as ??")
	}
	for i := range Card(NumCards) {
		back, err := Parse(i.String())
		if err != nil || back != i {
			t.Fatalf("round trip %v failed: %v %v", i, back, err)
		}
		back, err = Parse(i.Pretty())
		if err != nil || back != i {
			t.Fatalf("pretty round trip %v failed: %v %v", i, back, err)
		}
	}
}

func TestParseList(t *testing.T) {
	got, err := ParseList("As Kd,10h\tQ♠")
	if err != nil {
		t.Fatal(err)
	}
	want := []Card{New(Ace, Spades), New(King, Diamonds), New(Ten, Hearts), New(Queen, Spades)}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if _, err := ParseList("As Xx"); err == nil {
		t.Fatal("expected error")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("MustParseList should panic on bad input")
		}
	}()
	MustParseList("nope")
}

func TestJSON(t *testing.T) {
	in := []Card{New(Ace, Spades), New(Ten, Clubs)}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `["As","Tc"]` {
		t.Fatalf("json = %s", b)
	}
	var out []Card
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out[0] != in[0] || out[1] != in[1] {
		t.Fatalf("round trip = %v", out)
	}
	if err := json.Unmarshal([]byte(`["Zz"]`), &out); err == nil {
		t.Fatal("expected error for bad card")
	}
	if _, err := json.Marshal(Card(99)); err == nil {
		t.Fatal("expected error marshalling invalid card")
	}
}

func TestSet(t *testing.T) {
	var s Set
	c := New(Ace, Spades)
	if s.Has(c) || !s.Add(c) || !s.Has(c) || s.Add(c) {
		t.Fatal("set semantics broken")
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{"As", "10h", "K♠", "", "zz"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		c, err := Parse(s)
		if err != nil {
			return
		}
		if !c.Valid() {
			t.Fatalf("Parse(%q) returned invalid card %d", s, c)
		}
		if back, err := Parse(c.String()); err != nil || back != c {
			t.Fatalf("round trip of %q failed", s)
		}
	})
}
