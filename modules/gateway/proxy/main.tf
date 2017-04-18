variable "name" {}
variable "region" {}
variable "observer" {}
variable "deployment" {}

provider "aws" {
  region = "${var.region}"
}

data "aws_caller_identity" "current" {}

resource "aws_api_gateway_rest_api" "main" {
  name = "${var.deployment}-${var.name}"
}

resource "aws_api_gateway_method" "ANY_ROOT" {
  rest_api_id = "${aws_api_gateway_rest_api.main.id}"
  resource_id = "${aws_api_gateway_rest_api.main.root_resource_id}"
  http_method = "ANY"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "ANY_ROOT_integration" {
  rest_api_id = "${aws_api_gateway_rest_api.main.id}"
  resource_id = "${aws_api_gateway_rest_api.main.root_resource_id}"
  http_method = "${aws_api_gateway_method.ANY_ROOT.http_method}"
  type = "AWS_PROXY"
  uri = "arn:aws:apigateway:${var.region}:lambda:path/2015-03-31/functions/${var.observer}/invocations"
  integration_http_method = "POST"
}

resource "aws_api_gateway_resource" "proxy" {
  rest_api_id = "${aws_api_gateway_rest_api.main.id}"
  parent_id = "${aws_api_gateway_rest_api.main.root_resource_id}"
  path_part = "{proxy+}"
}

resource "aws_api_gateway_method" "ANY" {
  rest_api_id = "${aws_api_gateway_rest_api.main.id}"
  resource_id = "${aws_api_gateway_resource.proxy.id}"
  http_method = "ANY"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "ANY_integration" {
  rest_api_id = "${aws_api_gateway_rest_api.main.id}"
  resource_id = "${aws_api_gateway_resource.proxy.id}"
  http_method = "${aws_api_gateway_method.ANY.http_method}"
  type = "AWS_PROXY"
  uri = "arn:aws:apigateway:${var.region}:lambda:path/2015-03-31/functions/${var.observer}/invocations"
  integration_http_method = "POST"
}

resource "aws_lambda_permission" "allow_gateway" {
  statement_id = "AllowExecutionFromGateway"
  action = "lambda:InvokeFunction"
  function_name = "${var.observer}"
  principal = "apigateway.amazonaws.com"
  source_arn = "arn:aws:execute-api:${var.region}:${data.aws_caller_identity.current.account_id}:${aws_api_gateway_rest_api.main.id}/*/*/*"
}

output "api_id" {
  value = "${aws_api_gateway_rest_api.main.id}"
}
