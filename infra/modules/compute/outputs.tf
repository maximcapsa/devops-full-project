output "server_public_ip" {
  description = "Elastic IP — point DNS / browser / kubectl here"
  value       = aws_eip.server.public_ip
}

output "server_private_ip" {
  value = aws_instance.server.private_ip
}

output "server_instance_id" {
  description = "For SSM Session Manager: aws ssm start-session --target <id>"
  value       = aws_instance.server.id
}

output "kubeconfig_ssm_parameter" {
  description = "SecureString parameter holding the kubeconfig (server rewrites 127.0.0.1 -> EIP)"
  value       = "/${var.name}/k3s/kubeconfig"
}
