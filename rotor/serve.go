package rotor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

//LambdaEventContextKey is the key in the request's context.Context that holds the original
var LambdaEventContextKey = "lambda-event"

//LambdaContextContextKey is the key in the request's context.Context that holds the raw lambda context
var LambdaContextContextKey = "lambda-context"

//proxyResponse is a lambda message with specific fields that are expected by the API Gateway
type proxyResponse struct {
	StatusCode int               `json:"statusCode"`
	Body       string            `json:"body"`
	Headers    map[string]string `json:"headers"`
}

//bufferedResponse implements the response writer interface but buffers the body which
//is necessary for the creating a JSON formatted Lambda response anyway
type bufferedResponse struct {
	statusCode int
	header     http.Header
	*bytes.Buffer
}

func (br *bufferedResponse) Header() http.Header    { return br.header }
func (br *bufferedResponse) WriteHeader(status int) { br.statusCode = status }

//Output represents an outgoing message from the Lambda function to the API gateway
type Output struct {
	Error string      `json:"error,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

//proxyRequest is a Lambda event that comes from the API Gateway with the lambda proxy integration
type proxyRequest struct {
	Resource              string            `json:"resource"`
	Path                  string            `json:"path"`
	HTTPMethod            string            `json:"httpMethod"`
	Headers               map[string]string `json:"headers"`
	QueryStringParameters map[string]string `json:"queryStringParameters"`
	PathParameters        map[string]string `json:"pathParameters"`
	StageVariables        map[string]string `json:"stageVariables"`
	Body                  string            `json:"body"`
}

//Input represents an incoming message from the API Gateway to the Lambda function
type Input struct {
	Context interface{}     `json:"context"` //this is passed as an opaque value to the request context
	Event   json.RawMessage `json:"event"`
}

func outputErr(format string, a ...interface{}) (out *Output) {
	return &Output{Error: fmt.Sprintf(format, a...)}
}

//A Handler responds to Lambda Input with Lambda Output
type Handler interface {
	HandleEvent(in *Input) (out *Output, err error)
}

//GatewayProxyHandler can handle lambda input from the AWS api gateway configured
//with an integration type of AWS_PROXY
type GatewayProxyHandler struct {
	h http.Handler
}

//HandleEvent will transform a proxy event into a standard lib http request and
//fills the context.Context of the request with Lambda information
func (gwh *GatewayProxyHandler) HandleEvent(in *Input) (out *Output, err error) {
	if in.Event == nil {
		return nil, fmt.Errorf("decoded input has no event key")
	}

	preq := &proxyRequest{}
	err = json.Unmarshal(in.Event, &preq)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal '%s' as proxy event: %v", string(in.Event), err)
	}

	if gwh.h == nil {
		return &Output{Value: &proxyResponse{http.StatusNotFound, "404 Not Found", nil}}, nil
	}

	r, err := http.NewRequest(preq.HTTPMethod, preq.Path, bytes.NewBufferString(preq.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to turn event %+v into http request: %v", preq, err)
	}

	for k, val := range preq.Headers {
		for _, v := range strings.Split(val, ",") {
			r.Header.Add(k, strings.TrimSpace(v))
		}
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, LambdaEventContextKey, in.Event)
	ctx = context.WithValue(ctx, LambdaContextContextKey, in.Context)
	r = r.WithContext(ctx)

	w := &bufferedResponse{
		statusCode: http.StatusOK, //like standard lib, assume 200
		header:     http.Header{},
		Buffer:     bytes.NewBuffer(nil),
	}

	gwh.h.ServeHTTP(w, r) //down the middleware chain

	val := &proxyResponse{
		StatusCode: w.statusCode,
		Body:       w.Buffer.String(),
		Headers:    map[string]string{},
	}

	for k, v := range w.header {
		val.Headers[k] = strings.Join(v, ",")
	}

	return &Output{Value: val}, nil
}

//Serve accepts incoming Lambda JSON on Reader 'in' an writes response JSON on
//writer 'out', it takes any Lambda Event.
func Serve(r io.Reader, w io.Writer, h Handler) (err error) {
	dec := json.NewDecoder(r)
	enc := json.NewEncoder(w)
	for {
		var out *Output
		var in *Input
		err := dec.Decode(&in)
		if err == io.EOF {
			break //input ended, nothing left to serve
		}

		//decoder cannot recover from an invalid decode, write error and return err
		if err != nil {
			err := fmt.Errorf("failed to decode input: %v", err)
			enc.Encode(outputErr(err.Error()))
			return err
		}

		//handle the newly decoded input
		out, err = h.HandleEvent(in)
		if err != nil {
			out = outputErr("failed to handle input: %v", err)
		}

		//default output
		if out == nil {
			out = &Output{}
		}

		err = enc.Encode(out)
		if err != nil {
			return enc.Encode(outputErr("failed to encode output: %v", err))
		}
	}

	return nil
}

//ServeHTTP accepts incoming Lambda JSON on Reader 'in' an writes response JSON on
//writer 'out', it only takes Lambda events that come from the AWS Gateway configured
//with the AWS_PROXY integration types. Serve returns an error whenever it is
//no longer able to serve output
func ServeHTTP(r io.Reader, w io.Writer, h http.Handler) (err error) {
	srv := &GatewayProxyHandler{h: h}
	return Serve(r, w, srv)
}
