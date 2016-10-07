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

resource "aws_api_gateway_deployment" "api" {
  rest_api_id = "${module.api.rest_api_id}"
  stage_name = "test"
  stage_description = "test (${module.api.aws_api_gateway_method})" //THIS HACK IS MANDATORY
}

output "api_endpoint" {
  value = "https://${module.api.rest_api_id}.execute-api.${var.aws_region}.amazonaws.com/${aws_api_gateway_deployment.api.stage_name}"
}
