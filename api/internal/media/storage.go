package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Backend        string
	LocalDir       string
	S3Bucket       string
	S3Region       string
	S3Endpoint     string
	S3AccessKeyID  string
	S3SecretAccess string
	S3ForcePath    bool
}

type Storage interface {
	Put(ctx context.Context, key string, contentType string, body io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, key string) error
}

func New(cfg Config) (Storage, error) {
	backend := strings.TrimSpace(cfg.Backend)
	switch backend {
	case "s3":
		return newS3Storage(cfg)
	case "", "local":
		return newLocalStorage(cfg.LocalDir)
	default:
		return nil, fmt.Errorf("unsupported media backend: %s", backend)
	}
}

type localStorage struct {
	root string
}

func newLocalStorage(root string) (Storage, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "./var/media"
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create local media dir: %w", err)
	}
	return &localStorage{root: root}, nil
}

func (s *localStorage) Put(_ context.Context, key string, _ string, body io.Reader, _ int64) error {
	path := filepath.Join(s.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create local media parent dir: %w", err)
	}
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create local media file: %w", err)
	}
	if _, err := io.Copy(file, body); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write local media file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close local media file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("commit local media file: %w", err)
	}
	return nil
}

func (s *localStorage) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	path := filepath.Join(s.root, filepath.FromSlash(key))
	file, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open local media file: %w", err)
	}
	return file, "video/mp4", nil
}

func (s *localStorage) Delete(_ context.Context, key string) error {
	path := filepath.Join(s.root, filepath.FromSlash(key))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete local media file: %w", err)
	}
	return nil
}

type s3Storage struct {
	client *s3.Client
	bucket string
}

func newS3Storage(cfg Config) (Storage, error) {
	if strings.TrimSpace(cfg.S3Bucket) == "" {
		return nil, fmt.Errorf("AIMMOD_HUB_S3_BUCKET is required when AIMMOD_HUB_MEDIA_BACKEND=s3")
	}
	if strings.TrimSpace(cfg.S3Region) == "" {
		cfg.S3Region = "auto"
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.S3Region),
	}
	if strings.TrimSpace(cfg.S3AccessKeyID) != "" || strings.TrimSpace(cfg.S3SecretAccess) != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccess, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.S3ForcePath
		if endpoint := strings.TrimSpace(cfg.S3Endpoint); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	return &s3Storage{
		client: client,
		bucket: cfg.S3Bucket,
	}, nil
}

func (s *s3Storage) Put(ctx context.Context, key string, contentType string, body io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("put s3 object: %w", err)
	}
	return nil
}

func (s *s3Storage) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("get s3 object: %w", err)
	}
	contentType := "video/mp4"
	if output.ContentType != nil && strings.TrimSpace(*output.ContentType) != "" {
		contentType = strings.TrimSpace(*output.ContentType)
	}
	return output.Body, contentType, nil
}

func (s *s3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete s3 object: %w", err)
	}
	return nil
}
