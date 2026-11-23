package core_test

import (
	"reflect"
	"testing"
	"time"

	"opposition/internal/core"
)

func TestPlayersAreListedWithGameCountsMostGamesFirst(t *testing.T) {
	games := []core.Game{
		{White: "Carlsen, Magnus", Black: "Nepomniachtchi, Ian"},
		{White: "Nepomniachtchi, Ian", Black: "Carlsen, Magnus"},
		{White: "Carlsen, Magnus", Black: "Caruana, Fabiano"},
	}

	got := core.Players(games, "")

	want := []core.Player{
		{Name: "Carlsen, Magnus", Games: 3},
		{Name: "Nepomniachtchi, Ian", Games: 2},
		{Name: "Caruana, Fabiano", Games: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestPlayersCanBeSearchedByPartOfTheNameIgnoringCase(t *testing.T) {
	games := []core.Game{
		{White: "Carlsen, Magnus", Black: "Nepomniachtchi, Ian"},
		{White: "Caruana, Fabiano", Black: "Ding, Liren"},
	}

	got := core.Players(games, "MAGN")

	want := []core.Player{{Name: "Carlsen, Magnus", Games: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func day(year, month, dayOfMonth int) time.Time {
	return time.Date(year, time.Month(month), dayOfMonth, 0, 0, 0, 0, time.UTC)
}

func TestPlayersCarryTheirMostRecentRating(t *testing.T) {
	games := []core.Game{
		{White: "Opp", Black: "A", WhiteElo: 1900, Date: day(2024, 1, 1)},
		{White: "B", Black: "Opp", BlackElo: 2050, Date: day(2026, 3, 1)},
		{White: "Opp", Black: "C", WhiteElo: 1980, Date: day(2025, 6, 1)},
	}

	got := core.Players(games, "Opp")

	if len(got) != 1 || got[0].Rating != 2050 {
		t.Errorf("got %+v, want Opp rated 2050 from the latest game", got)
	}
}

func TestAGameWithoutARatingDoesNotHideAnEarlierOne(t *testing.T) {
	games := []core.Game{
		{White: "Opp", Black: "A", WhiteElo: 1900, Date: day(2024, 1, 1)},
		{White: "Opp", Black: "B", Date: day(2026, 3, 1)},
	}

	if got := core.Players(games, "Opp"); got[0].Rating != 1900 {
		t.Errorf("rating = %d, want 1900", got[0].Rating)
	}
}

func TestAnUndatedGameOnlySuppliesTheRatingWhenNoDatedGameHasOne(t *testing.T) {
	undatedOnly := []core.Game{{White: "Opp", Black: "A", WhiteElo: 1700}}
	both := []core.Game{
		{White: "Opp", Black: "A", WhiteElo: 1700},
		{White: "Opp", Black: "B", WhiteElo: 1850, Date: day(2020, 1, 1)},
	}

	if got := core.Players(undatedOnly, "Opp"); got[0].Rating != 1700 {
		t.Errorf("undated only: rating = %d, want 1700", got[0].Rating)
	}
	if got := core.Players(both, "Opp"); got[0].Rating != 1850 {
		t.Errorf("dated and undated: rating = %d, want 1850", got[0].Rating)
	}
}

func TestAPlayerWithoutAnyRatingHasRatingZero(t *testing.T) {
	if got := core.Players([]core.Game{{White: "Opp", Black: "A"}}, "Opp"); got[0].Rating != 0 {
		t.Errorf("rating = %d, want 0", got[0].Rating)
	}
}
