package api

import (
	"strings"
	"testing"
)

func TestNumberedWritesMovesWithTheirMoveNumbers(t *testing.T) {
	cases := []struct {
		sans     []string
		firstPly int
		want     string
	}{
		{[]string{"e4", "c5", "Nf3"}, 0, "1. e4 c5 2. Nf3"},
		{[]string{"d5", "exd5"}, 3, "2... d5 3. exd5"},
		{[]string{"c3"}, 2, "2. c3"},
		{nil, 0, ""},
	}
	for _, c := range cases {
		if got := numbered(c.sans, c.firstPly); got != c.want {
			t.Errorf("numbered(%v, %d) = %q, want %q", c.sans, c.firstPly, got, c.want)
		}
	}
}

func steps(sans ...string) []stepDTO {
	out := make([]stepDTO, 0, len(sans))
	for _, san := range sans {
		out = append(out, stepDTO{SAN: san})
	}
	return out
}

func half() *float64 { v := 0.5; return &v }

func baseReport() reportDTO {
	return reportDTO{
		Opponent: "Petrov, Ivan", MyColor: "white", TheirElo: 2105, MyElo: 1950,
		Summary:   summaryDTO{Games: 8, Wins: 2, Draws: 3, Losses: 3, First: "2019-01-01", Last: "2026-03-16"},
		MainLines: []lineDTO{{Line: steps("e4", "c5", "Nf3"), Games: 3, Wins: 1, Draws: 1, Losses: 1, Score: half()}},
		WeakSpots: []weakSpotDTO{{Line: steps("e4", "c5"), Move: moveDTO{SAN: "c3", Games: 2, Losses: 2, Score: new(float64)}}},
	}
}

func TestFactsStateTheStatisticsInPlainText(t *testing.T) {
	facts := renderFacts(baseReport())

	for _, want := range []string{
		"Opponent: Petrov, Ivan",
		"You play White",
		"8 games",
		"+2 =3 -3",
		"2019-01-01 to 2026-03-16",
		"1. e4 c5 2. Nf3 (3 games, +1 =1 -1)",
		"After 1. e4 c5: 2. c3 (2 games, they scored 0%, +0 =0 -2)",
	} {
		if !strings.Contains(facts, want) {
			t.Errorf("facts lack %q:\n%s", want, facts)
		}
	}
}

func TestFactsSayWhenEngineSectionsAreMissing(t *testing.T) {
	facts := renderFacts(baseReport())

	for _, want := range []string{"Recurring mistakes: not analysed", "Traps: not analysed"} {
		if !strings.Contains(facts, want) {
			t.Errorf("facts lack %q:\n%s", want, facts)
		}
	}
}

func TestFactsDescribeMistakesAndTraps(t *testing.T) {
	report := baseReport()
	report.Mistakes = &[]mistakeDTO{{
		Line: steps("e4", "e5", "Nf3"), NodeGames: 3, Move: moveDTO{SAN: "f6", Games: 2}, Loss: 160,
		Best: "Nc6", Eval: evalDTO{Text: "+1.91"}, Refutation: []string{"Nxe5", "Qe7"},
	}}
	report.Traps = &[]trapDTO{{
		Line: steps("e4", "c5"), Games: 6, Move: stepDTO{SAN: "c3"}, FacedGames: 2, Eval: evalDTO{Text: "+0.20"},
		ErrorChance: 0.75, ExpectedLoss: 135,
		Replies: []replyDTO{
			{SAN: "d5", Probability: 0.75, Loss: 180, Eval: evalDTO{Text: "+2.00"}, Punish: "exd5"},
			{SAN: "Nf6", Probability: 0.2, Loss: 0, Eval: evalDTO{Text: "+0.20"}},
		},
	}}

	facts := renderFacts(report)

	for _, want := range []string{
		"1. e4 e5 2. Nf3 f6: played in 2 of 3 games, gives away 1.6 pawns; the engine prefers 2... Nc6; you continue 3. Nxe5 Qe7 (+1.91 for you)",
		"After 1. e4 c5: 2. c3 (+0.20 for you; they faced it 2 times). A 2105-rated player goes wrong 75% of the time",
		"2... d5 (75%) loses 1.8 pawns, answer 3. exd5 (+2.00 for you)",
		"2... Nf6 (20%) is fine (+0.20 for you)",
	} {
		if !strings.Contains(facts, want) {
			t.Errorf("facts lack %q:\n%s", want, facts)
		}
	}
}

func TestFactsSayWhenAScanFoundNothing(t *testing.T) {
	report := baseReport()
	report.Mistakes, report.Traps = &[]mistakeDTO{}, &[]trapDTO{}

	facts := renderFacts(report)

	for _, want := range []string{"Recurring mistakes: none found", "Traps: none found"} {
		if !strings.Contains(facts, want) {
			t.Errorf("facts lack %q:\n%s", want, facts)
		}
	}
}
