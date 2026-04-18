package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

func Test(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/webhook:rolling")
	testhelpers.TestHTTPEndpoint(t, ctx, image, testhelpers.HTTPTestConfig{Port: "9000"}, nil)
}
