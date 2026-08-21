package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

func Test(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/allquiet-sync:rolling")
	// scratch image: no shell nor coreutils, so running the binary itself is
	// both the existence check and the smoke test.
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "/app/allquiet-sync", "-h")
}
