package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

func TestStreamlinkVersion(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/twitch-radio-worker:rolling")
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "streamlink", "--version")
}

func TestFFmpegVersion(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/twitch-radio-worker:rolling")
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "ffmpeg", "-version")
}

func TestEntrypointExists(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/oscaromeu/twitch-radio-worker:rolling")
	testhelpers.TestFileExists(t, ctx, image, "/entrypoint.sh", nil)
}
