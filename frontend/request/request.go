package request

import (
	"context"
	"net/http"

	"github.com/google/logger"
	"github.com/gorilla/sessions"
	"golang.org/x/text/language"

  "github.com/jsannemo/omogenjudge/frontend/templates"
  "github.com/jsannemo/omogenjudge/storage/models"
)

// RequestContext keeps request-scoped information available during a request
type RequestContext struct {
	// ID of the currently logged-in user
	UserId int32

  User *models.Account

	// Locales as set in Accept-Language
	Locales []language.Tag
}

// Request represents a single HTTP request
type Request struct {
	Writer   http.ResponseWriter
	Request  *http.Request
	Response Response
	Context  *RequestContext
	Session  *sessions.Session
}

// SetValue sets a request-scoped value with a given key.
// The value can be retrieved using Value(string)
func (r *Request) SetValue(key string, value interface{}) {
	r.Request = r.Request.WithContext(context.WithValue(r.Request.Context(), key, value))
}

// Value retrives a request-scoped value set with SetValue.
func (r *Request) Value(key string) interface{} {
	return r.Request.Context().Value(key)
}

func NewRequest(w http.ResponseWriter, r *http.Request) *Request {
	return &Request{w, r, nil, &RequestContext{}, nil}
}

// Response is the return type of a HTTP processor.
// It represents the response that should be sent back to the client.
// This is cached since we may have processors that should be run after the
// main request handler, but before the response is actually sent.
type Response interface {
	// The HTTP status code to return.
	Code() int
}

type templateResponse struct {
  // The name of the template element to execute
	Name     string

  // The data to be passed when executing the template
	Data     interface{}
}

func (templateResponse) Code() int {
	return http.StatusOK
}

type redirectResponse struct {
  // The path the client should be redirected to
	Path string
}

func (redirectResponse) Code() int {
	return http.StatusFound
}

type errorResponse struct {
  // The error that caused this error response
	Error error
}

func (errorResponse) Code() int {
	return http.StatusInternalServerError
}

// Error returns a 500 Internal Server Error Response wrapping the given error.
// The error message is not displayed, but it is logged internally.
func Error(err error) Response {
	return &errorResponse{err}
}

type codedResponse struct {
  msg string
  code int
}

func (c *codedResponse) Code() int {
  return c.code
}

func NotFound() Response {
  return &codedResponse{code: http.StatusNotFound}
}

func BadRequest(msg string) Response {
  return &codedResponse{msg: msg, code: http.StatusBadRequest}
}

// Redirect returns a Response that will cause a client to redirect to the given URL.
func Redirect(url string) Response {
	return &redirectResponse{url}
}

// Template returns a Response that renders a given template to the client.
func Template(name string, data interface{}) Response {
	return &templateResponse{name, data}
}

// Write writes the Response currently stored in the request to the client.
func (req *Request) Write(w http.ResponseWriter) {
	switch r := req.Response.(type) {
	case *errorResponse:
    logger.Errorf("Error response: %v", r.Error)
		w.WriteHeader(r.Code())
	case *redirectResponse:
		w.Header().Set("Location", r.Path)
		w.WriteHeader(r.Code())
	case *templateResponse:
		err := templates.ExecuteTemplates(w, r.Name,
			struct {
				D interface{}
				C interface{}
			}{r.Data, req.Context})
		if err != nil {
			logger.Errorf("Failed rendering template: %v", err)
		}
	default:
		w.WriteHeader(r.Code())
	}
}
