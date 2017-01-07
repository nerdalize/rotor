# rotor [![GoDoc](https://godoc.org/github.com/nerdalize/rotor/rotor?status.svg)](https://godoc.org/github.com/nerdalize/rotor/rotor)

_Rotor_ is a minimalistic toolset that makes it easy to run HTTP-serving logic written in Go for a serverless setup using [AWS Lambda](http://docs.aws.amazon.com/lambda/latest/dg/welcome.html) and the [API Gateway](https://aws.amazon.com/api-gateway/). It comes with the following:

 - A Go library that can be used to make any implementation of [http.Handler](https://golang.org/pkg/net/http/#Handler) serve its HTTP output from a Lambda function.
 - A [go generator](https://blog.golang.org/generate) that builds and packages your Go program into a .zip file that AWS Lambda expects by wrapping the executable with a NodeJS script.
 - A [Terraform module](https://www.terraform.io/docs/modules/usage.html) that uploads the .zip package and creates the nessesary API Gateway resources to proxy requests to the Lambda function using the AWS_PROXY integration.

## Getting started
To get started you'll need to have [Terraform](https://www.terraform.io/downloads.html) version >= 0.7.5 installed and available in your `$PATH`, you'll also need to have the Go 1.7 SDK installed with your `$GOPATH` correctly configured.  

1. Start a new Go project somewhere in your $GOPATH and create a main.go file with following content:

	```Go
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
	```

2. AWS Lambda doesn't natively support Go as a runtime but does support [other executables to be executed](aws.amazon.com/blogs/compute/running-executables-in-aws-lambda/) through a NodeJS wrapper. _Rotor_ comes with a [go generator](https://blog.golang.org/generate) thats compiles and wraps your go program before producing a zip file that can directly be uploaded to AWS. First install the generator:

	```shell
	go get -u github.com/nerdalize/rotor/rotorgen
	```

3. Make sure `$GOPATH/bin` in in your `$PATH` and then use the generator to create the `build.zip` file in your current working directory:

	```
	go generate ./main.go
	```


### Uploading and Creating the API Gateway
Publishing your HTTP service through an API Gateway requires requires access to AWS Credentials (for an example policy see below). Make sure you have your `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` ready (or set as an environment variable) before continuing.


1. To expose your HTTP service through the AWS API Gateway Rotor comes with a [Terraform module](https://www.terraform.io/docs/modules/usage.html). Create a `main.tf` file that looks like this:

	```hcl
	variable "aws_region" {
	  default = "eu-west-1" //You can set your preferred region here
	}

	provider "aws" {
	  region = "${var.aws_region}"
	}

	module "api" {
	  source = "github.com/nerdalize/rotor//rotortf"
	  aws_region = "${var.aws_region}"

	  func_name = "hello-api_all"
	  func_description = "A Rotor hello world function"
	  func_zip_path = "build.zip"

	  api_name = "Hello"
	  api_description = "A simple hello from Rotor"
	}
	```
2. You'll then need to fetch the Terraform module:

	```
	terraform get
	```

3. Make sure you have AWS credentials with the correct permissions and then apply the infrastructure

	```
	terraform apply
	```

### Publishing the API
To publish the API to the Internet we'll need to add a `aws_api_gateway_deployment` Terraform resource.

1. Re-open your `main.tf` and add the following:


	```hcl
	resource "aws_api_gateway_deployment" "api" {
	  rest_api_id = "${module.api.rest_api_id}"
	  stage_name = "test"
	  stage_description = "test (${module.api.aws_api_gateway_method})" //THIS HACK IS MANDATORY
	}

	output "api_endpoint" {
	  value = "https://${module.api.rest_api_id}.execute-api.${var.aws_region}.amazonaws.com/${aws_api_gateway_deployment.api.stage_name}"
	}
	```

	_NOTE: Unfortunately, this hack is necessary to make sure the deployment is not created before the actual resources and methods are available._


2. Then re-apply the infrastructre, the API endpoint should be printend to the screen.

	```
	terraform apply
	...
	api_endpoint=<your_endpoint>
	```

3. The API should now be reachable, without a path the gateway will return unauthorized but with _any_ path the request will be proxied to the Lambda function and handles by our Go code:

	```
	curl <your_endpoint>/foobar
	> hello from Rotor
	```

### Deploying New Changes

1. To publish changes you can simple re-run the generator to package and then re-apply using Terraform to deploy it:

	```
	go generate ./main.go
	```
	```
	terraform apply
	```


### Removing the AWS infrastructure

1. To clean up all AWS resources, simply run:

 ```
 terraform destroy
 ```

## What Nexts
The _Rotor_ tools aims to play well with your current workflow and other tools you might be using:

- _TODO: Guide: Integrating with Apex_
- _TODO: Guide: Customize build flags and adding other files to the zip_
- _TODO: Guide: Accessing the Raw Lambda event and context_
- _TODO: Reference: terraform module variables_


## Terraform AWS Policy
The Terraform module requires AWS Credentials with permissions to manage role policies, api gateway and lambda resources. A policy for such a user could look like:

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Stmt1476022435000",
            "Effect": "Allow",
            "Action": [
                "iam:AttachRolePolicy",
                "iam:CreatePolicy",
                "iam:CreateRole",
                "iam:DeletePolicy",
                "iam:DeleteRole",
                "iam:DeleteRolePolicy",
                "iam:DetachRolePolicy",
                "iam:GetPolicy",
                "iam:GetRole",
                "iam:GetRolePolicy",
                "iam:ListAttachedRolePolicies",
                "iam:ListInstanceProfilesForRole",
                "iam:ListRolePolicies",
                "iam:PassRole",
                "iam:PutRolePolicy",
                "lambda:AddPermission",
                "lambda:CreateFunction",
                "lambda:DeleteFunction",
                "lambda:GetFunction",
                "lambda:GetFunctionConfiguration",
                "lambda:ListVersionsByFunction",
                "lambda:GetFunctionConfiguration",
                "lambda:GetPolicy",
                "lambda:RemovePermission",
                "lambda:UpdateFunctionCode",
                "lambda:UpdateFunctionConfiguration",
                "apigateway:*"
            ],
            "Resource": [
                "*"
            ]
        }
    ]
}
```

### Using Rotor through the "external" provider
Rotor comes with the `rotorpkg` executable that can be used directly with Terraform's
[external provider](https://www.terraform.io/docs/providers/external) to package Go
lambda functions. To install, run:

```shell
go get -u github.com/nerdalize/rotor/rotorpkg
```

It can then used as in your terraform files like so:

```hcl
data "external" "lambda_pkg" {
  program = ["rotorpkg"]
  query = {
    dir = "${path.module}"
    ignore = ".git:*.tfstate:*.tfstate.backup:build.zip"
    output = "${path.module}/build.zip"
  }
}

resource "aws_iam_role" "func" {
  name = "my-func-role"
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF
}

resource "aws_lambda_function" "func" {
  filename = "${data.external.lambda_pkg.result.zip_filename}"
  source_code_hash = "${data.external.lambda_pkg.result.zip_base64sha256}"
  role = "${aws_iam_role.func.arn}"

  timeout = 3
  memory_size = 128
  function_name = "my-func"
  description = "my-func-description"
  handler = "index.handle"
  runtime = "nodejs4.3"
}
```
