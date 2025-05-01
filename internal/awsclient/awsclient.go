package awsclient

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// AWSClient is a service that interacts with AWS services

type AWSClient interface {
	GetBotToken(ctx context.Context) (string, error)
}

type Logger interface {
	LogEvent(string)
}

type AWSClientImpl struct {
	sess   *session.Session
	logger Logger
}

func NewAWSClient(l Logger) (AWSClient, error) {
	awsRegion := "us-east-2"
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})
	if err != nil {
		return nil, err
	}
	return &AWSClientImpl{
		sess:   sess,
		logger: l,
	}, nil
}

// Implements GetBotToken method
func (a *AWSClientImpl) GetBotToken(ctx context.Context) (string, error) {
	ssmsvc := ssm.New(a.sess)
	param, err := ssmsvc.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String("/kiddokey-bot/BOT_TOKEN"),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {

		a.logger.LogEvent("Error while getting token from AWS Parameter Store: " + err.Error())
		return "", err
	}

	return *param.Parameter.Value, nil
}
