package main

import (
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/codegangsta/cli"
)

var awsConfig *aws.Config

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

func deleteLambda(c *Config) {
	lambdaDeleter := LambdaDeleter{
		Config: c,
	}
	if err := lambdaDeleter.delete(); err != nil {
		log.Fatal("Error deleting lambda function:", err)
	}

	iamDeleter := IAMDeleter{
		Config: c,
	}
	if err := iamDeleter.delete(); err != nil {
		log.Fatal("Error deleteing iam role:", err)
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "gappa"
	app.Usage = "command line tool to make it easier to deploy aws lambda functions"

	awsConfig = &aws.Config{
		Region:      aws.String("us-east-1"),
		Credentials: credentials.NewSharedCredentials("", "saito"),
	}

	app.Commands = []cli.Command{
		{
			Name:  "deploy",
			Usage: "deploy lambda function",
			Action: func(c *cli.Context) {
				config, err := extractConfig("kappa.yml")
				if err != nil {
					log.Fatal("Failed to read config: ", err)
				}
				deploy(config)
			},
		},
		{
			Name:  "delete",
			Usage: "delete lambda function",
			Action: func(c *cli.Context) {
				config, err := extractConfig("kappa.yml")
				if err != nil {
					log.Fatal("Failed to read config: ", err)
				}
				deleteLambda(config)
			},
		},
	}

	app.Run(os.Args)
}
