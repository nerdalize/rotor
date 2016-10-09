package rotor

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServeHTTP(t *testing.T) {
	testCases := []struct {
		input    string
		output   string
		serveErr bool
		h        http.Handler
	}{
		{`{}`, `{"error":"failed to handle input: decoded input has no event key"}`, false, nil},
		{`{aaa}`, `{"error":"failed to decode input: invalid character 'a' looking for beginning of object key string"}`, true, nil},
		{`{"event":{}}`, `{"value":{"statusCode":404,"body":"404 Not Found","headers":null}}`, false, nil},
		{`{"event":{}}`, `{"value":{"statusCode":200,"body":"","headers":{}}}`, false, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})},
		{`{"event":{"resource": 123}}`, `{"error":"failed to handle input: failed to unmarshal '{\"resource\": 123}' as proxy event: json: cannot unmarshal number into Go value of type string"}`, false, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})},
	}

	for _, tc := range testCases {
		t.Run("input="+tc.input, func(t *testing.T) {

			inr, inw := io.Pipe()
			out := bytes.NewBuffer(nil)
			go func() {
				err := ServeHTTP(inr, out, tc.h)
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
