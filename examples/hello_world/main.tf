variable "version" {}
variable "owner" {}
variable "project" {}
variable "region_1" { default = "eu-west-1" }
provider "aws" {
  region = "${var.region_1}"
}

//A deployment groups and prefixes all resources
module "deployment" {
  source = "github.com/nerdalize/rotor//modules/deployment"
  version = "${var.version}"
  owner = "${var.owner}"
  project = "${var.project}"
}

//All observers (Lambda function) are executed with the same environment that comes with a special runtime AWS user
module "env" {
  source = "github.com/nerdalize/rotor//modules/env"
  deployment = "${module.deployment.id}"
  region = "${var.region_1}"
  permissions = "${data.aws_iam_policy_document.observers.json}"
  resource_attributes = {
    "my-table-name" = "${aws_dynamodb_table.t1.name}"
    "my-table-name-idx-a" = "${lookup(aws_dynamodb_table.t1.global_secondary_index[0], "name")}"
  }
}

//The lambda function runtime user will only have these permissions
data "aws_iam_policy_document" "observers" {
  policy_id = "${module.deployment.id}-observers"

  statement {
    actions = [
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:UpdateItem",
      "dynamodb:DeleteItem",
      "dynamodb:Query",
      "dynamodb:Scan"
    ]
    resources = [
      "${aws_dynamodb_table.t1.arn}*"
    ]
  }
}

//by outputting the exact environment as provided to the Lambda functions it becomes possible to simulate it locally
output "env" {
  value = "${module.env.vars}"
}


//
// Resources
//

resource "aws_dynamodb_table" "t1" {
  name           = "${module.deployment.id}-GameScores"
  read_capacity  = 1
  write_capacity = 1
  hash_key       = "UserId"
  range_key      = "GameTitle"

  attribute {
    name = "UserId"
    type = "S"
  }

  attribute {
    name = "GameTitle"
    type = "S"
  }

  attribute {
    name = "TopScore"
    type = "N"
  }

  global_secondary_index {
    name               = "idx1"
    hash_key           = "GameTitle"
    range_key          = "TopScore"
    write_capacity     = 1
    read_capacity      = 1
    projection_type    = "INCLUDE"
    non_key_attributes = ["UserId"]
  }
}

resource "aws_api_gateway_deployment" "main" {
  depends_on = ["module.gateway_stream"]
  rest_api_id = "${module.gateway_stream.api_id}"
  stage_name = "default"
}

output "endpoint" {
  value = "https://${module.gateway_stream.api_id}.execute-api.${var.region_1}.amazonaws.com/${aws_api_gateway_deployment.main.stage_name}"
}


//
// Observers
//

module "gateway_observer" {
  source = "github.com/nerdalize/rotor//modules/lambda"
  region = "${var.region_1}"
  deployment = "${module.deployment.id}"

  name = "gateway"
  package = "handler.zip"
  env = "${module.env.vars}"
}


//
// Streams
//

module "gateway_stream" {
  source = "github.com/nerdalize/rotor//modules/gateway/proxy"
  region = "${var.region_1}"
  deployment = "${module.deployment.id}"

  name = "gateway"
  observer = "${module.gateway_observer.arn}"
}
