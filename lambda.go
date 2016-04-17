package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/lambda"
)

type LambdaRetryer struct {
}

func (r *LambdaRetryer) MaxRetries() int {
	return 3
}

func (r *LambdaRetryer) RetryRules(req *request.Request) time.Duration {
	return time.Duration(10) * time.Second
}

func (r *LambdaRetryer) ShouldRetry(req *request.Request) bool {
	if req.Error != nil {
		if reqerr, ok := req.Error.(awserr.RequestFailure); ok {
			if reqerr.Code() == "InvalidParameterValueException" {
				return true
			}
		}
	}

	return false
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

	svc := lambda.New(session.New(awsConfig), &aws.Config{Retryer: &LambdaRetryer{}})

	if getFunctionOutput, err := svc.GetFunction(&lambda.GetFunctionInput{
		FunctionName: aws.String(l.Config.Name),
	}); err == nil {
		if codeHash64 == *getFunctionOutput.Configuration.CodeSha256 {
			log.Println("Code hasn't changed")
			return nil
		}

		if _, err := svc.UpdateFunctionCode(&lambda.UpdateFunctionCodeInput{
			FunctionName: aws.String(l.Config.Name),
			Publish:      aws.Bool(true),
			ZipFile:      packageBytes,
		}); err != nil {
			return err
		}

		log.Println("Code updated")
		return nil
	}

	if _, err := svc.CreateFunction(&lambda.CreateFunctionInput{
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
	}); err != nil {
		return err
	}

	log.Println("Lambda function created")
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
