package domain

import "testing"

func TestProjectPointsUsesPositionScoring(t *testing.T) {
	input := ProjectionInput{Position: Midfielder, ExpectedMinutes: 90, ExpectedGoals: .5, ExpectedAssists: .25, CleanSheetProbability: .4, ExpectedBonus: .5}
	got := ProjectPoints(input, DefaultProjectionCoefficients())
	want := 6.15
	if got < want-0.001 || got > want+0.001 {
		t.Fatalf("got %.3f, want %.3f", got, want)
	}
}
