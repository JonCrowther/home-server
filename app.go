package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// appOptionsPath is where Home Assistant writes the options an app is
// configured with (see app/config.yaml).
const appOptionsPath = "/data/options.json"

// loadAppOptions copies a Home Assistant app's options into environment
// variables, so they're read like any other setting: lifx_token becomes
// LIFX_TOKEN, log_level becomes LOG_LEVEL. Variables that are already set
// take priority, and a missing file (not running as an app) is not an error.
func loadAppOptions(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var opts map[string]string
	if err := json.Unmarshal(data, &opts); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	for key, value := range opts {
		name := strings.ToUpper(key)
		if _, set := os.LookupEnv(name); set || value == "" {
			continue
		}
		if err := os.Setenv(name, value); err != nil {
			return fmt.Errorf("setting %s: %w", name, err)
		}
	}
	return nil
}
