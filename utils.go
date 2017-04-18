package rotor

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

// GatewayRequest represents an Amazon API Gateway Proxy Event.
type GatewayRequest struct {
	HTTPMethod            string
	Headers               map[string]string
	Resource              string
	PathParameters        map[string]string
	Path                  string
	QueryStringParameters map[string]string
	Body                  string
	IsBase64Encoded       bool
	StageVariables        map[string]string
}

//GatewayResponse is returned to the API Gateway
type GatewayResponse struct {
	StatusCode int               `json:"statusCode"`
	Body       string            `json:"body"`
	Headers    map[string]string `json:"headers"`
}

//GatewayHandler implements the lambda handler while allowing normal http.Handlers to serve
type GatewayHandler struct {
	stripN int
	httpH  http.Handler
}

//NewGatewayHandler makes it easy to serve normal http
func NewGatewayHandler(stripBasePaths int, httpH http.Handler) *GatewayHandler {
	return &GatewayHandler{
		stripN: stripBasePaths,
		httpH:  httpH,
	}
}

//HandleEvent takes invocations from the API Gateway and turns them into http.Handler invocations
func (gwh *GatewayHandler) HandleEvent(ctx context.Context, msg json.RawMessage) (res interface{}, err error) {
	req := &GatewayRequest{}
	err = json.Unmarshal(msg, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode gateway request")
	}

	//parse path
	loc, err := url.Parse(req.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse request path")
	}

	//strip path base
	if gwh.stripN > 0 {
		comps := strings.SplitN(
			strings.TrimLeft(loc.Path, "/"),
			"/", gwh.stripN+1)
		if len(comps) >= gwh.stripN {
			loc.Path = "/" + strings.Join(comps[gwh.stripN:], "/")
		} else {
			loc.Path = "/"
		}
	}

	q := loc.Query()
	for k, param := range req.QueryStringParameters {
		q.Set(k, param)
	}

	loc.RawQuery = q.Encode()
	r, err := http.NewRequest(req.HTTPMethod, loc.String(), bytes.NewBufferString(req.Body))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP request")
	}

	for k, val := range req.Headers {
		for _, v := range strings.Split(val, ",") {
			r.Header.Add(k, strings.TrimSpace(v))
		}
	}

	w := &bufferedResponse{
		statusCode: http.StatusOK, //like standard lib, assume 200
		header:     http.Header{},
		Buffer:     bytes.NewBuffer(nil),
	}

	gwh.httpH.ServeHTTP(w, r.WithContext(ctx))

	resp := &GatewayResponse{
		StatusCode: w.statusCode,
		Body:       w.Buffer.String(),
		Headers:    map[string]string{},
	}

	for k, v := range w.header {
		resp.Headers[k] = strings.Join(v, ",")
	}

	return resp, nil
}

//bufferedResponse implements the response writer interface but buffers the body which is necessary for the creating a JSON formatted Lambda response anyway
type bufferedResponse struct {
	statusCode int
	header     http.Header
	*bytes.Buffer
}

func (br *bufferedResponse) Header() http.Header    { return br.header }
func (br *bufferedResponse) WriteHeader(status int) { br.statusCode = status }
