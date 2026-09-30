resource "orbstack_machine" "dev" {
  name       = "dev"
  image      = "ubuntu:noble"
  cpus       = 2
  memory_mib = 2048
  disk_gib   = 16

  cloud_init = <<-EOF
    #cloud-config
    packages:
      - git
  EOF
}
