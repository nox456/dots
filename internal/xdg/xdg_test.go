package xdg

import (
	"os"
	"testing"
)

func TestGetConfigDir(t *testing.T) {
	tests := []struct {
		name          string
		xdgConfigHome string
		want          string
	}{
		{name: "unset falls back to default", xdgConfigHome: "", want: defaultConfigDir},
		{name: "relative path falls back to default", xdgConfigHome: "relative/config", want: defaultConfigDir},
		{name: "dot-relative path falls back to default", xdgConfigHome: "./config", want: defaultConfigDir},
		{name: "absolute path appends dots", xdgConfigHome: "/custom/config", want: "/custom/config/dots"},
		{name: "root path appends dots", xdgConfigHome: "/", want: "//dots"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)

			if got := GetConfigDir(); got != tt.want {
				t.Errorf("GetConfigDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetStateDir(t *testing.T) {
	tests := []struct {
		name         string
		xdgStateHome string
		want         string
	}{
		{name: "unset falls back to default", xdgStateHome: "", want: defaultStateDir},
		{name: "relative path falls back to default", xdgStateHome: "relative/state", want: defaultStateDir},
		{name: "dot-relative path falls back to default", xdgStateHome: "./state", want: defaultStateDir},
		{name: "absolute path appends dots", xdgStateHome: "/custom/state", want: "/custom/state/dots"},
		{name: "root path appends dots", xdgStateHome: "/", want: "//dots"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", tt.xdgStateHome)

			if got := GetStateDir(); got != tt.want {
				t.Errorf("GetStateDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultDirsUseHome(t *testing.T) {
	home := os.Getenv("HOME")

	if want := home + "/.config"; defaultConfigDir != want {
		t.Errorf("defaultConfigDir = %q, want %q", defaultConfigDir, want)
	}
	if want := home + "/.local/state"; defaultStateDir != want {
		t.Errorf("defaultStateDir = %q, want %q", defaultStateDir, want)
	}
}
