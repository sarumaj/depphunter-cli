data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]
}

data "terraform_remote_state" "net" {
  backend = "local"
  config = {
    path = "${path.module}/state/net.tfstate"
  }
}
