package launcher_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/launcher"
)

func writeDesktopFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseDesktopFile(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "applications")
	os.MkdirAll(appDir, 0o755)

	writeDesktopFile(t, appDir, "firefox.desktop", `[Desktop Entry]
Name=Firefox
Exec=firefox %u
Icon=firefox
Comment=Web Browser
Type=Application
`)

	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_DIRS", dir)

	apps, err := launcher.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) == 0 {
		t.Fatal("expected at least one app")
	}
	var found *launcher.App
	for i := range apps {
		if apps[i].Name == "Firefox" {
			found = &apps[i]
			break
		}
	}
	if found == nil {
		t.Fatal("Firefox not found")
	}
	if found.Exec != "firefox" {
		t.Fatalf("expected exec 'firefox', got %q", found.Exec)
	}
	if found.Icon != "firefox" {
		t.Fatalf("expected icon 'firefox', got %q", found.Icon)
	}
}

func TestSkipsHiddenApps(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "applications")
	os.MkdirAll(appDir, 0o755)

	writeDesktopFile(t, appDir, "hidden.desktop", `[Desktop Entry]
Name=HiddenApp
Exec=hiddenapp
NoDisplay=true
`)

	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_DIRS", dir)

	apps, err := launcher.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range apps {
		if a.Name == "HiddenApp" {
			t.Fatal("hidden app should not appear in results")
		}
	}
}

func TestCleanExec(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "applications")
	os.MkdirAll(appDir, 0o755)

	writeDesktopFile(t, appDir, "code.desktop", `[Desktop Entry]
Name=VSCode
Exec=/usr/bin/code --unity-launch %F
Icon=code
`)

	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_DIRS", dir)

	apps, _ := launcher.LoadAll()
	for _, a := range apps {
		if a.Name == "VSCode" {
			if a.Exec != "/usr/bin/code --unity-launch" {
				t.Fatalf("expected cleaned exec, got %q", a.Exec)
			}
			return
		}
	}
	t.Fatal("VSCode not found")
}
