package config

import (
	"context"
	"crypto/rsa"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/dgrijalva/jwt-go"
)

type Keys struct {
	Public  *rsa.PublicKey
	Private *rsa.PrivateKey
}

func NewKeys() *Keys {
	return &Keys{}
}

// LoadPrivateFromSSM loads the private key from an AWS SSM parameter.
func (k *Keys) LoadPrivateFromSSM(ssmClient *ssm.Client, paramName string) error {
	result, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           &paramName,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return err
	}

	privKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(*result.Parameter.Value))
	if err != nil {
		return err
	}
	k.Private = privKey
	return nil
}

// LoadPublicFromSSM loads the public key from an AWS SSM parameter.
// Pass a non-empty endpoint to override the AWS endpoint (e.g. for LocalStack).
func (k *Keys) LoadPublicFromSSM(ssmClient *ssm.Client, paramName string) error {
	result, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           &paramName,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return err
	}

	pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(*result.Parameter.Value))
	if err != nil {
		return err
	}
	k.Public = pubKey
	return nil
}
