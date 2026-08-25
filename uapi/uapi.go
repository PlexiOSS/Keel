package uapi

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"

	docs "github.com/PlexiOSS/Keel/doclib"
	"github.com/PlexiOSS/Keel/jsonimpl"
	"go.uber.org/zap"

	"github.com/go-playground/validator/v10"
	"golang.org/x/exp/slices"
)

func (s *UAPIState) SetCurrentTag(tag string) {
	s.InitData.Tag = tag
}

func SetupState(s UAPIState) {
	if s.Constants == nil {
		panic("Constants is nil")
	}

	State = &s
}

var (
	State *UAPIState
)

const (
	GET Method = iota
	POST
	PATCH
	PUT
	DELETE
	HEAD
)

func (m Method) String() string {
	switch m {
	case GET:
		return "GET"
	case POST:
		return "POST"
	case PATCH:
		return "PATCH"
	case PUT:
		return "PUT"
	case DELETE:
		return "DELETE"
	case HEAD:
		return "HEAD"
	}

	panic("Invalid method")
}

func (r Route) String() string {
	return r.Method.String() + " " + r.Pattern + " (" + r.OpId + ")"
}

func (r Route) Route(ro Router) {
	if r.OpId == "" {
		panic("OpId is empty: " + r.String())
	}

	if r.Handler == nil {
		panic("Handler is nil: " + r.String())
	}

	if r.Docs == nil {
		panic("Docs is nil: " + r.String())
	}

	if r.Pattern == "" {
		panic("Pattern is empty: " + r.String())
	}

	if State.InitData.Tag == "" {
		panic("CurrentTag is empty: " + r.String())
	}

	if r.Setup != nil {
		r.Setup()
	}

	if State.BaseSanityCheck != nil {
		err := State.BaseSanityCheck(r)

		if err != nil {
			panic("Base sanity check failed: " + err.Error())
		}
	}

	if r.SanityCheck != nil {
		err := r.SanityCheck()

		if err != nil {
			panic("Sanity check failed: " + err.Error())
		}
	}

	docsObj := r.Docs()

	docsObj.Pattern = r.Pattern
	docsObj.OpId = r.OpId
	docsObj.Method = r.Method.String()
	docsObj.Tags = []string{State.InitData.Tag}
	docsObj.AuthType = []string{}

	for _, auth := range r.Auth {
		t, ok := State.AuthTypeMap[auth.Type]

		if !ok {
			panic("Invalid auth type: " + auth.Type)
		}

		docsObj.AuthType = append(docsObj.AuthType, t)
	}

	if State.PatchDocs != nil {
		docsObj = State.PatchDocs(docsObj)
	}

	brStart := strings.Count(r.Pattern, "{")
	brEnd := strings.Count(r.Pattern, "}")
	pathParams := []string{}
	patternParams := []string{}

	for _, param := range docsObj.Params {
		if param.In == "" || param.Name == "" || param.Schema == nil {
			panic("Param is missing required fields: " + r.String())
		}

		if param.In == "path" {
			pathParams = append(pathParams, param.Name)
		}
	}

	if !r.DisablePathSlashCheck {
		for _, param := range strings.Split(r.Pattern, "/") {
			if strings.HasPrefix(param, "{") && strings.HasSuffix(param, "}") {
				patternParams = append(patternParams, param[1:len(param)-1])
			} else if strings.Contains(param, "{") || strings.Contains(param, "}") {
				panic("{ and } in pattern but does not start with it " + r.String())
			}
		}
	}

	if brStart != brEnd {
		panic("Mismatched { and } in pattern: " + r.String())
	}

	if brStart != len(pathParams) {
		panic("Mismatched number of params and { in pattern: " + r.String())
	}

	if !r.DisablePathSlashCheck {
		if !slices.Equal(patternParams, pathParams) {
			panic("Mismatched params in pattern and docs: " + r.String())
		}
	}

	if len(r.Aliases) > 0 {
		docsObj.Description += "\n\nAliases for this endpoint:"
		for pattern, reason := range r.Aliases {
			docsObj.Description += "\n\n" + pattern + " (" + reason + ")"
		}
	}

	docs.Route(docsObj)

	createRouteHandler(r, ro, r.Pattern)

	if len(r.Aliases) > 0 {
		for pattern := range r.Aliases {
			createRouteHandler(r, ro, pattern)
		}
	}
}

func createRouteHandler(r Route, ro Router, pat string) {
	switch r.Method {
	case GET:
		ro.Get(pat, func(w http.ResponseWriter, req *http.Request) {
			handle(r, w, req)
		})
	case POST:
		ro.Post(pat, func(w http.ResponseWriter, req *http.Request) {
			handle(r, w, req)
		})
	case PATCH:
		ro.Patch(pat, func(w http.ResponseWriter, req *http.Request) {
			handle(r, w, req)
		})
	case PUT:
		ro.Put(pat, func(w http.ResponseWriter, req *http.Request) {
			handle(r, w, req)
		})
	case DELETE:
		ro.Delete(pat, func(w http.ResponseWriter, req *http.Request) {
			handle(r, w, req)
		})
	case HEAD:
		ro.Head(pat, func(w http.ResponseWriter, req *http.Request) {
			handle(r, w, req)
		})
	default:
		panic("Unknown method for route: " + r.String())
	}
}

