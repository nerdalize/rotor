variable "name" {}
variable "region" {}
variable "deployment" {}
variable "observer" {}

provider "aws" {
  region = "${var.region}"
}

variable "expression" {
  default = "rate(1 minute)"
}

resource "aws_cloudwatch_event_rule" "tick" {
  name        = "${var.deployment}-${var.name}"
  description = "An observable stream of clock ticks at a regular interval"
  schedule_expression = "${var.expression}"
}

resource "aws_cloudwatch_event_target" "target" {
  rule  = "${aws_cloudwatch_event_rule.tick.name}"
  arn   = "${var.observer}"
}

resource "aws_lambda_permission" "allow_event" {
  statement_id = "${var.deployment}-event-${count.index}"
  action = "lambda:InvokeFunction"
  function_name = "${var.observer}"
  principal = "events.amazonaws.com"
  source_arn = "${aws_cloudwatch_event_rule.tick.arn}"
}
