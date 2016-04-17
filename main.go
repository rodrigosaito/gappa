package main

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
)

func deploy(c *Config) {
	provisioner := IAMProvisioner{Config: c}
	if err := provisioner.provision(); err != nil {
		log.Fatal("Error provisioning IAM role:", err)
	}

	deployer := LambdaDeployer{
		Config: c,
		Role:   provisioner.Role,
	}
	if err := deployer.deploy(); err != nil {
		log.Fatal("Error deploying lambda function:", err)
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "gappa"
	app.Usage = "command line tool to make it easier to deploy aws lambda functions"

	app.Commands = []cli.Command{
		{
			Name:  "deploy",
			Usage: "deploy lambda function",
			Action: func(c *cli.Context) {
				deploy(extractConfig(c))
			},
		},
	}

	app.Run(os.Args)
}
