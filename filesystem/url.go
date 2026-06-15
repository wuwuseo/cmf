package filesystem

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/wuwuseo/cmf/config"
)

func IsPublicDirectory(directory string) bool {
	directory = strings.Trim(strings.ReplaceAll(directory, `\`, "/"), "/")
	if directory == "" {
		return false
	}
	return directory == "public" || strings.HasPrefix(directory, "public/")
}

func LocalPublicURL(cfg config.Config, diskName string, key string) (string, error) {
	baseURL := diskPublicBaseURL(cfg, diskName)
	if baseURL == "" {
		baseURL = cfg.Attachment.PublicBaseURL
	}
	if baseURL == "" {
		return "", fmt.Errorf("public base url is not configured")
	}

	publicKey := strings.TrimPrefix(cleanStorageKey(key), "public/")
	return joinURLPath(baseURL, publicKey), nil
}

func XAccelRedirectPath(cfg config.Config, key string) (string, error) {
	prefix := strings.TrimRight(strings.TrimSpace(cfg.Attachment.XAccelPrefix), "/")
	if prefix == "" {
		return "", fmt.Errorf("x-accel prefix is not configured")
	}
	return joinURLPath(prefix, cleanStorageKey(key)), nil
}

func S3PublicURL(cfg config.Config, diskName string, key string) (string, error) {
	baseURL := diskPublicBaseURL(cfg, diskName)
	if baseURL == "" {
		baseURL = cfg.Attachment.PublicBaseURL
	}
	if baseURL == "" {
		return "", fmt.Errorf("public base url is not configured")
	}
	return joinURLPath(baseURL, cleanStorageKey(key)), nil
}

func S3PresignedGetURL(ctx context.Context, cfg config.Config, diskName string, key string, ttl time.Duration) (string, error) {
	options := diskOptions(cfg, diskName)
	if len(options) == 0 {
		return "", fmt.Errorf("disk %q options are not configured", diskName)
	}

	accessKey := optionString(options, "access_key")
	secretKey := optionString(options, "secret_key")
	region := optionString(options, "region")
	bucket := optionString(options, "bucket")
	if accessKey == "" || secretKey == "" || region == "" || bucket == "" {
		return "", fmt.Errorf("s3 disk %q is missing required credentials or bucket options", diskName)
	}
	if ttl <= 0 {
		ttl = optionDurationSeconds(options, "presign_expires")
	}
	if ttl <= 0 && cfg.Attachment.AccessURLTTL > 0 {
		ttl = time.Duration(cfg.Attachment.AccessURLTTL) * time.Second
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}

	client := awss3.New(awss3.Options{
		BaseEndpoint: aws.String(optionString(options, "endpoint")),
		Credentials:  aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		Region:       region,
		UsePathStyle: optionBool(options, "use_path_style"),
	})
	presigner := awss3.NewPresignClient(client)
	request, err := presigner.PresignGetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(cleanStorageKey(key)),
	}, func(options *awss3.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return request.URL, nil
}

func diskPublicBaseURL(cfg config.Config, diskName string) string {
	return optionString(diskOptions(cfg, diskName), "public_base_url")
}

func diskOptions(cfg config.Config, diskName string) map[string]any {
	if diskName == "" {
		diskName = cfg.Filesystem.Default
	}
	if diskName == "" {
		diskName = "local"
	}
	if cfg.Filesystem.Disks == nil {
		return nil
	}
	disk, ok := cfg.Filesystem.Disks[diskName]
	if !ok {
		return nil
	}
	options, ok := disk.Options.(map[string]any)
	if !ok {
		return nil
	}
	return options
}

func optionString(options map[string]any, key string) string {
	if options == nil {
		return ""
	}
	switch value := options[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return ""
	}
}

func optionBool(options map[string]any, key string) bool {
	if options == nil {
		return false
	}
	switch value := options[key].(type) {
	case bool:
		return value
	case string:
		parsed, _ := strconv.ParseBool(value)
		return parsed
	default:
		return false
	}
}

func optionDurationSeconds(options map[string]any, key string) time.Duration {
	if options == nil {
		return 0
	}
	switch value := options[key].(type) {
	case int:
		return time.Duration(value) * time.Second
	case int64:
		return time.Duration(value) * time.Second
	case float64:
		return time.Duration(value) * time.Second
	case string:
		seconds, _ := strconv.Atoi(value)
		return time.Duration(seconds) * time.Second
	default:
		return 0
	}
}

func cleanStorageKey(key string) string {
	key = strings.ReplaceAll(strings.TrimSpace(key), `\`, "/")
	key = strings.TrimPrefix(path.Clean("/"+key), "/")
	if key == "." {
		return ""
	}
	return key
}

func joinURLPath(baseURL string, key string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	key = strings.TrimLeft(key, "/")
	if key == "" {
		return baseURL
	}
	return baseURL + "/" + key
}
