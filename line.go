package rotor

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type ctxKey int

const (
	sessionCtxKey    ctxKey = 0
	invocationCtxKey ctxKey = 1
	attributesCtxKey ctxKey = 2
)

//Handler interface allows any type to handle lambda events
type Handler interface {
	HandleEvent(ctx context.Context, msg json.RawMessage) (interface{}, error)
}

//HandlerFunc is an adaptor that can handle
type HandlerFunc func(ctx context.Context, msg json.RawMessage) (interface{}, error)

//Middleware allows plugins to manipulate the context and message passed to handlers
type Middleware func(Handler) Handler

func (mux *Mux) buildChain(endpoint Handler) Handler {
	// Return ahead of time if there aren't any middlewares for the chain
	if len(mux.middleware) == 0 {
		return endpoint
	}

	// Wrap the end handler with the middleware chain
	h := mux.middleware[len(mux.middleware)-1](endpoint)
	for i := len(mux.middleware) - 2; i >= 0; i-- {
		h = mux.middleware[i](h)
	}

	return h
}

//HandleEvent implements Handler
func (h HandlerFunc) HandleEvent(ctx context.Context, msg json.RawMessage) (interface{}, error) {
	return h(ctx, msg)
}

//Mux mutiplexes events to more specific handlers based on regexp matching of a context field
type Mux struct {
	handlers   map[*regexp.Regexp]Handler
	middleware []Middleware
}

//NewMux sets up new event multiplexer
func NewMux() *Mux {
	return &Mux{
		handlers: map[*regexp.Regexp]Handler{},
	}
}

//MatchARN adds a handler to the multiplexer that will be called when a caller matches the ARN of the invoked lambda function
func (mux *Mux) MatchARN(exp *regexp.Regexp, h Handler) {
	mux.handlers[exp] = h
}

//Use adds a middleware to the lambda handler
func (mux *Mux) Use(mw Middleware) {
	mux.middleware = append(mux.middleware, mw)
}

//Handle mill match the invoked function arn to a specific handler
func (mux *Mux) Handle(msg json.RawMessage, invoc *Invocation) (interface{}, error) {
	var testedExp []string
	for exp, handler := range mux.handlers {
		if exp.MatchString(invoc.InvokedFunctionARN) {
			ctx := context.Background()
			ctx = context.WithValue(ctx, invocationCtxKey, invoc)
			endpoint := HandlerFunc(func(ctx context.Context, msg json.RawMessage) (interface{}, error) {
				return handler.HandleEvent(ctx, msg)
			})

			return mux.buildChain(endpoint).HandleEvent(ctx, msg)
		}

		testedExp = append(testedExp, exp.String())
	}

	return nil, fmt.Errorf("none of the handlers (%s) matched the invoked function's ARN '%s'", strings.Join(testedExp, ", "), invoc.InvokedFunctionARN)
}
