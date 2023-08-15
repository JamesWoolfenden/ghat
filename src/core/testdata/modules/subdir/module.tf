module "subdir" {
  source = "git::https://github.com/hashicorp/terraform-aws-consul.git//modules/consul-cluster?ref=e9ceb573687c3d28516c9e3714caca84db64a766"
}
