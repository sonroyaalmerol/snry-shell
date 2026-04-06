package launcher

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Launch starts the given app detached from the current process.
func Launch(app App) error {
	fields := strings.Fields(app.Exec)
	if len(fields) == 0 {
		return nil
	}
	cmd := exec.Command(fields[0], fields[1:]...)
	cmd.Env = os.Environ()
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
			app, err := parseDesktopFile(path)
			if err != nil || app.Name == "" || app.Exec == "" {
				continue
			}
			apps = append(apps, app)
		}
	}
	return apps, nil
}

func parseDesktopFile(path string) (App, error) {
	f, err := os.Open(path)
	if err != nil {
		return App{}, err
	}
	defer f.Close()

	var app App
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
		switch k {
		case "Name":
			if app.Name == "" {
				app.Name = v
			}
		case "Exec":
			app.Exec = cleanExec(v)
		case "Icon":
			app.Icon = v
		case "Comment":
			if app.Comment == "" {
				app.Comment = v
			}
		case "NoDisplay":
			if strings.EqualFold(v, "true") {
				return App{}, nil
			}
		case "Hidden":
			if strings.EqualFold(v, "true") {
				return App{}, nil
			}
		}
	}
	return app, scanner.Err()
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

// WMClassToIcon builds a map from window class names (StartupWMClass) to
// icon theme names by scanning all installed .desktop files.
func WMClassToIcon() map[string]string {
	dirs := xdgDataDirs()
	m := make(map[string]string)
	for _, dir := range dirs {
		pattern := filepath.Join(dir, "applications", "*.desktop")
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
		 wmClass, icon := parseDesktopWMClass(path)
		 if wmClass != "" && icon != "" {
			 m[wmClass] = icon
		 }
		}
	}
	return m
}

func parseDesktopWMClass(path string) (wmClass, icon string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()

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
		switch k {
		case "StartupWMClass":
			if wmClass == "" {
				wmClass = v
			}
		case "Icon":
			if icon == "" {
				icon = v
			}
		}
	}
	return wmClass, icon
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
