package seo

import (
	_ "embed"
	"encoding/json"
)

//go:embed content.json
var raw []byte

type Page struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}
type Guide struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
}
type Content struct {
	UpdatedAt string          `json:"updatedAt"`
	Routes    map[string]Page `json:"routes"`
	Guides    []Guide         `json:"guides"`
}

var Published = func() Content {
	var content Content
	if err := json.Unmarshal(raw, &content); err != nil {
		panic(err)
	}
	return content
}()
