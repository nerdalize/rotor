package rotor_test

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nerdalize/rotor/rotor"
)

// ServeHTTP is a simple
func ExampleServeHTTP() {

	//handler can be any router from the Go ecosystem
	var handler http.Handler

	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		//the go 1.7 context.Context carries the lambda context
		lambdaContext := r.Context().Value(rotor.LambdaContextContextKey{})

		fmt.Fprintf(w, "hello world, ctx: %+v", lambdaContext)
	})

	log.Fatal(rotor.ServeHTTP(os.Stdin, os.Stdout, handler, rotor.ProxyConf{}))
}

func TestServeHTTP(t *testing.T) {
	testCases := []struct {
		input    string
		output   string
		serveErr bool
		h        http.Handler
		conf     rotor.ProxyConf
	}{
		{
			`{}`,
			`{"error":"failed to handle input: decoded input has no event key"}`, false,
			nil,
			rotor.ProxyConf{},
		},
		{
			`{aaa}`,
			`{"error":"failed to decode input: invalid character 'a' looking for beginning of object key string"}`,
			true,
			nil,
			rotor.ProxyConf{},
		},
		{
			`{"event":{}}`,
			`{"value":{"statusCode":404,"body":"404 Not Found","headers":null}}`, false,
			nil,
			rotor.ProxyConf{},
		},
		{
			`{"event":{}}`,
			`{"value":{"statusCode":200,"body":"","headers":{}}}`,
			false,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			rotor.ProxyConf{},
		},
		{
			`{"event":{"resource": 123}}`,
			`{"error":"failed to handle input: failed to unmarshal '{\"resource\": 123}' as proxy event: json: cannot unmarshal number into Go struct field proxyRequest.resource of type string"}`,
			false,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			rotor.ProxyConf{},
		},
		{
			`{"event":{"path": "/path", "queryStringParameters":{"email":"a@b"}}}`,
			`{"value":{"statusCode":200,"body":"/path?email=a%40b","headers":{}}}`, false,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "%+v", r.URL.String())
			}),
			rotor.ProxyConf{},
		},
		{
			`{"event":{"path": "/a/b/c/path", "queryStringParameters":{"email":"a@b"}}}`,
			`{"value":{"statusCode":200,"body":"/path?email=a%40b","headers":{}}}`, false,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "%+v", r.URL.String())
			}),
			rotor.ProxyConf{3},
		},
		{
			`{"event":{"path": "/foo", "queryStringParameters":{"email":"a@b"}}}`,
			`{"value":{"statusCode":200,"body":"/?email=a%40b","headers":{}}}`, false,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "%+v", r.URL.String())
			}),
			rotor.ProxyConf{3},
		},
	}

	for _, tc := range testCases {
		t.Run("input="+tc.input, func(t *testing.T) {

			inr, inw := io.Pipe()
			out := bytes.NewBuffer(nil)
			go func() {
				err := rotor.ServeHTTP(inr, out, tc.h, tc.conf)
				if tc.serveErr && err == nil {
					t.Error("Expected serve to fail, but it didnt")
				} else if !tc.serveErr && err != nil {
					t.Errorf("Expected serve not to fail, but it did: %v", err)
				}
			}()

			_, err := inw.Write([]byte(tc.input))
			if err != nil {
				t.Fatalf("Failed to write test case input: %v", err)
			}

			time.Sleep(time.Millisecond) //@TODO find something better here

			line, err := out.ReadString(0x0A) // `\n`
			if err != nil && err != io.EOF {
				t.Fatalf("Failed to read test case output: %v", err)
			}

			line = strings.TrimSpace(line)
			if line != tc.output {
				t.Errorf("Test case output of input '%s' should have been '%s', but got: '%s'", tc.input, tc.output, line)
			}
		})
	}

}
