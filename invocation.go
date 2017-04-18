package rotor

import "context"

//Invocation provides additional information about a lambda event
type Invocation struct {
	FunctionName          string       `json:"function_name"`
	FunctionVersion       string       `json:"function_version"`
	InvokedFunctionARN    string       `json:"invoked_function_arn"`
	MemoryLimitInMB       int          `json:"memory_limit_in_mb,string"`
	AWSRequestID          string       `json:"aws_request_id"`
	LogGroupName          string       `json:"log_group_name"`
	LogStreamName         string       `json:"log_stream_name"`
	RemainingTimeInMillis func() int64 `json:"-"`
}

//InvocationFromContext returns the original invocation context from a generic context
func InvocationFromContext(ctx context.Context) (*Invocation, bool) {
	inv, ok := ctx.Value(invocationCtxKey).(*Invocation)
	return inv, ok
}
