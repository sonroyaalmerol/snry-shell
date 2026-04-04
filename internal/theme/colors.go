package theme

// matugenOutput mirrors the JSON produced by matugen v4 `matugen image --json hex`.
type matugenOutput struct {
	Colors map[string]colorEntry `json:"colors"`
	Mode   string                `json:"mode"`
}

// colorEntry represents a single color role with dark/light/default variants.
type colorEntry struct {
	Dark    colorVal `json:"dark"`
	Default colorVal `json:"default"`
	Light   colorVal `json:"light"`
}

// colorVal holds the actual color hex string.
type colorVal struct {
	Color string `json:"color"`
}
