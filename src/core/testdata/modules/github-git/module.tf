module "disk_encryption_set" {
  source      = "" #v0.0.7
  common_tags = var.common_tags
  location    = var.location
  rg_name     = var.resource_group_name
}
