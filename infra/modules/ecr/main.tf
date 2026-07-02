# One ECR repo per service (arm64 images). A lifecycle policy keeps the last
# 10 images so storage cost stays near zero.

resource "aws_ecr_repository" "this" {
  for_each = toset(var.services)

  name                 = "${var.name}/${each.value}"
  image_tag_mutability = "MUTABLE"
  force_delete         = true # make destroy must not be blocked by images

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = var.tags
}

resource "aws_ecr_lifecycle_policy" "keep_last_10" {
  for_each = aws_ecr_repository.this

  repository = each.value.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "keep last 10 images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 10
      }
      action = { type = "expire" }
    }]
  })
}
