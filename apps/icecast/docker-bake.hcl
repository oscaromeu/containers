target "docker-metadata-action" {}

variable "APP" {
  default = "icecast"
}

variable "VERSION" {
  // renovate: datasource=repology depName=alpine_3_23/icecast
  default = "2.4.4"
}

variable "SOURCE" {
  default = "https://icecast.org/"
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
