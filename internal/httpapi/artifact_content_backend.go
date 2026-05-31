package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

type ArtifactContentBackend interface {
	Provider() domain.ArtifactContentStorageProvider
	Capture(ctx context.Context, artifact domain.Artifact, input domain.ArtifactContentCapture, content []byte) (domain.ArtifactContentCapture, error)
	Read(ctx context.Context, content domain.ArtifactContent) ([]byte, error)
}

type artifactContentBackends struct {
	captureBackend ArtifactContentBackend
	readers        map[domain.ArtifactContentStorageProvider]ArtifactContentBackend
}

func newArtifactContentBackends(captureBackend ArtifactContentBackend) *artifactContentBackends {
	postgres := postgresArtifactContentBackend{}
	if captureBackend == nil {
		captureBackend = postgres
	}
	readers := map[domain.ArtifactContentStorageProvider]ArtifactContentBackend{
		"": postgres,
		domain.ArtifactContentStorageProviderPostgres: postgres,
	}
	readers[captureBackend.Provider()] = captureBackend
	return &artifactContentBackends{
		captureBackend: captureBackend,
		readers:        readers,
	}
}

func (backends *artifactContentBackends) Capture(ctx context.Context, artifact domain.Artifact, input domain.ArtifactContentCapture, content []byte) (domain.ArtifactContentCapture, error) {
	return backends.captureBackend.Capture(ctx, artifact, input, content)
}

func (backends *artifactContentBackends) Read(ctx context.Context, content domain.ArtifactContent) ([]byte, error) {
	provider := content.StorageProvider
	if provider == "" {
		provider = domain.ArtifactContentStorageProviderPostgres
	}
	backend, ok := backends.readers[provider]
	if !ok {
		return nil, fmt.Errorf("artifact content storage provider %q is not configured", provider)
	}
	return backend.Read(ctx, content)
}

func (backends *artifactContentBackends) CaptureProvider() domain.ArtifactContentStorageProvider {
	return backends.captureBackend.Provider()
}

type postgresArtifactContentBackend struct{}

func (postgresArtifactContentBackend) Provider() domain.ArtifactContentStorageProvider {
	return domain.ArtifactContentStorageProviderPostgres
}

func (postgresArtifactContentBackend) Capture(_ context.Context, _ domain.Artifact, input domain.ArtifactContentCapture, content []byte) (domain.ArtifactContentCapture, error) {
	input.StorageProvider = domain.ArtifactContentStorageProviderPostgres
	input.StorageKey = ""
	input.Content = append([]byte{}, content...)
	return input, nil
}

func (postgresArtifactContentBackend) Read(_ context.Context, content domain.ArtifactContent) ([]byte, error) {
	if content.Content == nil && content.SizeBytes > 0 {
		return nil, errors.New("retained artifact content bytes are missing from postgres storage")
	}
	bytes := append([]byte{}, content.Content...)
	if err := verifyArtifactContentBytes(content, bytes); err != nil {
		return nil, err
	}
	return bytes, nil
}

type filesystemArtifactContentBackend struct {
	rootDir string
}

type S3ArtifactContentBackendOptions struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
	HTTPClient      *http.Client
}

type s3ArtifactContentBackend struct {
	endpoint        *url.URL
	region          string
	bucket          string
	prefix          string
	accessKeyID     string
	secretAccessKey string
	forcePathStyle  bool
	client          *http.Client
}

func NewFilesystemArtifactContentBackend(rootDir string) (ArtifactContentBackend, error) {
	cleanRoot := strings.TrimSpace(rootDir)
	if cleanRoot == "" {
		return nil, errors.New("artifact content filesystem directory is required")
	}
	absRoot, err := filepath.Abs(cleanRoot)
	if err != nil {
		return nil, err
	}
	return &filesystemArtifactContentBackend{rootDir: absRoot}, nil
}

func (backend *filesystemArtifactContentBackend) Provider() domain.ArtifactContentStorageProvider {
	return domain.ArtifactContentStorageProviderFilesystem
}

func (backend *filesystemArtifactContentBackend) Capture(_ context.Context, artifact domain.Artifact, input domain.ArtifactContentCapture, content []byte) (domain.ArtifactContentCapture, error) {
	key := path.Join(artifact.ID.String(), input.SHA256)
	filePath, err := backend.pathForKey(key)
	if err != nil {
		return domain.ArtifactContentCapture{}, err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		return domain.ArtifactContentCapture{}, err
	}
	tmpPath := filepath.Join(filepath.Dir(filePath), "."+filepath.Base(filePath)+"."+uuid.NewString()+".tmp")
	if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
		return domain.ArtifactContentCapture{}, err
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		return domain.ArtifactContentCapture{}, err
	}
	input.StorageProvider = domain.ArtifactContentStorageProviderFilesystem
	input.StorageKey = key
	input.Content = nil
	return input, nil
}

