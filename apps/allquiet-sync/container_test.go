package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

func Test(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/allquiet-sync:rolling")
	testhelpers.TestFileExists(t, ctx, image, "/app/allquiet-sync", nil)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "/app/allquiet-sync", "-h")
}
