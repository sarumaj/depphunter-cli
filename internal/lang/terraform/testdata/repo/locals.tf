locals {
  name = "web-${random_id.suffix.hex}"
  /* a comment with aws_instance.fake inside */
  policy = file("policy.json") # aws_instance.also_fake
}
