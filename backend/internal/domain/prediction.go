package domain

type ProjectionInput struct {
	Position                 Position
	ExpectedMinutes          float64
	ExpectedGoals            float64
	ExpectedAssists          float64
	CleanSheetProbability    float64
	ExpectedBonus            float64
	ExpectedDefensiveContrib float64
	ExpectedNegativePoints   float64
}

type ProjectionCoefficients struct {
	GoalPoints                  map[Position]float64
	AssistPoints                float64
	CleanSheetPoints            map[Position]float64
	DefensiveContributionPoints map[Position]float64
}

func DefaultProjectionCoefficients() ProjectionCoefficients {
	return ProjectionCoefficients{
		GoalPoints:                  map[Position]float64{Goalkeeper: 10, Defender: 6, Midfielder: 5, Forward: 4},
		AssistPoints:                3,
		CleanSheetPoints:            map[Position]float64{Goalkeeper: 4, Defender: 4, Midfielder: 1, Forward: 0},
		DefensiveContributionPoints: map[Position]float64{Goalkeeper: 0, Defender: 2, Midfielder: 2, Forward: 2},
	}
}

func ProjectPoints(input ProjectionInput, coefficients ProjectionCoefficients) float64 {
	appearance := 0.0
	if input.ExpectedMinutes > 0 {
		appearance = 1
	}
	if input.ExpectedMinutes >= 60 {
		appearance = 2
	}
	return appearance +
		input.ExpectedGoals*coefficients.GoalPoints[input.Position] +
		input.ExpectedAssists*coefficients.AssistPoints +
		input.CleanSheetProbability*coefficients.CleanSheetPoints[input.Position] +
		input.ExpectedBonus +
		input.ExpectedDefensiveContrib*coefficients.DefensiveContributionPoints[input.Position] -
		input.ExpectedNegativePoints
}
