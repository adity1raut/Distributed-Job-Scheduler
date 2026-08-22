// Package httpx holds the small set of helpers every handler uses to read
// and write JSON in a consistent shape, including the structured error
// envelope described in docs/design-decisions.md.
package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/adity1raut/job-scheduler/internal/apperr"
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func WriteError(w http.ResponseWriter, requestID string, err error) {
	appErr := apperr.As(err)
	WriteJSON(w, appErr.Status, errorEnvelope{Error: errorBody{
		Code:      appErr.Code,
		Message:   appErr.Message,
		RequestID: requestID,
	}})
}

func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return apperr.BadRequest("invalid request body: " + err.Error())
	}
	return nil
}
