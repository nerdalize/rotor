# rotor

## Getting started
To get started you'll need to have [Terraform](https://www.terraform.io/downloads.html) version >= 0.7.5 installed and available in your `$PATH`, you'll also need to have the Go 1.7 SDK installed with your `$GOPATH` correctly configured.  

1. Start a new Go project somewhere in your $GOPATH and create a main.go file with following content:

	```Go
	//go:generate rotorgen build.zip
	package main
	
	import (
		"fmt"
		"net/http"
		"os"
	
		"github.com/nerdalize/rotor/rotor"
	)
	
	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello from Rotor")
	})
	
	func main() {
		rotor.Serve(os.Stdin, os.Stdout, handler)
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
	 

### Creating an API Gateway

1. To expose your HTTP server through the AWS API Gateway; Rotor comes with a [Terraform module](https://www.terraform.io/docs/modules/usage.html) to do this. Create a `main.tf` file that looks like this: 

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
To publish the API to the Internet we'll need to add a staging Terraform resources. 

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
	
	NOTE: The hack is necessary to make sure the stage is not created before the actual resources and methods are created.
	
	
2. Then re-apply the infrastructre, the API endpoint should be printend to the screen.

	```
	terraform apply 
	...
	api_endpoint=<your_endpoint>
	```
	
3. The API should now be reachable, make sure to add some path: 

	```
	curl <your_endpoint>
	> hello from Rotor
	```
	
4. To clean up all AWS resources, simply run:

	```
	terraform destroy
	```