func respond(ctx context.Context, w http.ResponseWriter, data chan HttpResponse) {
	select {
	case <-ctx.Done():
		return
	case msg, ok := <-data:
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(State.Constants.InternalServerError))
		}

		if msg.Redirect != "" {
			msg.Headers = map[string]string{
				"Location":     msg.Redirect,
				"Content-Type": "text/html; charset=utf-8",
			}
			msg.Data = "<a href=\"" + msg.Redirect + "\">Found</a>.\n"
			msg.Status = http.StatusFound
		}

		if len(msg.Headers) > 0 {
			for k, v := range msg.Headers {
				w.Header().Set(k, v)
			}
		}

		if msg.Json != nil {
			bytes, err := jsonimpl.Marshal(msg.Json)

			if err != nil {
				State.Logger.Error("[uapi.respond] Failed to unmarshal JSON response", zap.Error(err), zap.Int("size", len(msg.Data)))
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(State.Constants.InternalServerError))
				return
			}

			msg.Json = nil
			msg.Bytes = bytes
		}

		if msg.Status == 0 {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(msg.Status)
		}

		if len(msg.Bytes) > 0 {
			w.Write(msg.Bytes)
		}

		w.Write([]byte(msg.Data))
		return
	}
}

func CompileValidationErrors(payload any) map[string]string {
	var errors = make(map[string]string)

	structType := reflect.TypeOf(payload)

	for _, f := range reflect.VisibleFields(structType) {
		errors[f.Name] = f.Tag.Get("msg")

		arrayMsg := f.Tag.Get("amsg")

		if arrayMsg != "" {
			errors[f.Name+"$arr"] = arrayMsg
		}
	}

	return errors
}

func ValidatorErrorResponse(compiled map[string]string, v validator.ValidationErrors) HttpResponse {
	var errors = make(map[string]string)

	firstError := ""

	for i, err := range v {
		fname := err.StructField()
		if strings.Contains(err.Field(), "[") {
			fname = strings.Split(err.Field(), "[")[0] + "$arr"
		}

		field := compiled[fname]

		var errorMsg string
		if field != "" {
			errorMsg = field + " [" + err.Tag() + "]"
		} else {
			errorMsg = err.Error()
		}

		if i == 0 {
			firstError = errorMsg
		}

		errors[err.StructField()] = errorMsg
	}

	return HttpResponse{
		Status: http.StatusBadRequest,
		Json:   State.DefaultResponder.New(firstError, errors),
	}
}

func DefaultResponse(statusCode int) HttpResponse {
	switch statusCode {
	case http.StatusForbidden:
		return HttpResponse{
			Status: statusCode,
			Data:   State.Constants.Forbidden,
		}
	case http.StatusUnauthorized:
		return HttpResponse{
			Status: statusCode,
			Data:   State.Constants.Unauthorized,
		}
	case http.StatusNotFound:
		return HttpResponse{
			Status: statusCode,
			Data:   State.Constants.ResourceNotFound,
		}
	case http.StatusBadRequest:
		return HttpResponse{
			Status: statusCode,
			Data:   State.Constants.BadRequest,
		}
	case http.StatusInternalServerError:
		return HttpResponse{
			Status: statusCode,
			Data:   State.Constants.InternalServerError,
		}
	case http.StatusMethodNotAllowed:
		return HttpResponse{
			Status: statusCode,
			Data:   State.Constants.MethodNotAllowed,
		}
	case http.StatusNoContent, http.StatusOK:
		return HttpResponse{
			Status: http.StatusNoContent,
		}
	}

	return HttpResponse{
		Status: statusCode,
		Data:   State.Constants.InternalServerError,
	}
}

func handle(r Route, w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	resp := make(chan HttpResponse)

	go func() {
		defer func() {
			err := recover()

			if err != nil {
				State.Logger.Error("[uapi/handle] Request handler panic'd", zap.String("operationId", r.OpId), zap.String("method", req.Method), zap.String("endpointPattern", r.Pattern), zap.String("path", req.URL.Path), zap.Any("error", err))
				resp <- HttpResponse{
					Status: http.StatusInternalServerError,
					Data:   State.Constants.InternalServerError,
				}
			}
		}()

		authData, httpResp, ok := State.Authorize(r, req)

		if !ok {
			resp <- httpResp
			return
		}

		rd := &RouteData{
			Context: ctx,
			Auth:    authData,
		}

		if State.RouteDataMiddleware != nil {
			var err error
			rd, err = State.RouteDataMiddleware(rd, req)

			if err != nil {
				resp <- HttpResponse{
					Status: http.StatusInternalServerError,
					Json:   State.DefaultResponder.New(err.Error(), nil),
				}
				return
			}
		}

		resp <- r.Handler(*rd, req)
	}()

	respond(ctx, w, resp)
}

func marshalReq(r *http.Request, dst interface{}) (resp HttpResponse, ok bool) {
	defer r.Body.Close()

	bodyBytes, err := io.ReadAll(r.Body)

	if err != nil {
		State.Logger.Error("[uapi/marshalReq] Failed to read body", zap.Error(err), zap.Int("size", len(bodyBytes)))
		return DefaultResponse(http.StatusInternalServerError), false
	}

	if len(bodyBytes) == 0 {
		return HttpResponse{
			Status: http.StatusBadRequest,
			Data:   State.Constants.BodyRequired,
		}, false
	}

	err = jsonimpl.Unmarshal(bodyBytes, &dst)

	if err != nil {
		State.Logger.Error("[uapi/marshalReq] Failed to unmarshal JSON", zap.Error(err), zap.Int("size", len(bodyBytes)))
		return HttpResponse{
			Status: http.StatusBadRequest,
			Json: State.DefaultResponder.New("Invalid JSON", map[string]string{
				"error": err.Error(),
			}),
		}, false
	}

	return HttpResponse{}, true
}

func MarshalReq(r *http.Request, dst any) (resp HttpResponse, ok bool) {
	return marshalReq(r, dst)
}

func MarshalReqWithHeaders(r *http.Request, dst any, headers map[string]string) (resp HttpResponse, ok bool) {
	resp, err := marshalReq(r, dst)

	resp.Headers = headers

	return resp, err
}
