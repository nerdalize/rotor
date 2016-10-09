//go:generate rotorgen build.zip
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/nerdalize/rotor/rotor"
)

var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "hello from Rotor!")
})

func main() {
	log.Fatal(rotor.ServeHTTP(os.Stdin, os.Stdout, handler))
}
