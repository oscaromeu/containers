package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

func TestWebhookEndpoint(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/alertmanager-webhook-logger:rolling")
	testhelpers.TestHTTPEndpoint(t, ctx, image, testhelpers.HTTPTestConfig{
		Port:       "6725",
		StatusCode: 400,
	}, nil)
}
