variable "region" {}
variable "deployment" {}
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

//
// A limited permission runtime user
//

resource "aws_iam_user" "runtime" {
  force_destroy = true
  name = "${var.deployment}-runtime"
  path = "/${var.deployment}/"
}

resource "aws_iam_user_policy" "runtime" {
  name = "${var.deployment}-runtime"
  user = "${aws_iam_user.runtime.name}"
  policy = "${var.permissions}"
}

resource "aws_iam_access_key" "runtime" {
  user    = "${aws_iam_user.runtime.name}"
}

//
// The environment variables
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

output "vars" {
  value = "${data.template_file.env.vars}"
}
