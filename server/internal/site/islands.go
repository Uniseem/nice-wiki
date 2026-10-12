package site

import (
	"encoding/json"
	"html/template"
)

type Island struct {
	Name  string
	Props template.JS
}

func NewIsland(name string, props any) (Island, error) {
	data, err := json.Marshal(props)
	if err != nil {
		return Island{}, err
	}
	return Island{Name: name, Props: template.JS(data)}, nil
}
