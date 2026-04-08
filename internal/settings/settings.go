package settings

import (
	"github.com/sonroyaalmerol/snry-shell/internal/store"
)

const (
	keyDarkMode              = "dark_mode"
	keyDoNotDisturb          = "do_not_disturb"
	keyInputMode             = "input_mode"
	keyIdleLockTimeout       = "idle_lock_timeout"
	keyIdleDisplayOffTimeout = "idle_displayoff_timeout"
	keyLockDisplayOffTimeout = "lock_displayoff_timeout"
	keyIdleSuspendTimeout    = "idle_suspend_timeout"
	keyLockMaxAttempts       = "lock_max_attempts"
	keyLockoutDuration       = "lockout_duration"
	keyLockShowClock         = "lock_show_clock"
	keyLockShowUser          = "lock_show_user"
	keyLidCloseAction        = "lid_close_action"
	keyPowerButtonAction     = "power_button_action"
	keyBarPosition           = "bar.position"
	keyBarShowBatteryPct     = "bar.show_battery_pct"
	keyClockFormat           = "clock.format"
	keyNotificationTimeout   = "notifications.timeout"
	keyNotificationPosition  = "notifications.position"
	keyVolumeStep            = "audio.volume_step"
	keyBrightnessStep        = "brightness.step"
	keyBlurStrength        = "theme.blur_strength"
	keyWallpaperSource     = "theme.wallpaper_source"
	keyWallpaperFit        = "theme.wallpaper_fit"
	keyWallpaperBlur       = "theme.wallpaper_blur"
	keyWallpaperBrightness = "theme.wallpaper_brightness"
	keyWallpaperGrayscale  = "theme.wallpaper_grayscale"
)

type Config struct {
	DarkMode              bool
	DoNotDisturb          bool
	InputMode             string
	IdleLockTimeout       int // seconds before lock
	IdleDisplayOffTimeout int // seconds before display off
	LockDisplayOffTimeout int // seconds before display off when locked
	IdleSuspendTimeout    int // seconds after lock before suspend
	LockMaxAttempts       int
	LockoutDuration       int
	LockShowClock         bool
	LockShowUser          bool
	LidCloseAction        string
	PowerButtonAction     string

	BarPosition          string
	BarShowBatteryPct    bool
	ClockFormat          string
	NotificationTimeout  int
	NotificationPosition string
	VolumeStep           float64
	BrightnessStep       float64
	BlurStrength int

	WallpaperSource     string // original user-selected path
	WallpaperFit        string // "cover", "contain", "fill", "scale-down"
	WallpaperBlur       int    // 0–50
	WallpaperBrightness int    // 0–200, 100 = no change
	WallpaperGrayscale  bool
}

func DefaultConfig() Config {
	return Config{
		DarkMode:              true,
		DoNotDisturb:          false,
		InputMode:             "auto",
		IdleLockTimeout:       300,
		IdleDisplayOffTimeout: 120,
		LockDisplayOffTimeout: 30,
		IdleSuspendTimeout:    0,
		LockMaxAttempts:       3,
		LockoutDuration:       30,
		LockShowClock:         true,
		LockShowUser:          true,
		LidCloseAction:        "suspend",
		PowerButtonAction:     "shutdown",

		BarPosition:          "top",
		BarShowBatteryPct:    true,
		ClockFormat:          "24h",
		NotificationTimeout:  5000,
		NotificationPosition: "top-right",
		VolumeStep:           0.05,
		BrightnessStep:       0.05,
		BlurStrength: 20,

		WallpaperSource:     "",
		WallpaperFit:        "cover",
		WallpaperBlur:       0,
		WallpaperBrightness: 100,
		WallpaperGrayscale:  false,
	}
}

