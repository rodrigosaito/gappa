package main

import "github.com/codegangsta/cli"

type Config struct {
	Name         string
	Environments []Environment
	Lambda       *Lambda
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
	Effect   string
	Resource string
	Action   []string
}

type Lambda struct {
	Description string
	Handler     string
	Runtime     string
	MemorySize  string
	Timeout     int
}

func extractConfig(c *cli.Context) *Config {
	// TODO read file
	return &Config{
		Name: "some_lambda",
		Environments: []Environment{
			Environment{
				Profile: "saito",
				Region:  "us-east-1",
				Policy: &Policy{
					Version: "2012-10-17",
					Statements: []PolicyStatement{
						PolicyStatement{
							Effect:   "Allow",
							Resource: "*",
							Action: []string{
								"logs:*",
							},
						},
					},
				},
			},
		},
	}
}
