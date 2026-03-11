###############################################################################
# prod environment root module
###############################################################################

terraform {
  required_version = ">= 1.7"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket         = "terraform-state-154845614889"
    key            = "prod/scores-api/terraform.tfstate"
    region         = "eu-central-1"
    encrypt        = true
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = local.tags
  }
}

locals {
  name = "${var.project}-${var.environment}"

  tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}

# ── Networking ────────────────────────────────────────────────────────────────
module "vpc" {
  source = "../../modules/vpc"

  name = local.name
  cidr = var.vpc_cidr
  tags = local.tags
}

# ── Container registry ────────────────────────────────────────────────────────
module "ecr" {
  source = "../../modules/ecr"

  name = local.name
  tags = local.tags
}

# ── DNS (Route 53) ────────────────────────────────────────────────────────────
# Kept as a resource (not data source) to match the existing Terraform state.
resource "aws_route53_zone" "this" {
  name = var.route53_zone_name
}

# ── TLS certificate (must be in us-east-1 for ACM + ALB; same region is fine) ─
resource "aws_acm_certificate" "api" {
  domain_name       = var.api_domain
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

# ── DNS validation records for ACM certificate ───────────────────────────────
resource "aws_route53_record" "acm_validation" {
  for_each = {
    for dvo in aws_acm_certificate.api.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }

  allow_overwrite = true
  name            = each.value.name
  records         = [each.value.record]
  ttl             = 60
  type            = each.value.type
  zone_id         = aws_route53_zone.this.zone_id
}

resource "aws_acm_certificate_validation" "api" {
  certificate_arn         = aws_acm_certificate.api.arn
  validation_record_fqdns = [for record in aws_route53_record.acm_validation : record.fqdn]
}

# ── Load balancer ─────────────────────────────────────────────────────────────
module "alb" {
  source = "../../modules/alb"

  name              = local.name
  vpc_id            = module.vpc.vpc_id
  public_subnet_ids = module.vpc.public_subnet_ids
  certificate_arn   = aws_acm_certificate_validation.api.certificate_arn
  tags              = local.tags
}

# ── ECS Fargate service ───────────────────────────────────────────────────────
module "ecs" {
  source = "../../modules/ecs"

  name                  = local.name
  vpc_id                = module.vpc.vpc_id
  private_subnet_ids    = module.vpc.private_subnet_ids
  alb_security_group_id = module.alb.security_group_id
  target_group_arn      = module.alb.target_group_arn
  image_uri             = "${module.ecr.repository_url}:${var.image_tag}"

  task_cpu      = var.task_cpu
  task_memory   = var.task_memory
  desired_count = var.desired_count

  tags = local.tags

  # Ensure ALB listeners are fully created before ECS service registers with target group
  depends_on = [module.alb]
}

# ── DNS alias record (API → ALB) ─────────────────────────────────────────────
resource "aws_route53_record" "api" {
  zone_id = aws_route53_zone.this.zone_id
  name    = var.api_domain
  type    = "A"

  alias {
    name                   = module.alb.alb_dns_name
    zone_id                = module.alb.alb_zone_id
    evaluate_target_health = true
  }
}
