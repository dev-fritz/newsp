package dtos

// Pagination bounds enforced by the server.
const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

// PageMeta describes a page of a list response.
type PageMeta struct {
	Total  int `json:"total" example:"42"`
	Limit  int `json:"limit" example:"20"`
	Offset int `json:"offset" example:"0"`
}
