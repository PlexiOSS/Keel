package uapi

import (
	"context"
	"net/http"

	docs "github.com/PlexiOSS/Keel/doclib"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type UAPIConstants struct {
	ResourceNotFound    string
	BadRequest          string
	Forbidden           string
	Unauthorized        string
	InternalServerError string
	MethodNotAllowed    string
	BodyRequired        string
}

type UAPIDefaultResponder interface {
	New(msg string, ctx map[string]string) any
}

type UAPIInitData struct {
	Tag string
}

type UAPIState struct {
	Logger              *zap.Logger
	Authorize           func(r Route, req *http.Request) (AuthData, HttpResponse, bool)
	AuthTypeMap         map[string]string
	RouteDataMiddleware func(rd *RouteData, req *http.Request) (*RouteData, error)
	BaseSanityCheck     func(r Route) error
	PatchDocs           func(d *docs.Doc) *docs.Doc
	Context             context.Context
	Constants           *UAPIConstants
	DefaultResponder    UAPIDefaultResponder
	InitData            UAPIInitData
}

type APIRouter interface {
	Routes(r *chi.Mux)
	Tag() (string, string)
}

type Method int

type AuthType struct {
	URLVar       string
	Type         string
	AllowedScope string
}

type AuthData struct {
	TargetType string         `json:"target_type"`
	ID         string         `json:"id"`
	Authorized bool           `json:"authorized"`
	Banned     bool           `json:"banned"`
	Data       map[string]any `json:"data"`
}

type Route struct {
	Method                Method
	Pattern               string
	Aliases               map[string]string
	OpId                  string
	Handler               func(d RouteData, r *http.Request) HttpResponse
	Setup                 func()
	Docs                  func() *docs.Doc
	Auth                  []AuthType
	ExtData               map[string]any
	AuthOptional          bool
	SanityCheck           func() error
	DisablePathSlashCheck bool
}

type RouteData struct {
	Context context.Context
	Auth    AuthData
	Props   map[string]string
}

type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
	Head(pattern string, h http.HandlerFunc)
}

type HttpResponse struct {
	Data     string
	Bytes    []byte
	Json     any
	Headers  map[string]string
	Status   int
	Redirect string
}
