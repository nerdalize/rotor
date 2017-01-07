package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHappyTempPath(t *testing.T) {
	dir, err := ioutil.TempDir("", "rotorpkg_")
	if err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(filepath.Join(dir, "main.go"), []byte(`
package main
func main() {}
`), 0666); err != nil {
		t.Fatal(err)
	}

	query := bytes.NewBufferString(fmt.Sprintf(`{"dir": "%s"}`, dir))
	result := bytes.NewBuffer(nil)
	logs := log.New(os.Stderr, "", log.LstdFlags)
	err = run(logs, query, result)
	if err != nil {
		t.Errorf("run of query '%x' shouldnt fail, got: %v", query, err)
	}

	if strings.Contains(result.String(), dir) {
		t.Errorf("output should not contain explicit dir, got: '%s'", result.String())
	}
}

func TestHappyExplicitPath(t *testing.T) {
	dir, err := ioutil.TempDir("", "rotorpkg_")
	if err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(filepath.Join(dir, "main.go"), []byte(`
package main
func main() {}
`), 0666); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(dir, "build.zip")
	query := bytes.NewBufferString(fmt.Sprintf(`{"dir": "%s", "output": "%s"}`, dir, output))
	result := bytes.NewBuffer(nil)
	logs := log.New(os.Stderr, "", log.LstdFlags)
	err = run(logs, query, result)
	if err != nil {
		t.Errorf("run of query '%x' shouldnt fail, got: %v", query, err)
	}

	if !strings.Contains(result.String(), output) {
		t.Errorf("output refer to explicit zip file, got: '%s'", result.String())
	}
}
