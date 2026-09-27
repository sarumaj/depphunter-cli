output "ip" {
  value = aws_instance.web.public_ip
}

output "banner" {
  value = <<-EOT
    Hello ${local.name}
    $${not.interpolated}
  EOT
}

output "subnet" {
  value = module.network.subnet_id
}
