package page

type Page[T any] struct {
	Data T `json:"data"`
}
