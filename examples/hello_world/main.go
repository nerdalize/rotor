package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/advanderveer/go-dynamo"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/nerdalize/rotor"
)

//GatewayHandler handles events from the API gateway stream, in this hello world exeample it simply returns a scan of the table referenced through Terraform as 'my-table-name'
var GatewayHandler = rotor.NewGatewayHandler(0, http.HandlerFunc(
	func(w http.ResponseWriter, r *http.Request) {
		sess := rotor.RuntimeSession(r.Context())
		tname := rotor.ResourceAttribute(r.Context(), "my-table-name")
		db := dynamodb.New(sess)

		items := []interface{}{}
		if _, err := dynamo.NewScan(tname).Execute(db, items); err != nil {
			fmt.Fprintf(w, `{"error": "%s"}`, err.Error())
			return
		}

		enc := json.NewEncoder(w)
		err := enc.Encode(items)
		if err != nil {
			fmt.Fprintf(w, `{"error": "%s"}`, err.Error())
			return
		}
	}))

//Handle will route lambda events to the specific handler
func Handle(msg json.RawMessage, invoc *rotor.Invocation) (interface{}, error) {
	mux := rotor.NewMux()
	mux.MatchARN(regexp.MustCompile(`-gateway$`), GatewayHandler)

	mux.Use(rotor.EarlyTimeout(5000))   //gives handlers 5sec to clean up
	mux.Use(rotor.WithRuntimeSession()) //adds an aws session to the context
	mux.Use(rotor.ResourceAttributes()) //adds Terraform resource attributes

	return mux.Handle(msg, invoc)
}
