package suggest

type Signals struct {
	Languages []string `json:"languages"`
	Timezone  string   `json:"timezone"`
	Country   string   `json:"country"`
	Current   string   `json:"current"`
}

func (s Signals) Key() string {
	value := s.Timezone + "|" + s.Country + "|" + s.Current
	for _, language := range s.Languages {
		value += "|" + language
	}
	return value
}
