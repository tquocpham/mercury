package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetDefaultString_returnsDefault(t *testing.T) {
	cfg := NewConfig("yaml")
	got := cfg.SetDefaultString("web_port", "8080", false)
	if got != "8080" {
		t.Fatalf("expected %q, got %q", "8080", got)
	}
}

func TestSetDefaultString_envOverride(t *testing.T) {
	t.Setenv("WEB_PORT", "9999")
	cfg := NewConfig("yaml")
	got := cfg.SetDefaultString("web_port", "8080", false)
	if got != "9999" {
		t.Fatalf("expected env override %q, got %q", "9999", got)
	}
}

func TestSetDefaultInt_returnsDefault(t *testing.T) {
	cfg := NewConfig("yaml")
	got := cfg.SetDefaultInt("max_conn", 100, false)
	if got != 100 {
		t.Fatalf("expected %d, got %d", 100, got)
	}
}

func TestSetDefaultBool_returnsDefault(t *testing.T) {
	cfg := NewConfig("yaml")
	got := cfg.SetDefaultBool("debug", true, false)
	if !got {
		t.Fatal("expected true")
	}
}

func TestSetDefaultDuration_returnsDefault(t *testing.T) {
	cfg := NewConfig("yaml")
	got := cfg.SetDefaultDuration("timeout", 5*time.Second, false)
	if got != 5*time.Second {
		t.Fatalf("expected %v, got %v", 5*time.Second, got)
	}
}

func TestAllSettings_onlyShowsRegisteredKeys(t *testing.T) {
	cfg := NewConfig("yaml")
	cfg.SetDefaultString("registered", "val", false)
	// SetDefault directly bypasses register, so it should not appear
	cfg.SetDefault("unregistered", "other")

	settings := cfg.AllSettings()
	if _, ok := settings["registered"]; !ok {
		t.Error("registered key should appear in AllSettings")
	}
	if _, ok := settings["unregistered"]; ok {
		t.Error("unregistered key should not appear in AllSettings")
	}
}

func TestAllSettings_masksSecureKeys(t *testing.T) {
	cfg := NewConfig("yaml")
	cfg.SetDefaultString("db_password", "super_secret", true)

	settings := cfg.AllSettings()
	if settings["db_password"] != "****" {
		t.Fatalf("expected masked value, got %v", settings["db_password"])
	}
}

func TestAllSettings_nonSecureKeyIsVisible(t *testing.T) {
	cfg := NewConfig("yaml")
	cfg.SetDefaultString("log_level", "info", false)

	settings := cfg.AllSettings()
	if settings["log_level"] != "info" {
		t.Fatalf("expected %q, got %v", "info", settings["log_level"])
	}
}

func TestLoad_silentlyIgnoresMissingFile(t *testing.T) {
	cfg := NewConfig("yaml")
	if err := cfg.Load("/nonexistent/path/config.yaml"); err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
}

func TestLoad_mergesYAMLValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("web_port: \"9001\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := NewConfig("yaml")
	cfg.SetDefaultString("web_port", "8080", false)

	if err := cfg.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := cfg.GetString("web_port"); got != "9001" {
		t.Fatalf("expected %q after load, got %q", "9001", got)
	}
}

func TestLoadPaths_skipsNonExistentFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("foo: bar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := NewConfig("yaml")
	err := cfg.LoadPaths([]string{"/no/such/file.yaml", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.GetString("foo"); got != "bar" {
		t.Fatalf("expected %q, got %q", "bar", got)
	}
}

func TestLoadPaths_mergesMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	if err := os.WriteFile(first, []byte("key_a: value_a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("key_b: value_b\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := NewConfig("yaml")
	if err := cfg.LoadPaths([]string{first, second}); err != nil {
		t.Fatal(err)
	}
	if got := cfg.GetString("key_a"); got != "value_a" {
		t.Fatalf("key_a: expected %q, got %q", "value_a", got)
	}
	if got := cfg.GetString("key_b"); got != "value_b" {
		t.Fatalf("key_b: expected %q, got %q", "value_b", got)
	}
}
