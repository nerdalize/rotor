package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/nerdalize/rotor"
)

func setLambdaEnv(tb testing.TB, name string) func() {
	cmd := exec.Command("terraform", "output", "-json", name)
	cmd.Stderr = os.Stderr
	pr, err := cmd.StdoutPipe()
	if err != nil {
		tb.Fatalf("failed to create terraform stdout pipe: %+v", err)
	}

	go cmd.Run()

	v := struct {
		Value map[string]string `json:"value"`
	}{}

	dec := json.NewDecoder(pr)
	err = dec.Decode(&v)
	if err != nil {
		tb.Fatalf("failed to decode terraform output: %+v", err)
	}

	for k, v := range v.Value {
		os.Setenv(k, v)
	}

	return func() {
		for k := range v.Value {
			os.Unsetenv(k)
		}
	}
}

func TestGateway(t *testing.T) {
	reset := setLambdaEnv(t, "env")
	defer reset()

	res, err := Handle([]byte(`{}`), &rotor.Invocation{
		InvokedFunctionARN:    "test:arn-gateway",
		RemainingTimeInMillis: func() int64 { return 3000 },
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(fmt.Sprintf("%#v", res), "200") {
		t.Errorf("expected result to contain http OK, got: %#v", res)
	}
}
