// Package rotor is library that makes it trivial to create Lambda functions written in
// Go that serve HTTP for the AWS API Gateway configured with the AWS_PROXY integration type
//
// Specifically, it allows for using any http.Handler implementation from the
// Go ecosystem in a serverless setup.
//
// Author: Ad van der Veer
package rotor
