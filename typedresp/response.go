package typedresp

import (
	"encoding/json"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type Error struct {
	Status  int
	Message string
}

func (e Error) Error() string {
	return e.Message
}

func NewErr(err error) Error {
	return Error{Status: http.StatusInternalServerError, Message: err.Error()}
}

func ErrStatus(status int, message string) Error {
	return Error{Status: status, Message: message}
}

type Response struct {
	status int
	json   any
	text   *string
	stream io.ReadCloser
	noBody bool
}

func JSON(status int, v any) Response {
	return Response{status: status, json: v}
}

func Text(status int, s string) Response {
	return Response{status: status, text: &s}
}

func NoContent() Response {
	return Response{status: http.StatusNoContent, noBody: true}
}

func Stream(status int, body io.ReadCloser) Response {
	return Response{status: status, stream: body}
}

func (r Response) Write(w http.ResponseWriter, logger *zap.Logger) {
	switch {
	case r.stream != nil:
		defer r.stream.Close()

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(r.status)

		if _, err := io.Copy(w, r.stream); err != nil && logger != nil {
			logger.Error("typedresp: failed streaming response body", zap.Error(err))
		}
	case r.json != nil:
		body, err := json.Marshal(r.json)

		if err != nil {
			if logger != nil {
				logger.Error("typedresp: failed marshalling response", zap.Error(err))
			}
			Text(http.StatusInternalServerError, err.Error()).Write(w, logger)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(r.status)
		w.Write(body)
	case r.text != nil:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(r.status)
		w.Write([]byte(*r.text))
	default:
		w.WriteHeader(r.status)
	}
}
