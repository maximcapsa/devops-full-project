# k3s cluster: 1 on-demand server (holds the Elastic IP -> Traefik ingress) +
# a spot agent ASG. Agents join via the k3s token published to SSM Parameter
# Store by the server's user-data. Node access is SSM Session Manager only —
# no SSH, no port 22, no key pairs.

data "aws_ssm_parameter" "al2023_arm64" {
  name = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64"
}

locals {
  token_param      = "/${var.name}/k3s/token"
  kubeconfig_param = "/${var.name}/k3s/kubeconfig"
}

# ── Node IAM: SSM agent, ECR pulls, and the k3s-token/kubeconfig params ──────
resource "aws_iam_role" "node" {
  name_prefix = "${var.name}-node-"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "ssm_core" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy_attachment" "ecr_read" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

data "aws_caller_identity" "current" {}

resource "aws_iam_role_policy" "cluster_params" {
  name_prefix = "cluster-params-"
  role        = aws_iam_role.node.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["ssm:PutParameter", "ssm:GetParameter"]
      Resource = "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/${var.name}/k3s/*"
    }]
  })
}

resource "aws_iam_instance_profile" "node" {
  name_prefix = "${var.name}-node-"
  role        = aws_iam_role.node.name
  tags        = var.tags
}

# ── Elastic IP: stable public entry point, referenced in the server TLS SAN ──
resource "aws_eip" "server" {
  domain = "vpc"
  tags   = merge(var.tags, { Name = "${var.name}-server" })
}

# ── k3s server (on-demand: it is the control plane AND the ingress node) ─────
resource "aws_instance" "server" {
  ami                    = nonsensitive(data.aws_ssm_parameter.al2023_arm64.value)
  instance_type          = var.server_instance_type
  subnet_id              = var.subnet_ids[0]
  vpc_security_group_ids = [var.node_security_group_id]
  iam_instance_profile   = aws_iam_instance_profile.node.name

  root_block_device {
    volume_type = "gp3"
    volume_size = var.root_volume_gb
  }

  user_data = templatefile("${path.module}/user_data/server.sh.tpl", {
    eip              = aws_eip.server.public_ip
    region           = var.region
    token_param      = local.token_param
    kubeconfig_param = local.kubeconfig_param
  })

  tags = merge(var.tags, { Name = "${var.name}-server", Role = "k3s-server" })

  lifecycle {
    ignore_changes = [ami] # don't replace the control plane on AMI refreshes
  }
}

resource "aws_eip_association" "server" {
  instance_id   = aws_instance.server.id
  allocation_id = aws_eip.server.id
}

# ── k3s agents: spot ASG (cheap, interruptible — that's the HA demo) ─────────
resource "aws_launch_template" "agent" {
  name_prefix   = "${var.name}-agent-"
  image_id      = nonsensitive(data.aws_ssm_parameter.al2023_arm64.value)
  instance_type = var.agent_instance_type

  iam_instance_profile {
    name = aws_iam_instance_profile.node.name
  }

  vpc_security_group_ids = [var.node_security_group_id]

  instance_market_options {
    market_type = "spot"
    spot_options {
      spot_instance_type = "one-time"
    }
  }

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_type = "gp3"
      volume_size = var.root_volume_gb
    }
  }

  user_data = base64encode(templatefile("${path.module}/user_data/agent.sh.tpl", {
    server_ip   = aws_instance.server.private_ip
    region      = var.region
    token_param = local.token_param
  }))

  tag_specifications {
    resource_type = "instance"
    tags          = merge(var.tags, { Name = "${var.name}-agent", Role = "k3s-agent" })
  }

  tags = var.tags
}

resource "aws_autoscaling_group" "agents" {
  name_prefix         = "${var.name}-agents-"
  min_size            = var.agent_count
  max_size            = var.agent_count + 1
  desired_capacity    = var.agent_count
  vpc_zone_identifier = var.subnet_ids

  launch_template {
    id      = aws_launch_template.agent.id
    version = "$Latest"
  }

  dynamic "tag" {
    for_each = var.tags
    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = true
    }
  }
}
