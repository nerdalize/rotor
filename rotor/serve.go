package rotor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

//LambdaEventContextKey is the key in the context that holds the original
var LambdaEventContextKey = "lambda-event"

//LambdaContextContextKey is the context key that holds the raw lambda context
var LambdaContextContextKey = "lambda-context"

//ProxyResponse is a lambda message with specific fields that are expected by the API Gateway
type ProxyResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

//bufferedResponse implements the response writer interface but buffers the body
type bufferedResponse struct {
	statusCode int
	header     http.Header

	*bytes.Buffer
}

func (br *bufferedResponse) Header() http.Header {
	return br.header
}

func (br *bufferedResponse) WriteHeader(status int) {
	br.statusCode = status
}

//Output represents an outgoing message from the Lambda function to the API gateway
type Output struct {
	Error string         `json:"error,omitempty"`
	Value *ProxyResponse `json:"value"`
}

//ProxyRequest is a Lambda event that comes from the API Gateway with the lambda proxy integration
type ProxyRequest struct {
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
	Context interface{}   `json:"context"` //this is passed as an opaque value to the request context
	Event   *ProxyRequest `json:"event"`
}

func outputErr(format string, a ...interface{}) (out *Output) {
	return &Output{Error: fmt.Sprintf(format, a...)}
}

func handle(in *Input, h http.Handler) (out *Output, err error) {
	if in.Event == nil {
		return nil, fmt.Errorf("Decoded input has no event key")
	}

	r, err := http.NewRequest(in.Event.HTTPMethod, in.Event.Path, bytes.NewBufferString(in.Event.Body))
	if err != nil {
		return nil, fmt.Errorf("Failed to turn event %+v into http request: %v", in.Event, err)
	}

	for k, val := range in.Event.Headers {
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

	h.ServeHTTP(w, r) //down the middleware chain

	val := &ProxyResponse{
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
//writer 'out'.
func Serve(in io.Reader, out io.Writer, h http.Handler) {
	logs := log.New(os.Stderr, "", log.LstdFlags)
	dec := json.NewDecoder(io.TeeReader(os.Stdin, os.Stderr))
	enc := json.NewEncoder(io.MultiWriter(os.Stdout, os.Stderr))
	for {
		var out *Output
		var in *Input
		err := dec.Decode(&in)
		if err == io.EOF {
			break //stdin closed, nothing left to do
		}

		if err != nil {
			out = outputErr("failed to decode input: %v", err)
		} else {
			out, err = handle(in, h)
			if err != nil {
				out = outputErr("failed to handle input: %v", err)
			}

			if out == nil {
				out = &Output{Value: &ProxyResponse{404, nil, "not found"}}
			}
		}

		err = enc.Encode(out)
		if err != nil {
			logs.Fatal(enc.Encode(outputErr("failed to encode output: %v", err)))
		}
	}
}
