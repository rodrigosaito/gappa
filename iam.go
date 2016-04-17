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
	svc := iam.New(session.New(), &aws.Config{Region: aws.String("us-east-1")})

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

	if _, err := svc.CreateRole(&iam.CreateRoleInput{
		RoleName:                 aws.String(i.Config.Name), // Required
		AssumeRolePolicyDocument: aws.String(doc),           // Required
		Path: aws.String("/gappa/"),
	}); err != nil {
		log.Println(err.Error())
		return err
	}

	if _, err := svc.AttachRolePolicy(&iam.AttachRolePolicyInput{
		PolicyArn: i.Policy.Arn,              // Required
		RoleName:  aws.String("some_lambda"), // Required
	}); err != nil {
		return err
	}

	log.Println("Role created")

	return nil
}
