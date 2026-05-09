package main

import (
	"context"
	"testing"

	"github.com/oscaromeu/containers/testhelpers"
)

const defaultImage = "ghcr.io/oscaromeu/ops-tools:rolling"

func TestPython(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "python3", "--version")
}

func TestPythonRequests(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "python3", "-c", "import requests")
}

func TestPythonYaml(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "python3", "-c", "import yaml")
}

func TestPythonRich(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "python3", "-c", "import rich")
}

func TestHttpie(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "http", "--version")
}

func TestCurl(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "curl", "--version")
}

func TestJq(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "jq", "--version")
}

func TestDig(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "dig", "-v")
}

func TestNc(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "nc", "-h")
}

func TestTcpdump(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "tcpdump", "--version")
}

func TestNmap(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "nmap", "--version")
}

func TestMtr(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "mtr", "--version")
}

func TestSocat(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "socat", "-V")
}

func TestYq(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage(defaultImage)
	testhelpers.TestCommandSucceeds(t, ctx, image, nil, "yq", "--version")
}
