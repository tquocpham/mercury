package config

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// AWSConfig holds the AWS configuration values read from viper config.
type AWSConfig struct {
	AccessKey string
	SecretKey string
	Region    string
	Endpoint  string
}

// NewSSMClient creates an SSM client from explicit config values.
// No environment variables or IMDS are consulted.
func NewSSMClient(ctx context.Context, cfg AWSConfig) *ssm.Client {
	awscfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	}

	opts := func(o *ssm.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = &cfg.Endpoint
		}
	}

	return ssm.NewFromConfig(awscfg, opts)
}
