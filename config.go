package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type configMetadata struct {
	Path        string `yaml:"path"`
	PostMessage string `yaml:"post_message"`
}

func loadConfig(filePath string) (map[string][]configMetadata, error) {
	// Read the YAML file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	// Unmarshal YAML into a map[string][]Option
	var result map[string][]configMetadata
	err = yaml.Unmarshal(data, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}

	return result, nil
}

func getOptionLabels(configs map[string][]configMetadata) []string {
	var labels []string
	for label := range configs {
		labels = append(labels, label)
	}
	return labels
}

func getConfigMetadata(configs map[string][]configMetadata, selectedConfigs []string) []configMetadata {
	var configMetadata []configMetadata

	for _, selectedConfig := range selectedConfigs {
		selectedConfigMetadata, exist := configs[selectedConfig]
		if exist {
			configMetadata = append(configMetadata, selectedConfigMetadata...)
		}
	}
	return configMetadata
}
