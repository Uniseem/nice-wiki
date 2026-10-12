package translate

type Estimate struct {
	Characters, InputTokens, OutputTokens int
	Cost                                  float64
}

func EstimateCost(characters int, inputPerMillion, outputPerMillion float64) Estimate {
	input := characters/3 + 1
	output := characters/2 + 1
	return Estimate{Characters: characters, InputTokens: input, OutputTokens: output, Cost: float64(input)/1_000_000*inputPerMillion + float64(output)/1_000_000*outputPerMillion}
}
