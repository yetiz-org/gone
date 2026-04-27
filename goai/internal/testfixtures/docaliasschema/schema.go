package docaliasschema

type Page[T any] struct {
	Data T `json:"data"`
}

type SharedResponse struct {
	ID string `json:"id"`
}
