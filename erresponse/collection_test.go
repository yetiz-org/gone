package erresponse

import "testing"

func Test_Collect(t *testing.T) {
	for _, known := range []ErrorResponse{InvalidRequest, NotFound, ServerError} {
		if Collection.ErrorResponses[known] != known {
			t.Errorf("predefined response %q is missing from the catalogue", known.Code())
		}
	}
	codes := make(map[string]ErrorResponse, len(Collection.ErrorResponses))
	for registered, response := range Collection.ErrorResponses {
		if registered != response {
			t.Errorf("catalogue entry %q does not preserve the registered response", registered.Code())
		}
		code := response.Code()
		if code == "" {
			t.Error("catalogue contains an empty error code")
		}
		if previous, exists := codes[code]; exists {
			t.Errorf("error code %q is shared by %q and %q", code, previous.ErrorName(), response.ErrorName())
		}
		codes[code] = response
	}
}
