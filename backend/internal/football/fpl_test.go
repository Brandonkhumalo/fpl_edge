package football

import (
	"strings"
	"testing"
)

func TestDecodeNormalizesBootstrapData(t *testing.T) {
	payload := `{
		"events":[{"id":1,"deadline_time":"2026-08-08T10:00:00Z","is_current":true},{"id":38,"deadline_time":"2027-05-23T15:00:00Z","is_current":false}],
		"teams":[{"id":1,"name":"Arsenal","short_name":"ARS"}],
		"element_types":[{"id":1,"singular_name_short":"GKP"},{"id":2,"singular_name_short":"DEF"},{"id":3,"singular_name_short":"MID"},{"id":4,"singular_name_short":"FWD"}],
		"elements":[{"id":1,"first_name":"David","second_name":"Raya","team":1,"element_type":1,"now_cost":60,"status":"a","total_points":10,"form":"4.5"}]
	}`
	data, err := decode(strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if data.SeasonName != "2026/27" || len(data.Players) != 1 || data.Players[0].Price != 60 {
		t.Fatalf("unexpected normalized data: %+v", data)
	}
}

func TestDecodeRejectsIncompleteData(t *testing.T) {
	if _, err := decode(strings.NewReader(`{"events":[],"teams":[],"elements":[]}`)); err == nil {
		t.Fatal("expected incomplete payload error")
	}
}
