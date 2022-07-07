package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ErrResponse represents the JSON formatted error response that is
// returned by the feeder gateway.
type ErrResponse struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Problems string `json:"problems,omitempty"`
}

// clientErr sets a 400 Bad Request header and then serves a JSON
// formatted client error. If msg != "", ErrResponse.Code will be set to
// "StarkErrorCode.MALFORMED_REQUEST" with ErrResponse.Message == msg,
// otherwise a generic error is returned.
func clientErr(w http.ResponseWriter, code int, msg string) {
	var res *ErrResponse
	switch {
	case msg != "":
		res = &ErrResponse{Code: "StarkErrorCode.MALFORMED_REQUEST", Message: msg}
	default:
		res = &ErrResponse{
			Code:     strconv.Itoa(code),
			Message:  fmt.Sprintf("%d: %s", code, http.StatusText(code)),
			Problems: http.StatusText(code),
		}
	}

	// XXX: Might prefer json.Marshal instead to reduce the payload size.
	raw, err := json.MarshalIndent(&res, "", "  ")
	if err != nil {
		serverErr(w, err)
	}

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}

// serverErr sets a 500 Internal Server Error header and then serves a
// JSON formatted client error.
func serverErr(w http.ResponseWriter, err error) {
	// TODO: Log stack trace.

	res := &ErrResponse{
		Code:     strconv.Itoa(http.StatusInternalServerError),
		Message:  http.StatusText(http.StatusInternalServerError),
		Problems: http.StatusText(http.StatusInternalServerError),
	}

	// XXX: Might prefer json.Marshal instead to reduce the payload size.
	raw, err := json.MarshalIndent(&res, "", "  ")
	if err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}
