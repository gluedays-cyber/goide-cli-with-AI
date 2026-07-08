package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"goide/crypto"
)

type CustomModel struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
}

type APIProvider struct {
	APIKey string        `json:"apiKey"`
	Models []CustomModel `json:"models"`
}

type AIConfig struct {
	Google  APIProvider `json:"google"`
	Groq    APIProvider `json:"groq"`
	Ollama  APIProvider `json:"ollama"`
	Upstage APIProvider `json:"upstage"`
}

type ModelOption struct {
	Provider string
	Name     string
	Alias    string
}

// LoadAIConfig reads ~/Documents/.apikeys.json, decrypts the keys,
// and flattens the models into a slice of ModelOption.
func LoadAIConfig() (config AIConfig, options []ModelOption, err error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return config, nil, e
	}
	configPath := filepath.Join(home, "Documents", ".apikeys.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, nil, err
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, nil, err
	}

	// Decrypt API keys
	if config.Google.APIKey != "" {
		if dec, err := crypto.Decrypt(config.Google.APIKey); err == nil {
			config.Google.APIKey = dec
		}
	}
	if config.Groq.APIKey != "" {
		if dec, err := crypto.Decrypt(config.Groq.APIKey); err == nil {
			config.Groq.APIKey = dec
		}
	}
	if config.Ollama.APIKey != "" {
		if dec, err := crypto.Decrypt(config.Ollama.APIKey); err == nil {
			config.Ollama.APIKey = dec
		}
	}
	if config.Upstage.APIKey != "" {
		if dec, err := crypto.Decrypt(config.Upstage.APIKey); err == nil {
			config.Upstage.APIKey = dec
		}
	}

	// Gather models
	for _, m := range config.Google.Models {
		options = append(options, ModelOption{Provider: "google", Name: m.Name, Alias: m.Alias})
	}
	for _, m := range config.Groq.Models {
		options = append(options, ModelOption{Provider: "groq", Name: m.Name, Alias: m.Alias})
	}
	for _, m := range config.Ollama.Models {
		options = append(options, ModelOption{Provider: "ollama", Name: m.Name, Alias: m.Alias})
	}
	for _, m := range config.Upstage.Models {
		options = append(options, ModelOption{Provider: "upstage", Name: m.Name, Alias: m.Alias})
	}

	return config, options, nil
}
