package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

type IAMProvisioner struct {
	Config *Config
	Policy *iam.Policy
	Role   *iam.Role
}

func (i *IAMProvisioner) provision() error {
	if err := i.provisionPolicy(); err != nil {
		return err
	}
	if err := i.provisionRole(); err != nil {
		return err
	}

	return nil
}

func (i *IAMProvisioner) provisionPolicy() error {
	i.Config.CurrentEnv().Policy.Version = "2012-10-17"

	b, _ := json.Marshal(i.Config.CurrentEnv().Policy)

	svc := iam.New(session.New(awsConfig))

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
			log.Println("Policy hasn't changed")
			return nil
		}

		log.Println("Policy already exists, updating")
		log.Println("Creating policy version")

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

	log.Println("Policy created")

	return nil
}

func (i *IAMProvisioner) provisionRole() error {
	svc := iam.New(session.New(awsConfig))

	if getRoleOutput, err := svc.GetRole(&iam.GetRoleInput{
		RoleName: aws.String(i.Config.Name),
	}); err == nil {
		i.Role = getRoleOutput.Role
		log.Println("Role already exists")
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

	createRoleOutput, err := svc.CreateRole(&iam.CreateRoleInput{
		RoleName:                 aws.String(i.Config.Name), // Required
		AssumeRolePolicyDocument: aws.String(doc),           // Required
		Path: aws.String("/gappa/"),
	})
	if err != nil {
		log.Println(err.Error())
		return err
	}

	i.Role = createRoleOutput.Role

	if _, err := svc.AttachRolePolicy(&iam.AttachRolePolicyInput{
		PolicyArn: i.Policy.Arn,              // Required
		RoleName:  aws.String(i.Config.Name), // Required
	}); err != nil {
		return err
	}

	log.Println("Role created")

	return nil
}
