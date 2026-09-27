resource "aws_subnet" "this" {
  cidr_block = var.cidr
  vpc_id     = aws_vpc.this.id
}

resource "aws_vpc" "this" {
  cidr_block = var.cidr

  dynamic "tag" {
    for_each = var.tags
    content {
      key   = tag.key
      value = tag.value
    }
  }
}

resource "aws_iam_policy" "p" {
  policy = file("${path.module}/policy.json")
}
