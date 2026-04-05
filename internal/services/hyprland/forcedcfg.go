package hyprland

import "fmt"

// ForcedConfig is a single Hyprland config option the shell wants to
// temporarily override while the shell is running.
type ForcedConfig struct {
	Option string
	Value  string
}

// ForcedConfigs manages temporarily overriding Hyprland config values.
// On shell exit, Restore puts back the user's original values.
type ForcedConfigs struct {
	querier *Querier
	saved   map[string]string
}

// NewForcedConfigs creates a ForcedConfigs bound to the given Querier.
func NewForcedConfigs(querier *Querier) *ForcedConfigs {
	return &ForcedConfigs{querier: querier, saved: make(map[string]string)}
}

// Apply saves current values and applies all forced configs.
func (f *ForcedConfigs) Apply(cfgs []ForcedConfig) error {
	for _, c := range cfgs {
		cur, err := f.querier.GetOption(c.Option)
		if err != nil {
			return fmt.Errorf("forced config %s: %w", c.Option, err)
		}
		f.saved[c.Option] = cur
		if err := f.querier.SetKeyword(c.Option, c.Value); err != nil {
			return fmt.Errorf("forced config %s: %w", c.Option, err)
		}
	}
	return nil
}

// Restore reverts all forced configs back to their original values.
func (f *ForcedConfigs) Restore() {
	for option, original := range f.saved {
		_ = f.querier.SetKeyword(option, original)
	}
}
