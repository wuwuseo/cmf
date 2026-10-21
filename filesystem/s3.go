package filesystem

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	defaultS3Region = "us-east-1"
	// 内容寻址 key 同 key 即同内容，公开文件可以放心长缓存。
	publicObjectCacheControl = "public, max-age=31536000, immutable"
)

// s3Storage 直接基于 AWS SDK v2 实现 storage.Storage。
// 相比只接受 []byte 的通用适配器，额外提供 SetReader 流式上传，
// 避免分片合并或大文件上传时把整个文件读进内存。
type s3Storage struct {
	client     *awss3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
	bucket     string
}

// newS3Storage 根据磁盘 options 创建 S3 适配器。
// 缺失的可选配置按安全默认值补齐，不再对 options 直接做类型断言。
func newS3Storage(options map[string]any) (*s3Storage, error) {
	bucket := optionString(options, "bucket")
	if bucket == "" {
		return nil, fmt.Errorf("s3 存储缺少 bucket 配置")
	}

	client, err := newS3Client(options)
	if err != nil {
		return nil, fmt.Errorf("初始化 s3 客户端失败: %w", err)
	}

	return &s3Storage{
		client:     client,
		uploader:   manager.NewUploader(client),
		downloader: manager.NewDownloader(client),
		bucket:     bucket,
	}, nil
}

// newS3Client 构建 S3 客户端。自定义 endpoint（MinIO、自建网关）默认使用
// path-style 寻址；显式配置 use_path_style 时以配置为准。
func newS3Client(options map[string]any) (*awss3.Client, error) {
	region := optionString(options, "region")
	if region == "" {
		region = defaultS3Region
	}
	endpoint := normalizeS3Endpoint(optionString(options, "endpoint"))
	usePathStyle := optionBoolDefault(options, "use_path_style", endpoint != "")

	awsCfg, err := loadS3AWSConfig(
		region,
		optionString(options, "access_key"),
		optionString(options, "secret_key"),
	)
	if err != nil {
		return nil, err
	}

	return awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = usePathStyle
	}), nil
}

func loadS3AWSConfig(region, accessKey, secretKey string) (aws.Config, error) {
	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if accessKey != "" || secretKey != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}
	return awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
}

// normalizeS3Endpoint 补齐协议并去掉尾部斜杠，兼容 "127.0.0.1:9000" 这类写法。
func normalizeS3Endpoint(endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint != "" && !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	return endpoint
}

func (s *s3Storage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

func (s *s3Storage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	key = cleanStorageKey(key)
	if key == "" {
		return nil, nil
	}

	buf := manager.NewWriteAtBuffer([]byte{})
	_, err := s.downloader.Download(ctx, buf, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if isS3NotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *s3Storage) Set(key string, value []byte, expiration time.Duration) error {
	return s.SetWithContext(context.Background(), key, value, expiration)
}

func (s *s3Storage) SetWithContext(ctx context.Context, key string, value []byte, _ time.Duration) error {
	return s.putObject(ctx, key, bytes.NewReader(value), "")
}

func (s *s3Storage) SetReader(key string, reader io.Reader, expiration time.Duration) error {
	return s.SetReaderWithContentType(key, reader, "", expiration)
}

// SetReaderWithContentType 流式上传，并按需写入 Content-Type 与缓存头。
func (s *s3Storage) SetReaderWithContentType(key string, reader io.Reader, contentType string, _ time.Duration) error {
	return s.putObject(context.Background(), key, reader, contentType)
}

func (s *s3Storage) putObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	key = cleanStorageKey(key)
	if key == "" || body == nil {
		return nil
	}

	input := &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	if strings.HasPrefix(key, "public/") {
		input.CacheControl = aws.String(publicObjectCacheControl)
	}

	_, err := s.uploader.Upload(ctx, input)
	return err
}

func (s *s3Storage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (s *s3Storage) DeleteWithContext(ctx context.Context, key string) error {
	key = cleanStorageKey(key)
	if key == "" {
		return nil
	}
	_, err := s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *s3Storage) Reset() error {
	return s.ResetWithContext(context.Background())
}

// ResetWithContext 清空桶内对象；与 gofiber 适配器保持相同的语义。
func (s *s3Storage) ResetWithContext(ctx context.Context) error {
	paginator := awss3.NewListObjectsV2Paginator(s.client, &awss3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		objects := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, object := range page.Contents {
			objects = append(objects, types.ObjectIdentifier{Key: object.Key})
		}
		if len(objects) == 0 {
			continue
		}
		if _, err := s.client.DeleteObjects(ctx, &awss3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{Objects: objects},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *s3Storage) Close() error {
	return nil
}

func isS3NotFound(err error) bool {
	if err == nil {
		return false
	}
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return true
		}
	}
	return false
}
