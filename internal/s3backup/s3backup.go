package s3backup

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ivan/infra-shelf/internal/backup"
	"github.com/ivan/infra-shelf/internal/config"
)

type Client struct {
	cfg    config.S3Config
	client *s3.Client
}

type UploadedFile struct {
	File backup.File
	Key  string
}

func New(ctx context.Context, cfg config.S3Config) (*Client, error) {
	if cfg.Bucket == "" {
		return nil, nil
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.ForcePathStyle
		if cfg.Endpoint != "" {
			options.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	return &Client{
		cfg:    cfg,
		client: client,
	}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.cfg.Bucket != ""
}

func (c *Client) Destination() string {
	if !c.Enabled() {
		return "not configured"
	}
	if c.cfg.Prefix == "" {
		return "s3://" + c.cfg.Bucket
	}
	return "s3://" + c.cfg.Bucket + "/" + c.cfg.Prefix
}

func (c *Client) Upload(ctx context.Context, file backup.File) (UploadedFile, error) {
	if !c.Enabled() {
		return UploadedFile{}, nil
	}

	handle, err := os.Open(file.Path)
	if err != nil {
		return UploadedFile{}, err
	}
	defer handle.Close()

	key := c.key(file)
	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
		Body:   handle,
	})
	if err != nil {
		return UploadedFile{}, err
	}

	return UploadedFile{File: file, Key: key}, nil
}

func (c *Client) UploadMany(ctx context.Context, files []backup.File) ([]UploadedFile, error) {
	uploaded := make([]UploadedFile, 0, len(files))
	for _, file := range files {
		item, err := c.Upload(ctx, file)
		if err != nil {
			return uploaded, fmt.Errorf("upload %s/%s to %s: %w", file.App, file.Name, c.Destination(), err)
		}
		uploaded = append(uploaded, item)
	}
	return uploaded, nil
}

func (c *Client) DeleteMany(ctx context.Context, files []backup.File) error {
	if !c.Enabled() {
		return nil
	}
	for _, file := range files {
		if _, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(c.cfg.Bucket),
			Key:    aws.String(c.key(file)),
		}); err != nil {
			return fmt.Errorf("delete %s/%s from %s: %w", file.App, file.Name, c.Destination(), err)
		}
	}
	return nil
}

func (c *Client) key(file backup.File) string {
	parts := []string{}
	if c.cfg.Prefix != "" {
		parts = append(parts, strings.Trim(c.cfg.Prefix, "/"))
	}
	parts = append(parts, file.App, file.Name)
	return path.Join(parts...)
}
