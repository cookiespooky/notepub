package s3util

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Endpoint       string
	Region         string
	ForcePathStyle bool
	Bucket         string
	Prefix         string
	AccessKey      string
	SecretKey      string
	Anonymous      bool
}

type Object struct {
	Key          string
	ETag         string
	LastModified *time.Time
	Size         int64
	ContentType  string
}

func NewClient(ctx context.Context, cfg Config) (*s3.Client, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if cfg.Endpoint != "" {
			return aws.Endpoint{
				URL:               cfg.Endpoint,
				SigningRegion:     cfg.Region,
				HostnameImmutable: true,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
		config.WithEndpointResolverWithOptions(customResolver),
	}
	if cfg.Anonymous {
		opts = append(opts, config.WithCredentialsProvider(aws.AnonymousCredentials{}))
	} else if cfg.AccessKey != "" || cfg.SecretKey != "" {
		creds := aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""))
		opts = append(opts, config.WithCredentialsProvider(creds))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.ForcePathStyle
	}), nil
}

func ListObjects(ctx context.Context, client *s3.Client, bucket, prefix string) ([]Object, error) {
	out := []Object{}
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			out = append(out, Object{
				Key:          aws.ToString(obj.Key),
				ETag:         strings.Trim(aws.ToString(obj.ETag), `"`),
				LastModified: obj.LastModified,
				Size:         aws.ToInt64(obj.Size),
			})
		}
	}
	return out, nil
}

func FetchObject(ctx context.Context, client *s3.Client, bucket, key string) ([]byte, error) {
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func PresignGet(ctx context.Context, client *s3.Client, bucket, key string) (string, string, error) {
	ps := s3.NewPresignClient(client)
	out, err := ps.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 5 * time.Minute
	})
	if err != nil {
		return "", "", err
	}
	return out.URL, time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339), nil
}
