package api_test

import (
	"os"
	"testing"
)

var config *Config

func TestMain(m *testing.M) {
	config = Load()

	if config.BaseURL == "" {
		panic("TEST_API_BASEURL is not set")
	}

	os.Exit(m.Run())
}
