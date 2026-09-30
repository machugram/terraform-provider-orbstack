terraform {
  required_providers {
    orbstack = {
      source = "machugram/orbstack"
    }
  }
}

provider "orbstack" {}

resource "orbstack_machine" "live" {
  name       = "tf-live-demo"
  image      = "ubuntu:noble"
  cpus       = 1
  memory_mib = 2048
  disk_gib   = 8
}

resource "orbstack_container" "live" {
  name  = "tf-live-web"
  image = "nginx:alpine"

  restart = "always"

  ports = {
    "18443" = "80"
  }

  env = {
    LIVE = "create"
  }
}
