provider "aws" {}

locals {
  name = "flux-test-${var.rand}"
}

module "eks" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/eks?ref=tf-resource-tags"

  name = local.name
  tags = var.tags
}

module "test_ecr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr?ref=tf-resource-tags"

  name = "test-repo-${local.name}"
  tags = var.tags
}

module "test_app_ecr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr?ref=tf-resource-tags"

  name = "test-app-${local.name}"
  tags = var.tags
}
