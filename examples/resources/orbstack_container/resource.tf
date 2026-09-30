resource "orbstack_container" "web" {
  name  = "web"
  image = "nginx:latest"

  ports   = { "8080" = "80" }
  env     = { NGINX_HOST = "localhost" }
  volumes = { "/tmp/site" = "/usr/share/nginx/html" }
}
