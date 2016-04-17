package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/codegangsta/cli"
)

type Config struct {
	Name         string
	Environments []Environment
	Lambda       *Lambda
}

type Environment struct {
	Region string
	Policy *Policy
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
				Region: "us-east-1",
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

type IAMProvisioner struct {
	Config *Config
	Policy *iam.Policy
	Role   *iam.Role
}

func (i *IAMProvisioner) provision() error {
	i.provisionPolicy()
	i.provisionRole()

	return nil
}

func (i *IAMProvisioner) provisionPolicy() error {
	b, _ := json.Marshal(i.Config.Environments[0].Policy)

	svc := iam.New(session.New(), &aws.Config{Region: aws.String("us-east-1")})

	user := ""
	if resp, err := svc.GetUser(&iam.GetUserInput{}); err == nil {
		user = *resp.User.UserId
	} else {
		return err
	}

	policyArn := fmt.Sprintf("arn:aws:iam::%v:policy/gappa/%v", user, i.Config.Name)

	getPolicyOutput, err := svc.GetPolicy(&iam.GetPolicyInput{
		PolicyArn: aws.String(policyArn), // Required
	})
	if err == nil {
		i.Policy = getPolicyOutput.Policy

		policyVersionOutput, _ := svc.GetPolicyVersion(&iam.GetPolicyVersionInput{
			PolicyArn: aws.String(policyArn),
			VersionId: getPolicyOutput.Policy.DefaultVersionId,
		})

		doc, _ := url.QueryUnescape(*policyVersionOutput.PolicyVersion.Document)
		if doc == string(b) {
			fmt.Println("Policy hasn't changed")
			return nil
		}

		fmt.Println("Policy already exists, updating")
		fmt.Println("Creating policy version")

		_, err := svc.CreatePolicyVersion(&iam.CreatePolicyVersionInput{
			PolicyArn:      aws.String(policyArn),
			PolicyDocument: aws.String(string(b)),
			SetAsDefault:   aws.Bool(true),
		})
		if err != nil {
			return err
		}
	}

	policyOutput, err := svc.CreatePolicy(&iam.CreatePolicyInput{
		PolicyName:     aws.String(i.Config.Name),
		PolicyDocument: aws.String(string(b)),
		Path:           aws.String("/gappa/"),
	})
	if err != nil {
		return err
	}

	i.Policy = policyOutput.Policy

	fmt.Println("Policy created")

	return nil
}

func (i *IAMProvisioner) provisionRole() error {
	svc := iam.New(session.New(), &aws.Config{Region: aws.String("us-east-1")})

	if getRoleOutput, err := svc.GetRole(&iam.GetRoleInput{
		RoleName: aws.String(i.Config.Name),
	}); err == nil {
		i.Role = getRoleOutput.Role
		fmt.Println("ady exists")
		return nil
	}

	doc := `{
		"Version": "2008-10-17",
		"Statement": [
		{
			"Sid": "",
			"Effect": "Allow",
			"Principal": {
				"Service": "lambda.amazonaws.com"
			},
			"Action": "sts:AssumeRole"
		}
		]
	}`

	if _, err := svc.CreateRole(&iam.CreateRoleInput{
		RoleName:                 aws.String(i.Config.Name), // Required
		AssumeRolePolicyDocument: aws.String(doc),           // Required
		Path: aws.String("/gappa/"),
	}); err != nil {
		fmt.Println(err.Error())
		return err
	}

	if _, err := svc.AttachRolePolicy(&iam.AttachRolePolicyInput{
		PolicyArn: i.Policy.Arn,              // Required
		RoleName:  aws.String("some_lambda"), // Required
	}); err != nil {
		return err
	}

	fmt.Println("Role created")

	return nil
}

type LambdaDeployer struct {
	Config *Config
	Role   *iam.Role
	File   *os.File
}

func (l *LambdaDeployer) deploy() error {
	err := l.preparePackage()
	if err != nil {
		return err
	}

	packageBytes, err := ioutil.ReadFile(l.File.Name())
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write(packageBytes)

	codeHash64 := string(base64.StdEncoding.EncodeToString(h.Sum(nil)))

	svc := lambda.New(session.New(), &aws.Config{Region: aws.String("us-east-1")})

	if getFunctionOutput, err := svc.GetFunction(&lambda.GetFunctionInput{
		FunctionName: aws.String(l.Config.Name),
	}); err == nil {
		if codeHash64 == *getFunctionOutput.Configuration.CodeSha256 {
			fmt.Println("Code hasn't changed")
			return nil
		}

		if _, err := svc.UpdateFunctionCode(&lambda.UpdateFunctionCodeInput{
			FunctionName: aws.String(l.Config.Name),
			Publish:      aws.Bool(true),
			ZipFile:      packageBytes,
		}); err != nil {
			return err
		}

		fmt.Println("Code updated")
		return nil
	}

	createFunctionOutput, err := svc.CreateFunction(&lambda.CreateFunctionInput{
		Code: &lambda.FunctionCode{ // Required
			ZipFile: packageBytes,
		},
		FunctionName: aws.String(l.Config.Name),            // Required
		Handler:      aws.String("lambda_handler.handler"), // Required
		Role:         l.Role.Arn,                           // Required
		Runtime:      aws.String("python2.7"),              // Required
		Description:  aws.String("Description"),
		MemorySize:   aws.Int64(128),
		Publish:      aws.Bool(true),
		Timeout:      aws.Int64(3),
	})
	if err != nil {
		return err
	}

	fmt.Println(createFunctionOutput)

	fmt.Println("Lambda function created")
	return nil
}

func (l *LambdaDeployer) preparePackage() error {
	buf := new(bytes.Buffer)

	w := zip.NewWriter(buf)

	files, _ := ioutil.ReadDir("./src/")

	for _, file := range files {
		f, err := w.Create(file.Name())
		if err != nil {
			return err
		}

		body, err := ioutil.ReadFile(fmt.Sprintf("./src/%v", file.Name()))
		if err != nil {
			return err
		}

		_, err = f.Write([]byte(body))
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		return err
	}

	out, err := ioutil.TempFile("", "package")
	if err != nil {
		return err
	}

	if _, err := buf.WriteTo(out); err != nil {
		return err
	}

	l.File = out

	return nil
}

func deploy(c *Config) {
	provisioner := IAMProvisioner{Config: c}
	provisioner.provision()

	deployer := LambdaDeployer{
		Config: c,
		Role:   provisioner.Role,
	}
	if err := deployer.deploy(); err != nil {
		fmt.Println("Error deploying lambda function: ", err.Error())
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
