output "deploy_role_arn" {
  description = "Set as AWS_DEPLOY_ROLE_ARN in GitHub Actions"
  value       = aws_iam_role.deploy.arn
}
