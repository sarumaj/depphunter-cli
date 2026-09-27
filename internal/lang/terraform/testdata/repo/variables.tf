variable "region" {
  type = string
}

variable "cidr" {
  default = "10.0.0.0/16"
}

variable "subnets" {
  type    = list(object({ name = string, id = string }))
  default = []
}
