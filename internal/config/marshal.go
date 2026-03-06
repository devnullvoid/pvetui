package config

// marshaledConfig is the sanitized structure written to disk.
type marshaledConfig struct {
	Profiles       map[string]ProfileConfig `yaml:"profiles,omitempty"`
	DefaultProfile string                   `yaml:"default_profile,omitempty"`
	Debug          bool                     `yaml:"debug,omitempty"`
	CacheDir       string                   `yaml:"cache_dir,omitempty"`
	AgeDir         string                   `yaml:"age_dir,omitempty"`
	KeyBindings    KeyBindings              `yaml:"key_bindings,omitempty"`
	Theme          ThemeConfig              `yaml:"theme,omitempty"`
	Plugins        PluginConfig             `yaml:"plugins"`
	GroupSettings  map[string]GroupSettingsConfig `yaml:"group_settings,omitempty"`
	Addr           string                   `yaml:"addr,omitempty"`
	User           string                   `yaml:"user,omitempty"`
	Password       string                   `yaml:"password,omitempty"`
	TokenID        string                   `yaml:"token_id,omitempty"`
	TokenSecret    string                   `yaml:"token_secret,omitempty"`
	Realm          string                   `yaml:"realm,omitempty"`
	ApiPath        string                   `yaml:"api_path,omitempty"`
	Insecure       bool                     `yaml:"insecure,omitempty"`
	SSHUser        string                   `yaml:"ssh_user,omitempty"`
	VMSSHUser      string                   `yaml:"vm_ssh_user,omitempty"`
	SSHJumpHost    SSHJumpHost              `yaml:"ssh_jump_host,omitempty"`
}

// MarshalYAML implements yaml.Marshaler to ensure legacy single-profile fields
// are omitted when profile-based configuration is in use.
func (cfg *Config) MarshalYAML() (any, error) {
	if cfg == nil {
		return nil, nil
	}

	clean := marshaledConfig{
		Profiles:       cfg.Profiles,
		DefaultProfile: cfg.DefaultProfile,
		Debug:          cfg.Debug,
		CacheDir:       cfg.CacheDir,
		AgeDir:         cfg.AgeDir,
		KeyBindings:    cfg.KeyBindings,
		Theme:          cfg.Theme,
		Plugins:        cfg.Plugins,
		GroupSettings:  cfg.GroupSettings,
	}

	if len(cfg.Profiles) == 0 {
		clean.Addr = cfg.Addr
		clean.User = cfg.User
		clean.Password = cfg.Password
		clean.TokenID = cfg.TokenID
		clean.TokenSecret = cfg.TokenSecret
		clean.Realm = cfg.Realm
		clean.ApiPath = cfg.ApiPath
		clean.Insecure = cfg.Insecure
		clean.SSHUser = cfg.SSHUser
		clean.VMSSHUser = cfg.VMSSHUser
		clean.SSHJumpHost = cfg.SSHJumpHost
	}

	return clean, nil
}
