// Package assets provides embedded resources for snry-shell.
package assets

import _ "embed"

//go:embed style.css
var StyleCSS string
