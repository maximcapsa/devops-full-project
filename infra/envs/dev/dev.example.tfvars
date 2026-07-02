# Copy to dev.tfvars (gitignored) and fill in. Then:
#   terraform plan -var-file=dev.tfvars
region            = "us-east-1"
admin_cidr        = "0.0.0.0/0" # set to <your-ip>/32
github_repository = "maximcapsa/devops-full-project"
alert_email       = "you@example.com"
