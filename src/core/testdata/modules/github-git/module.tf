module "disk_encryption_set" {
  source      = "git::https://github.com/JamesWoolfenden/terraform-azurerm-diskencryptionset.git?ref=fc0b830997dd820476a7ad5e4b6ef2dcbdc766d7"
  common_tags = var.common_tags
  location    = var.location
  rg_name     = var.resource_group_name
}
