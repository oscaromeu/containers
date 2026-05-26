target "docker-metadata-action" {}

variable "APP" {
  default = "alertmanager-webhook-logger"
}

# Source is vendored under ./src/ — bump this when you re-vendor from upstream.
variable "VERSION" {
  default = "1.0"
}

variable "SOURCE" {
  default = "https://github.com/oscaromeu/containers/tree/main/apps/alertmanager-webhook-logger"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  inherits = ["docker-metadata-action"]
  args = {
    VERSION = "${VERSION}"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
  }
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
  tags = ["${APP}:${VERSION}"]
}

target "image-all" {
  inherits = ["image"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
}