func Load() (Config, error) {
	d := DefaultConfig()
	return Config{
		DarkMode:              store.LookupOr(keyDarkMode, d.DarkMode),
		DoNotDisturb:          store.LookupOr(keyDoNotDisturb, d.DoNotDisturb),
		InputMode:             store.LookupOr(keyInputMode, d.InputMode),
		IdleLockTimeout:       store.LookupOr(keyIdleLockTimeout, d.IdleLockTimeout),
		IdleDisplayOffTimeout: store.LookupOr(keyIdleDisplayOffTimeout, d.IdleDisplayOffTimeout),
		LockDisplayOffTimeout: store.LookupOr(keyLockDisplayOffTimeout, d.LockDisplayOffTimeout),
		IdleSuspendTimeout:    store.LookupOr(keyIdleSuspendTimeout, d.IdleSuspendTimeout),
		LockMaxAttempts:       store.LookupOr(keyLockMaxAttempts, d.LockMaxAttempts),
		LockoutDuration:       store.LookupOr(keyLockoutDuration, d.LockoutDuration),
		LockShowClock:         store.LookupOr(keyLockShowClock, d.LockShowClock),
		LockShowUser:          store.LookupOr(keyLockShowUser, d.LockShowUser),
		LidCloseAction:        store.LookupOr(keyLidCloseAction, d.LidCloseAction),
		PowerButtonAction:     store.LookupOr(keyPowerButtonAction, d.PowerButtonAction),

		BarPosition:          store.LookupOr(keyBarPosition, d.BarPosition),
		BarShowBatteryPct:    store.LookupOr(keyBarShowBatteryPct, d.BarShowBatteryPct),
		ClockFormat:          store.LookupOr(keyClockFormat, d.ClockFormat),
		NotificationTimeout:  store.LookupOr(keyNotificationTimeout, d.NotificationTimeout),
		NotificationPosition: store.LookupOr(keyNotificationPosition, d.NotificationPosition),
		VolumeStep:           store.LookupOr(keyVolumeStep, d.VolumeStep),
		BrightnessStep:       store.LookupOr(keyBrightnessStep, d.BrightnessStep),
		BlurStrength: store.LookupOr(keyBlurStrength, d.BlurStrength),

		WallpaperSource:     store.LookupOr(keyWallpaperSource, d.WallpaperSource),
		WallpaperFit:        store.LookupOr(keyWallpaperFit, d.WallpaperFit),
		WallpaperBlur:       store.LookupOr(keyWallpaperBlur, d.WallpaperBlur),
		WallpaperBrightness: store.LookupOr(keyWallpaperBrightness, d.WallpaperBrightness),
		WallpaperGrayscale:  store.LookupOr(keyWallpaperGrayscale, d.WallpaperGrayscale),
	}, nil
}

func Save(cfg Config) error {
	return store.SetMany(map[string]any{
		keyDarkMode:              cfg.DarkMode,
		keyDoNotDisturb:          cfg.DoNotDisturb,
		keyInputMode:             cfg.InputMode,
		keyIdleLockTimeout:       cfg.IdleLockTimeout,
		keyIdleDisplayOffTimeout: cfg.IdleDisplayOffTimeout,
		keyLockDisplayOffTimeout: cfg.LockDisplayOffTimeout,
		keyIdleSuspendTimeout:    cfg.IdleSuspendTimeout,
		keyLockMaxAttempts:       cfg.LockMaxAttempts,
		keyLockoutDuration:       cfg.LockoutDuration,
		keyLockShowClock:         cfg.LockShowClock,
		keyLockShowUser:          cfg.LockShowUser,
		keyLidCloseAction:        cfg.LidCloseAction,
		keyPowerButtonAction:     cfg.PowerButtonAction,

		keyBarPosition:          cfg.BarPosition,
		keyBarShowBatteryPct:    cfg.BarShowBatteryPct,
		keyClockFormat:          cfg.ClockFormat,
		keyNotificationTimeout:  cfg.NotificationTimeout,
		keyNotificationPosition: cfg.NotificationPosition,
		keyVolumeStep:           cfg.VolumeStep,
		keyBrightnessStep:       cfg.BrightnessStep,
		keyBlurStrength: cfg.BlurStrength,

		keyWallpaperSource:     cfg.WallpaperSource,
		keyWallpaperFit:        cfg.WallpaperFit,
		keyWallpaperBlur:       cfg.WallpaperBlur,
		keyWallpaperBrightness: cfg.WallpaperBrightness,
		keyWallpaperGrayscale:  cfg.WallpaperGrayscale,
	})
}
