package football

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

func TestFetchRequiresBothValidatedSources(t *testing.T) {
	fixtureStatus := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fixtures/" {
			w.WriteHeader(fixtureStatus)
			_, _ = w.Write([]byte(`[{"id":1,"event":1,"team_h":1,"team_a":2,"kickoff_time":"2026-11-08T12:00:00Z","team_h_difficulty":2,"team_a_difficulty":4}]`))
			return
		}
		_, _ = w.Write([]byte(`{"events":[{"id":1,"deadline_time":"2026-11-08T10:00:00Z"}],"teams":[{"id":1,"name":"A","short_name":"A"},{"id":2,"name":"B","short_name":"B"}],"element_types":[{"id":1,"singular_name_short":"GKP"},{"id":2,"singular_name_short":"DEF"},{"id":3,"singular_name_short":"MID"},{"id":4,"singular_name_short":"FWD"}],"elements":[{"id":1,"first_name":"Test","second_name":"Player","team":1,"element_type":2,"now_cost":50,"status":"a","minutes":900,"starts":10,"expected_goals":"1.1","expected_assists":"0.8","defensive_contribution":105}]}`))
	}))
	defer server.Close()
	client := &Client{url: server.URL + "/bootstrap-static/", httpClient: server.Client()}
	data, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Fixtures) != 1 || data.Players[0].DefensiveActions != 105 || *data.Players[0].Minutes != 900 {
		t.Fatal("forecast inputs lost in normalization")
	}
	fixtureStatus = http.StatusServiceUnavailable
	if _, err = client.Fetch(context.Background()); err == nil {
		t.Fatal("partial provider refresh accepted")
	}
}

func TestDecodeNormalizesBootstrapData(t *testing.T) {
	payload := `{
		"events":[{"id":1,"deadline_time":"2026-08-08T10:00:00Z","is_current":true},{"id":38,"deadline_time":"2027-05-23T15:00:00Z","is_current":false}],
		"teams":[{"id":1,"code":3,"name":"Arsenal","short_name":"ARS"}],
		"element_types":[{"id":1,"singular_name_short":"GKP"},{"id":2,"singular_name_short":"DEF"},{"id":3,"singular_name_short":"MID"},{"id":4,"singular_name_short":"FWD"}],
		"elements":[{"id":1,"web_name":"Raya","first_name":"David","second_name":"Raya Martin","team":1,"element_type":1,"now_cost":60,"status":"a","total_points":10,"form":"4.5"}]
	}`
	data, err := decode(strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if data.SeasonName != "2026/27" || len(data.Players) != 1 || data.Players[0].Price != 60 || data.Clubs[0].KitCode != 3 {
		t.Fatalf("unexpected normalized data: %+v", data)
	}
	if data.Players[0].SelectedByPercent != nil {
		t.Fatal("missing ownership must remain unknown")
	}
	withOwnership := strings.Replace(payload, `"form":"4.5"`, `"form":"4.5","selected_by_percent":"42.7"`, 1)
	data, err = decode(strings.NewReader(withOwnership))
	if err != nil || data.Players[0].SelectedByPercent == nil || *data.Players[0].SelectedByPercent != 42.7 {
		t.Fatalf("ownership was not decoded: %v", err)
	}
	for _, invalid := range []string{"NaN", "101", "-1", "invalid"} {
		_, err = decode(strings.NewReader(strings.Replace(withOwnership, "42.7", invalid, 1)))
		if err == nil {
			t.Fatalf("accepted invalid ownership %q", invalid)
		}
	}
}

func TestEnrichSquadNumbersUsesOfficialClubList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h5><a>Arsenal</a></h5><p>1 David Raya<br />7 Bukayo Saka</p></body></html>`))
	}))
	defer server.Close()
	data := domain.FootballData{
		SeasonName: "2026/27",
		Clubs:      []domain.FootballClub{{ProviderID: 1, Name: "Arsenal"}},
		Players: []domain.FootballPlayer{
			{ClubID: 1, FirstName: "David", LastName: "Raya Martin", WebName: "Raya"},
			{ClubID: 1, FirstName: "Bukayo", LastName: "Saka", WebName: "Saka"},
		},
	}
	client := &Client{squadNumbersURL: server.URL, httpClient: server.Client()}
	if err := client.enrichSquadNumbers(context.Background(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Players[0].SquadNumber == nil || *data.Players[0].SquadNumber != 1 || data.Players[1].SquadNumber == nil || *data.Players[1].SquadNumber != 7 {
		t.Fatalf("shirt numbers were not matched: %+v", data.Players)
	}
}

func TestDecodeRejectsIncompleteData(t *testing.T) {
	if _, err := decode(strings.NewReader(`{"events":[],"teams":[],"elements":[]}`)); err == nil {
		t.Fatal("expected incomplete payload error")
	}
}
