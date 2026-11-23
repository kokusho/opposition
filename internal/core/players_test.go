package core_test

import (
	"reflect"
	"testing"

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
