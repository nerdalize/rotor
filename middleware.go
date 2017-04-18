package rotor

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

//EarlyTimeout will cancel the lambda context millisecond before actual timeout to give handlers time to shutdown cleanly
func EarlyTimeout(shutdownTimeMillis int64) func(Handler) Handler {
	return func(h Handler) Handler {
		return HandlerFunc(
			func(ctx context.Context, msg json.RawMessage) (interface{}, error) {
				lctx, ok := InvocationFromContext(ctx)
				if !ok {
					return h.HandleEvent(ctx, msg)
				}

				ctx, cancel := context.WithTimeout(ctx, time.Millisecond*time.Duration(lctx.RemainingTimeInMillis()-5000))
				defer cancel()
				return h.HandleEvent(ctx, msg)
			})
	}
}

//RuntimeSession will return a aws session from the lambda context or panic if it isn't available
func RuntimeSession(ctx context.Context) *session.Session {
	sess, ok := ctx.Value(invocationCtxKey).(*session.Session)
	if !ok {
		panic("no aws session available in context")
	}

	return sess
}

//WithRuntimeSession will include a line specific "runtime" aws.Session into the context that is not based on native lambda credentials
func WithRuntimeSession() func(Handler) Handler {
	region := os.Getenv("LINE_AWS_REGION")
	accessKey := os.Getenv("LINE_AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("LINE_AWS_SECRET_ACCESS_KEY")
	if region == "" || accessKey == "" || secretKey == "" {
		panic("cannot use the RuntimeSession middleware without all of the following environment variables: LINE_AWS_REGION, LINE_AWS_ACCESS_KEY_ID, LINE_AWS_SECRET_ACCESS_KEY")
	}

	sess, err := session.NewSession(
		&aws.Config{
			Region: aws.String(region),
			Credentials: credentials.NewStaticCredentials(
				accessKey,
				secretKey,
				"",
			),
		},
	)
	if err != nil {
		panic("failed to setup AWS session: " + err.Error())
	}

	return func(h Handler) Handler {
		return HandlerFunc(
			func(ctx context.Context, msg json.RawMessage) (interface{}, error) {
				ctx = context.WithValue(ctx, invocationCtxKey, sess)
				return h.HandleEvent(ctx, msg)
			})
	}
}

//ResourceAttribute will retrieve a resource attribute from the context or return an empty string if not found
func ResourceAttribute(ctx context.Context, name string) string {
	attrs, ok := ctx.Value(attributesCtxKey).(map[string]string)
	if !ok {
		return ""
	}

	val, ok := attrs[name]
	if !ok {
		return ""
	}
	return val
}

//ResourceAttributes middelware will read a line specific json encoded map from the environment and make it available for reading in handlers
func ResourceAttributes() func(Handler) Handler {
	attributesData := os.Getenv("LINE_RESOURCE_ATTRIBUTES")
	if attributesData == "" {
		panic("cannot use the ResourceAttributes middleware without all of the following environment variables: LINE_RESOURCE_ATTRIBUTES")
	}

	attributes := map[string]string{}
	err := json.Unmarshal([]byte(attributesData), &attributes)
	if err != nil {
		panic("failed to unmarshal LINE_RESOURCE_ATTRIBUTES env, is it valid JSON?")
	}

	return func(h Handler) Handler {
		return HandlerFunc(
			func(ctx context.Context, msg json.RawMessage) (interface{}, error) {
				ctx = context.WithValue(ctx, attributesCtxKey, attributes)
				return h.HandleEvent(ctx, msg)
			})
	}
}