func (backend *filesystemArtifactContentBackend) Read(_ context.Context, content domain.ArtifactContent) ([]byte, error) {
	filePath, err := backend.pathForKey(content.StorageKey)
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if err := verifyArtifactContentBytes(content, bytes); err != nil {
		return nil, err
	}
	return bytes, nil
}

func (backend *filesystemArtifactContentBackend) pathForKey(key string) (string, error) {
	if strings.TrimSpace(key) == "" || strings.Contains(key, "\x00") {
		return "", errors.New("artifact content storage key is invalid")
	}
	cleanKey := path.Clean("/" + key)
	relKey := strings.TrimPrefix(cleanKey, "/")
	if relKey == "." || relKey == "" || relKey != key || strings.HasPrefix(relKey, "../") || strings.Contains(relKey, "/../") {
		return "", errors.New("artifact content storage key is invalid")
	}
	fullPath := filepath.Join(backend.rootDir, filepath.FromSlash(relKey))
	cleanPath := filepath.Clean(fullPath)
	rootWithSeparator := filepath.Clean(backend.rootDir) + string(os.PathSeparator)
	if cleanPath != filepath.Clean(backend.rootDir) && !strings.HasPrefix(cleanPath, rootWithSeparator) {
		return "", errors.New("artifact content storage key escapes root")
	}
	return cleanPath, nil
}

func NewS3ArtifactContentBackend(options S3ArtifactContentBackendOptions) (ArtifactContentBackend, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(options.Endpoint), "/")
	if endpoint == "" {
		return nil, errors.New("artifact content S3 endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("artifact content S3 endpoint must use http or https")
	}
	bucket := strings.TrimSpace(options.Bucket)
	if bucket == "" || strings.Contains(bucket, "/") {
		return nil, errors.New("artifact content S3 bucket is required")
	}
	accessKeyID := strings.TrimSpace(options.AccessKeyID)
	secretAccessKey := strings.TrimSpace(options.SecretAccessKey)
	if accessKeyID == "" || secretAccessKey == "" {
		return nil, errors.New("artifact content S3 credentials are required")
	}
	region := strings.TrimSpace(options.Region)
	if region == "" {
		region = "us-east-1"
	}
	prefix, err := cleanS3Prefix(options.Prefix)
	if err != nil {
		return nil, err
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &s3ArtifactContentBackend{
		endpoint:        parsed,
		region:          region,
		bucket:          bucket,
		prefix:          prefix,
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		forcePathStyle:  options.ForcePathStyle,
		client:          client,
	}, nil
}

func (backend *s3ArtifactContentBackend) Provider() domain.ArtifactContentStorageProvider {
	return domain.ArtifactContentStorageProviderS3
}

func (backend *s3ArtifactContentBackend) Capture(ctx context.Context, artifact domain.Artifact, input domain.ArtifactContentCapture, content []byte) (domain.ArtifactContentCapture, error) {
	key := backend.objectKey(artifact.ID.String(), input.SHA256)
	req, err := backend.newRequest(ctx, http.MethodPut, key, bytes.NewReader(content))
	if err != nil {
		return domain.ArtifactContentCapture{}, err
	}
	req.ContentLength = int64(len(content))
	req.Header.Set("Content-Type", artifactContentStorageContentType(input.ContentType))
	req.Header.Set("X-Amz-Content-Sha256", input.SHA256)
	if err := backend.do(req, http.StatusOK, http.StatusNoContent); err != nil {
		return domain.ArtifactContentCapture{}, err
	}
	input.StorageProvider = domain.ArtifactContentStorageProviderS3
	input.StorageKey = key
	input.Content = nil
	return input, nil
}

func (backend *s3ArtifactContentBackend) Read(ctx context.Context, content domain.ArtifactContent) ([]byte, error) {
	req, err := backend.newRequest(ctx, http.MethodGet, content.StorageKey, nil)
	if err != nil {
		return nil, err
	}
	response, err := backend.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil, domain.ErrNotFound
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return nil, fmt.Errorf("artifact content S3 read failed: status=%d body=%q", response.StatusCode, string(body))
	}
	bytes, err := io.ReadAll(io.LimitReader(response.Body, content.SizeBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(bytes)) > content.SizeBytes {
		return nil, fmt.Errorf("retained artifact content size mismatch: metadata=%d actual>%d", content.SizeBytes, content.SizeBytes)
	}
	if err := verifyArtifactContentBytes(content, bytes); err != nil {
		return nil, err
	}
	return bytes, nil
}

func (backend *s3ArtifactContentBackend) do(req *http.Request, allowedStatuses ...int) error {
	response, err := backend.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	for _, status := range allowedStatuses {
		if response.StatusCode == status {
			return nil
		}
	}
	return fmt.Errorf("artifact content S3 request failed: method=%s status=%d", req.Method, response.StatusCode)
}

func (backend *s3ArtifactContentBackend) newRequest(ctx context.Context, method string, key string, body io.Reader) (*http.Request, error) {
	cleanKey, err := cleanS3Key(key)
	if err != nil {
		return nil, err
	}
	targetURL := backend.objectURL(cleanKey)
	req, err := http.NewRequestWithContext(ctx, method, targetURL.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
	signS3Request(req, backend.region, "s3", backend.accessKeyID, backend.secretAccessKey)
	return req, nil
}

func (backend *s3ArtifactContentBackend) objectURL(key string) *url.URL {
	target := *backend.endpoint
	if backend.forcePathStyle {
		target.Path = path.Join(target.EscapedPath(), backend.bucket, key)
		return &target
	}
	target.Host = backend.bucket + "." + target.Host
	target.Path = path.Join(target.EscapedPath(), key)
	return &target
}

func (backend *s3ArtifactContentBackend) objectKey(parts ...string) string {
	key := path.Join(parts...)
	if backend.prefix == "" {
		return key
	}
	return path.Join(backend.prefix, key)
}

func signS3Request(req *http.Request, region string, service string, accessKeyID string, secretAccessKey string) {
	amzDate := req.Header.Get("X-Amz-Date")
	date := amzDate[:8]
	payloadHash := req.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = "UNSIGNED-PAYLOAD"
		req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	}
	canonicalHeaders, signedHeaders := canonicalS3Headers(req.Header)
	canonicalQuery := req.URL.Query().Encode()
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL.EscapedPath()),
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	scope := strings.Join([]string{date, region, service, "aws4_request"}, "/")
	hashedCanonicalRequest := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hex.EncodeToString(hashedCanonicalRequest[:]),
	}, "\n")
	signature := hex.EncodeToString(hmacSHA256(signingKey(secretAccessKey, date, region, service), []byte(stringToSign)))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", accessKeyID, scope, signedHeaders, signature))
}

