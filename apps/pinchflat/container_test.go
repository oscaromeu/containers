package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

const defaultImage = "ghcr.io/oscaromeu/pinchflat:rolling"

func TestYtDlp(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "yt-dlp", "--version")
}

func TestDeno(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "deno", "--version")
}

func TestHealthcheckEndpoint(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestHTTPEndpoint(t, ctx, image,
		testhelpers.HTTPTestConfig{Port: "8945", Path: "/healthcheck"},
		nil,
	)
}
