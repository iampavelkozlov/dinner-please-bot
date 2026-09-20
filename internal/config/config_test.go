package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{
		Bot: BotConfig{
			Token:            "token",
			AllowedUsernames: []string{"pav_kozlov"},
			RecipesPath:      "recipes",
			PageSize:         6,
		},
		Logger: LoggerConfig{Level: "info"},
	}
	require.NoError(t, valid.Validate())

	tests := map[string]func(*Config){
		"empty token":        func(c *Config) { c.Bot.Token = "" },
		"empty allowlist":    func(c *Config) { c.Bot.AllowedUsernames = nil },
		"blank username":     func(c *Config) { c.Bot.AllowedUsernames = []string{""} },
		"empty recipes path": func(c *Config) { c.Bot.RecipesPath = "" },
		"invalid page size":  func(c *Config) { c.Bot.PageSize = 0 },
		"invalid log level":  func(c *Config) { c.Logger.Level = "trace" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := valid
			mutate(&cfg)
			require.Error(t, cfg.Validate())
		})
	}
}

func TestLoad(t *testing.T) {
	t.Setenv("BOT_TOKEN", "token-from-env")
	t.Setenv("BOT_ALLOWED_USERNAMES", "@pav_kozlov, angelina_pos")
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
bot:
  token: token-from-file
  allowed_usernames: [someone]
  recipes_path: recipes
  page_size: 6
logger:
  level: info
`), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "token-from-env", cfg.Bot.Token)
	require.Equal(t, []string{"pav_kozlov", "angelina_pos"}, cfg.Bot.AllowedUsernames)

	_, err = Load(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "read config")
}
