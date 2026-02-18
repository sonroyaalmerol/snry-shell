package theme

// matugenOutput mirrors the JSON produced by `matugen image --json hex`.
type matugenOutput struct {
	Colors struct {
		Dark  palette `json:"dark"`
		Light palette `json:"light"`
	} `json:"colors"`
}

// palette holds every M3 color token produced by matugen for one scheme variant.
type palette struct {
	Primary            string `json:"primary"`
	OnPrimary          string `json:"on_primary"`
	PrimaryContainer   string `json:"primary_container"`
	OnPrimaryContainer string `json:"on_primary_container"`

	Secondary            string `json:"secondary"`
	OnSecondary          string `json:"on_secondary"`
	SecondaryContainer   string `json:"secondary_container"`
	OnSecondaryContainer string `json:"on_secondary_container"`

	Tertiary            string `json:"tertiary"`
	OnTertiary          string `json:"on_tertiary"`
	TertiaryContainer   string `json:"tertiary_container"`
	OnTertiaryContainer string `json:"on_tertiary_container"`

	Error            string `json:"error"`
	OnError          string `json:"on_error"`
	ErrorContainer   string `json:"error_container"`
	OnErrorContainer string `json:"on_error_container"`

	Surface                 string `json:"surface"`
	SurfaceDim              string `json:"surface_dim"`
	SurfaceBright           string `json:"surface_bright"`
	SurfaceContainer        string `json:"surface_container"`
	SurfaceContainerLow     string `json:"surface_container_low"`
	SurfaceContainerHigh    string `json:"surface_container_high"`
	SurfaceContainerHighest string `json:"surface_container_highest"`
	OnSurface               string `json:"on_surface"`
	OnSurfaceVariant        string `json:"on_surface_variant"`

	Background   string `json:"background"`
	OnBackground string `json:"on_background"`

	Outline        string `json:"outline"`
	OutlineVariant string `json:"outline_variant"`
}