func canonicalS3Headers(headers http.Header) (string, string) {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		values := headers.Values(name)
		trimmed := make([]string, 0, len(values))
		for _, value := range values {
			trimmed = append(trimmed, strings.Join(strings.Fields(value), " "))
		}
		lines = append(lines, name+":"+strings.Join(trimmed, ","))
	}
	return strings.Join(lines, "\n") + "\n", strings.Join(names, ";")
}

func signingKey(secret string, date string, region string, service string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	regionKey := hmacSHA256(dateKey, []byte(region))
	serviceKey := hmacSHA256(regionKey, []byte(service))
	return hmacSHA256(serviceKey, []byte("aws4_request"))
}

func hmacSHA256(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}

func canonicalURI(value string) string {
	if value == "" {
		return "/"
	}
	return value
}

func cleanS3Prefix(value string) (string, error) {
	clean := strings.Trim(strings.TrimSpace(value), "/")
	if clean == "" {
		return "", nil
	}
	return cleanS3Key(clean)
}

func cleanS3Key(key string) (string, error) {
	if strings.TrimSpace(key) == "" || strings.Contains(key, "\x00") {
		return "", errors.New("artifact content S3 storage key is invalid")
	}
	clean := path.Clean("/" + key)
	rel := strings.TrimPrefix(clean, "/")
	if rel == "." || rel == "" || rel != key || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") {
		return "", errors.New("artifact content S3 storage key is invalid")
	}
	return rel, nil
}

func artifactContentStorageContentType(contentType string) string {
	clean := strings.TrimSpace(contentType)
	if clean == "" {
		return "application/octet-stream"
	}
	return clean
}

func verifyArtifactContentBytes(content domain.ArtifactContent, bytes []byte) error {
	if int64(len(bytes)) != content.SizeBytes {
		return fmt.Errorf("retained artifact content size mismatch: metadata=%d actual=%d", content.SizeBytes, len(bytes))
	}
	if content.SHA256 == "" {
		return nil
	}
	hash := sha256.Sum256(bytes)
	if got := hex.EncodeToString(hash[:]); got != content.SHA256 {
		return fmt.Errorf("retained artifact content sha256 mismatch: metadata=%s actual=%s", content.SHA256, got)
	}
	return nil
}
