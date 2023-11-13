package lapd

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

const config_file_name = "lapd.yml"

// Structs for config file
type Filter struct {
	BasePath     string   `yaml:"base_path"`
	RelativePath string   `yaml:"relative_path"`
	Include      []string `yaml:"include"`
	Exclude      []string `yaml:"exclude"`
}

type FuncCfg struct {
	Name    string   `yaml:"name"`
	Filters []Filter `yaml:"filter"`
}

type Config struct {
	Functions        []FuncCfg `yaml:"functions"`
	S3Bucket         string    `yaml:"s3_bucket"`
	Package          string    `yaml:"package"`
	LocalPackageName string    `yaml:"local_package_name"`
}

// Read file "lapd.yml" as configuration file
func (c *Config) GetConfig() (*Config, error) {

	if _, err := os.Stat(config_file_name); os.IsNotExist(err) {
		CreateConfigFile()
	}

	yamlFile, err := os.ReadFile(config_file_name)
	if err != nil {
		log.Printf("Error reading YAML file: %s\n", err)
		return nil, err
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Printf("Error parsing YAML file: %s\n", err)
		return nil, err
	}
	return c, nil
}

// Example exclude filters:
//
//	".venv",
//	"node_modules",
//	"dist",
func CreateConfigFile() {
	c := Config{
		LocalPackageName: "deploy.zip",
		S3Bucket:         "lapd",
		Package:          "deploy.zip",
		Functions: []FuncCfg{
			{
				Name: "default",
				Filters: []Filter{
					{
						BasePath:     ".",
						RelativePath: "src",
						Include:      []string{"*"},
						Exclude:      []string{},
					},
					{
						BasePath:     ".venv/lib/python3.11/site-packages/",
						RelativePath: ".",
						Include:      []string{"*"},
						Exclude:      []string{},
					},
				},
			},
		},
	}
	data, err := yaml.Marshal(&c)
	if err != nil {
		log.Printf("Cant markshall the default config, here is why: %v\n", err)
	}
	err = os.WriteFile("lapd.yml", data, 0644)
	if err != nil {
		log.Printf("Cant write default config, here is why:  %v\n", err)
	}
}
