package xdg

import (
	"os"
	"path/filepath"
)

var defaultConfigDir = os.Getenv("HOME") + "/.config"
var defaultStateDir = os.Getenv("HOME") + "/.local/state"

func GetConfigDir() string {
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		return defaultConfigDir
	}

	if !filepath.IsAbs(xdgConfigHome) {
		return defaultConfigDir
	}

	return xdgConfigHome + "/dots"
}

func GetStateDir() string {
	xdgStateHome := os.Getenv("XDG_STATE_HOME")
	if xdgStateHome == "" {
		return defaultStateDir
	}

	if !filepath.IsAbs(xdgStateHome) {
		return defaultStateDir
	}

	return xdgStateHome + "/dots"
}
