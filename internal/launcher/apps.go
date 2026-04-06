package launcher

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Launch starts the given app detached from the current process.
func Launch(app App) error {
	fields := strings.Fields(app.Exec)
	if len(fields) == 0 {
		return nil
	}
	cmd := exec.Command(fields[0], fields[1:]...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

// App represents a parsed XDG .desktop entry.
type App struct {
	Name    string
	Exec    string
	Icon    string
	Comment string
}

// LoadAll reads all .desktop files from XDG data directories.
func LoadAll() ([]App, error) {
	dirs := xdgDataDirs()
	var apps []App
	for _, dir := range dirs {
		pattern := filepath.Join(dir, "applications", "*.desktop")
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			kv := parseDesktopKeys(path)
			if kv["NoDisplay"] == "true" || kv["Hidden"] == "true" {
				continue
			}
			name := kv["Name"]
			exec_ := cleanExec(kv["Exec"])
			if name == "" || exec_ == "" {
				continue
			}
			comment := kv["Comment"]
			if comment == "" {
				comment = kv["Name"]
			}
			apps = append(apps, App{Name: name, Exec: exec_, Icon: kv["Icon"], Comment: comment})
		}
	}
	return apps, nil
}

// WMClassToIcon builds a map from window class names (StartupWMClass) to
// icon theme names by scanning all installed .desktop files.
func WMClassToIcon() map[string]string {
	dirs := xdgDataDirs()
	m := make(map[string]string)
	for _, dir := range dirs {
		pattern := filepath.Join(dir, "applications", "*.desktop")
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			kv := parseDesktopKeys(path)
			if wmClass, icon := kv["StartupWMClass"], kv["Icon"]; wmClass != "" && icon != "" {
				m[wmClass] = icon
			}
		}
	}
	return m
}

// parseDesktopKeys returns all key=value pairs from the [Desktop Entry] section.
func parseDesktopKeys(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	kv := make(map[string]string)
	inDesktopEntry := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "[") {
			inDesktopEntry = line == "[Desktop Entry]"
			continue
		}
		if !inDesktopEntry {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if _, exists := kv[k]; !exists {
			kv[k] = v
		}
	}
	return kv
}

// cleanExec strips field codes (%f, %F, %u, %U, etc.) from the Exec value.
func cleanExec(exec string) string {
	fields := strings.Fields(exec)
	cleaned := fields[:0]
	for _, f := range fields {
		if !strings.HasPrefix(f, "%") {
			cleaned = append(cleaned, f)
		}
	}
	return strings.Join(cleaned, " ")
}

func xdgDataDirs() []string {
	dirs := []string{filepath.Join(os.Getenv("HOME"), ".local/share")}
	if d := os.Getenv("XDG_DATA_DIRS"); d != "" {
		dirs = append(dirs, strings.Split(d, ":")...)
	} else {
		dirs = append(dirs, "/usr/local/share", "/usr/share")
	}
	return dirs
}
