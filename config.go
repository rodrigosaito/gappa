package main

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Name         string
	Environments map[string]Environment
	Lambda       *Lambda
}

func (c *Config) CurrentEnv() Environment {
	return c.Environments["prod"]
}

func (c *Config) String() string {
	d, err := yaml.Marshal(c)
	if err != nil {
		return "Config"
	}

	return string(d)
}

type Environment struct {
	Region  string
	Profile string
	Policy  *Policy
}

type Policy struct {
	Version    string
	Statements []PolicyStatement `json:"Statement"`
}

type PolicyStatement struct {
	Effect   string   `yaml:"Effect"`
	Resource string   `yaml:"Resource"`
	Action   []string `yaml:"Action"`
}

type Lambda struct {
	Description string
	Handler     string
	Runtime     string
	MemorySize  string
	Timeout     int
	VpcConfig   *VpcConfig `yaml:"vpc_config"`
}

type VpcConfig struct {
	SecurityGroupIds []string
	SubnetIds        []string
}

func extractConfig(filename string) (*Config, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := &Config{}

	if err := yaml.Unmarshal(content, config); err != nil {
		return nil, err
	}

	return config, nil
}
