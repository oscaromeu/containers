package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

func Test(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/bitcoind:rolling")
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "bitcoind", "--version")
}
