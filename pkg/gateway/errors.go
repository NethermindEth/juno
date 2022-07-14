package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
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
// starkErr ErrResponse.Message == msg, otherwise a generic error is
// returned.
func clientErr(w http.ResponseWriter, code int, starkErr, msg string) {
	// XXX: Client error currently defaults to a
	// "StarkErrorCode.MALFORMED_REQUEST" error code but the feeder tends
	// return more informative codes as well such as the indication of an
	// out of bounds error so that could also serve as an input to this
	// function.
	var res *ErrResponse
	switch {
	case msg != "":
		res = &ErrResponse{Code: starkErr, Message: msg}
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
		logErr.Println(err.Error())
		serverErr(w, err)
	}

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}

// notImplementedErr sets a 501 Not Implemented header and then serves
// a JSON formatted error.
func notImplementedErr(w http.ResponseWriter) {
	clientErr(w, http.StatusNotImplemented, "", "")
}

// serverErr sets a 500 Internal Server Error header and then serves a
// JSON formatted client error.
func serverErr(w http.ResponseWriter, err error) {
	// Log stack trace. Set frame depth to 2 in order to report the file
	// and line number where the error occurred.
	logErr.Output(2, fmt.Sprintf("%s\n%s", err.Error(), debug.Stack()))

	res := &ErrResponse{
		Code:     strconv.Itoa(http.StatusInternalServerError),
		Message:  http.StatusText(http.StatusInternalServerError),
		Problems: http.StatusText(http.StatusInternalServerError),
	}

	// XXX: Might prefer json.Marshal instead to reduce the payload size.
	raw, err := json.MarshalIndent(&res, "", "  ")
	if err != nil {
		logErr.Println(err.Error())
		panic(err)
	}

	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}
