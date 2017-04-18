variable "region" {}
variable "deployment" {}
variable "name" {}
variable "package" {}
variable "timeout" {
  default = "10"
}

variable "memory" {
  default = "128"
}

variable "env" {
  type = "map"
  default = {}
}

variable "resource_attributes" {
  type = "map"
  default = {}
}

//the default policy json is bogus and wont provide give any permission
variable "permissions" {
  default = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": ["logs:DescribeLogStreams"],
      "Effect": "Allow",
      "Resource": "arn:aws:logs:placeholder:*"
    }
  ]
}
EOF
}

provider "aws" {
  region = "${var.region}"
}

data "aws_caller_identity" "current" {}
resource "aws_iam_role" "lambda" {
  name = "${var.deployment}-lambda-${var.name}"
  assume_role_policy = "${data.aws_iam_policy_document.lambda_assume.json}"
}

resource "aws_iam_role_policy" "lambda" {
  name = "${var.deployment}-lambda-${var.name}"
  role = "${aws_iam_role.lambda.id}"
  policy = "${data.aws_iam_policy_document.lambda.json}"
}

data "aws_iam_policy_document" "lambda_assume" {
  policy_id = "${var.deployment}-lambda-assume=${var.name}"
  statement {
    actions = [ "sts:AssumeRole" ]
    principals {
      type = "Service"
      identifiers = [
        "lambda.amazonaws.com",
        "states.${var.region}.amazonaws.com"
      ]
    }
  }
}

data "aws_iam_policy_document" "lambda" {
  policy_id = "${var.deployment}-lambda"

  //allow role to write stdin/out to cloudwatch log groups
  statement {
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams"
    ]
    resources = [
      "arn:aws:logs:${var.region}:${data.aws_caller_identity.current.account_id}:log-group:/aws/lambda/${var.deployment}*"
    ]
  }
}

//
// runtime user for this observer
//

resource "aws_iam_user" "runtime" {
  force_destroy = true
  name = "${var.deployment}-runtime-${var.name}"
  path = "/${var.deployment}-${var.name}/"
}

resource "aws_iam_user_policy" "runtime" {
  name = "${var.deployment}-runtime-${var.name}"
  user = "${aws_iam_user.runtime.name}"
  policy = "${var.permissions}"
}

resource "aws_iam_access_key" "runtime" {
  user    = "${aws_iam_user.runtime.name}"
}

//
// local env variables will be merged with global
//

data "template_file" "env" {
  template = ""
  vars {
    "LINE_DEPLOYMENT" = "${var.deployment}"
    "LINE_RESOURCE_ATTRIBUTES" = "${jsonencode(var.resource_attributes)}"
    "LINE_AWS_REGION" = "${var.region}"
    "LINE_AWS_ACCESS_KEY_ID" = "${aws_iam_access_key.runtime.id}"
    "LINE_AWS_SECRET_ACCESS_KEY" = "${aws_iam_access_key.runtime.secret}"
  }
}

//
// runtime configuration for this observer
//

resource "aws_lambda_function" "func" {
  function_name = "${var.deployment}-${var.name}"
  filename = "${var.package}"
  source_code_hash = "${base64sha256(file("${var.package}"))}"
  role = "${aws_iam_role.lambda.arn}"

  timeout = "${var.timeout}"
  memory_size = "${var.memory}"
  handler = "handler.Handle"
  runtime = "python2.7"
  environment = {
    variables = "${merge(data.template_file.env.vars, var.env)}"
  }
}

resource "aws_cloudwatch_log_group" "gateway" {
    name = "/aws/lambda/${aws_lambda_function.func.function_name}"
    retention_in_days = 60
}

output "arn" {
  value = "${aws_lambda_function.func.arn}"
}

output "environment" {
  value = "${merge(data.template_file.env.vars, var.env)}"
}
