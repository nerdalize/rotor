variable "aws_region" {}

//Name of the function role that is created
variable "role_function_name" {
  default = "rotor_function"
}

//Name of the API Gateway API
variable "api_name" {}

//Description of the API Gateway API
variable "api_description" {}

//Name of the Lambda function that handles all API calls
variable "func_name" {}

//Description of the Lambda function that handles all API Calls
variable "func_description" {}

//Path to the zipped Lambda function
variable "func_zip_path" {}

//Policy that describes permissions of the Lambda function
variable "func_policy" {
  default = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "logs:*"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

//Current account ID used for creating Lambda invoke policy
data "aws_caller_identity" "current" {}

//Outputs the rest API for other resources to integrate with
output "rest_api_id" {
  value = "${aws_api_gateway_rest_api.rotor.id}"
}

//this allows the stage to wait for the resource to be actually integration
output "aws_api_gateway_method" {
  value = "${aws_api_gateway_integration.proxy_ANY_integration.resource_id}"
}

#lambda function needs a role that is able to use other AWS services
resource "aws_iam_role" "rotor_function" {
  name = "${var.role_function_name}"
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

resource "aws_iam_role_policy" "rotor_function_policy" {
    name = "RotorLambdaFunctionPolicy"
    role = "${aws_iam_role.rotor_function.id}"
    policy = "${var.func_policy}"
}

#the lambda function that will handle all API calls
resource "aws_lambda_function" "rotor-api_prod_all" {
  function_name = "${var.func_name}"
  description = "${var.func_description}"
  filename = "${var.func_zip_path}"

  role = "${aws_iam_role.rotor_function.arn}" //role the function assumes
  handler = "index.handler" //index.js exports.handler
  source_code_hash = "${base64sha256(file(var.func_zip_path))}"
  runtime = "nodejs4.3"
}

//start of the API gateway
resource "aws_api_gateway_rest_api" "rotor" {
  name = "${var.api_name}"
  description = "${var.api_description}"
}

//a single proxy resource
resource "aws_api_gateway_resource" "proxy" {
  rest_api_id = "${aws_api_gateway_rest_api.rotor.id}"
  parent_id = "${aws_api_gateway_rest_api.rotor.root_resource_id}"
  path_part = "{proxy+}"
}

//a single ANY methods
resource "aws_api_gateway_method" "proxy_ANY" {
  rest_api_id = "${aws_api_gateway_rest_api.rotor.id}"
  resource_id = "${aws_api_gateway_resource.proxy.id}"
  http_method = "ANY"
  authorization = "NONE"
  request_parameters = {
    "method.request.path.proxy" = true
  }
}

//proxy integration for the Lambda function
resource "aws_api_gateway_integration" "proxy_ANY_integration" {
  rest_api_id = "${aws_api_gateway_rest_api.rotor.id}"
  resource_id = "${aws_api_gateway_resource.proxy.id}"
  http_method = "ANY"
  type = "AWS_PROXY"
  uri = "arn:aws:apigateway:${var.aws_region}:lambda:path/2015-03-31/functions/${aws_lambda_function.rotor-api_prod_all.arn}/invocations"
  integration_http_method = "POST"
}

//we use resource-based permission for gateway invokation to shave of 90ms latency
resource "aws_lambda_permission" "allow_gateway" {
    statement_id = "AllowExecutionFromRotorGateway"
    action = "lambda:InvokeFunction"
    function_name = "${aws_lambda_function.rotor-api_prod_all.function_name}"
    principal = "apigateway.amazonaws.com"
    source_arn = "arn:aws:execute-api:eu-west-1:${data.aws_caller_identity.current.account_id}:${aws_api_gateway_rest_api.rotor.id}/*/*/*"
}
