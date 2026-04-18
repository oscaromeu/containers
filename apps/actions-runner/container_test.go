package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

func Test(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/actions-runner:rolling")
	testhelpers.TestFileExists(t, ctx, image, "/usr/local/bin/yq", nil)
}
