target "docker-metadata-action" {}

variable "APP" {
  default = "allquiet-sync"
}

# Temporary mirror of flanksio/containers#440 for the homelab PoC — bump this
# when re-syncing from the flnx branch.
variable "VERSION" {
  default = "1.0"
}

variable "SOURCE" {
  default = "https://github.com/oscaromeu/containers/tree/main/apps/allquiet-sync"
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
