package pingen

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const ConfigEnvVar = "PINGEN_CONFIG_PATH"

// Config stores persisted settings for the CLI.
type Config struct {
	Env                  string `json:"env"`
	APIBase              string `json:"api_base"`
	IdentityBase         string `json:"identity_base"`
	OrganisationID       string `json:"organisation_id"`
	AccessToken          string `json:"access_token"`
	AccessTokenExpiresAt int64  `json:"access_token_expires_at"`
	ClientID             string `json:"client_id"`
	ClientSecret         string `json:"client_secret"`
}

func ConfigPath() (string, error) {
	if override := os.Getenv(ConfigEnvVar); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, "pingen", "config.json"), nil
}

func LoadConfig(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil
		}
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}

func SaveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(payload)
	return err
}

func MergeConfig(base Config, override Config) Config {
	merged := base
	if override.Env != "" {
		merged.Env = override.Env
	}
	if override.APIBase != "" {
		merged.APIBase = override.APIBase
	}
	if override.IdentityBase != "" {
		merged.IdentityBase = override.IdentityBase
	}
	if override.OrganisationID != "" {
		merged.OrganisationID = override.OrganisationID
	}
	if override.AccessToken != "" {
		merged.AccessToken = override.AccessToken
	}
	if override.AccessTokenExpiresAt != 0 {
		merged.AccessTokenExpiresAt = override.AccessTokenExpiresAt
	}
	if override.ClientID != "" {
		merged.ClientID = override.ClientID
	}
	if override.ClientSecret != "" {
		merged.ClientSecret = override.ClientSecret
	}
	return merged
}
