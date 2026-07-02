output "server_public_ip" {
  description = "Elastic IP: storefront/API entry + k8s API endpoint"
  value       = module.compute.server_public_ip
}

output "server_instance_id" {
  description = "SSM Session Manager target"
  value       = module.compute.server_instance_id
}

output "kubeconfig_ssm_parameter" {
  description = "Fetch with: aws ssm get-parameter --name <this> --with-decryption --query Parameter.Value --output text > kubeconfig"
  value       = module.compute.kubeconfig_ssm_parameter
}

output "ecr_repository_urls" {
  value = module.ecr.repository_urls
}

output "deploy_role_arn" {
  description = "AWS_DEPLOY_ROLE_ARN for GitHub Actions OIDC"
  value       = module.iam.deploy_role_arn
}
