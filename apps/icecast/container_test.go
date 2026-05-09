package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

func TestIcecastVersion(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/icecast:rolling")
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "icecast", "-v")
}

func TestEntrypointExists(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/icecast:rolling")
	testhelpers.TestFileExists(t, ctx, image, "/entrypoint.sh", nil)
}

func TestStatusEndpoint(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/icecast:rolling")
	testhelpers.TestHTTPEndpoint(t, ctx, image,
		testhelpers.HTTPTestConfig{Port: "8000"},
		&testhelpers.ContainerConfig{
			Env: map[string]string{
				"ICECAST_SOURCE_PASSWORD": "test",
				"ICECAST_RELAY_PASSWORD":  "test",
				"ICECAST_ADMIN_PASSWORD":  "test",
			},
		},
	)
}
