package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

func TestPlaywrightInstalled(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/playwright-runner:rolling")
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "npx", "playwright", "--version")
}

// Runs the baked, network-free self-test spec to verify the runner can
// discover, transpile and execute a mounted-style spec end to end (config +
// reporters included), without needing any test baked into the default testDir.
func TestSelftestSpecRuns(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/playwright-runner:rolling")
	cfg := &testhelpers.ContainerConfig{Env: map[string]string{"PW_TEST_DIR": "/app/selftest"}}
	testhelpers.TestCommandSucceeds(t, ctx, image, cfg, "npx", "playwright", "test")
}
