variable "version" {}
variable "owner" {}
variable "project" {}

resource "random_id" "id" { byte_length = 4 }
data "template_file" "p" {
  template = "$${i}-$${p}$${v}-$${o}"
  vars {
    i = "${random_id.id.hex}"
    v = "${replace(lower(var.version),"/[^a-zA-Z0-9]/", "")}"
    p = "${replace(lower(var.project),"/[^a-zA-Z0-9]/", "")}"
    o = "${replace(replace(lower(var.owner),"/[^a-zA-Z0-9]/", ""), "/(.{0,5})(.*)/", "$1")}"
  }
}

output "id" {
  value = "${data.template_file.p.rendered}"
}
