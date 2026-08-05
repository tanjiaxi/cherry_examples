// Package activity contains the persistence boundary for asynchronous activity
// state.  It uses DynamoDB's HTTPS JSON protocol directly so the demo does not
// hide its correctness rules behind a large SDK.
package activity

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type DynamoConfig struct {
	Endpoint     string
	Region       string
	AccessKey    string
	SecretKey    string
	SessionToken string
	SignRequests bool
	Table        string
	Timeout      time.Duration
}

type DynamoClient struct {
	cfg  DynamoConfig
	url  *url.URL
	http *http.Client
}

type DynamoError struct {
	Status  int
	Code    string `json:"__type"`
	Message string `json:"message"`
	Raw     string `json:"-"`
}

func (e *DynamoError) Error() string {
	return fmt.Sprintf("dynamodb status=%d code=%s message=%s", e.Status, e.Code, e.Message)
}

func NewDynamoClient(cfg DynamoConfig) (*DynamoClient, error) {
	if cfg.Endpoint == "" || cfg.Table == "" {
		return nil, fmt.Errorf("dynamodb endpoint and table are required")
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid dynamodb endpoint %q", cfg.Endpoint)
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 3 * time.Second
	}
	if cfg.SignRequests && (cfg.AccessKey == "" || cfg.SecretKey == "") {
		return nil, fmt.Errorf("dynamodb access_key and secret_key required when signing is enabled")
	}
	return &DynamoClient{cfg: cfg, url: u, http: &http.Client{Timeout: cfg.Timeout}}, nil
}

func (c *DynamoClient) transactWrite(ctx context.Context, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal DynamoDB transaction: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810.TransactWriteItems")
	if c.cfg.SignRequests {
		c.sign(req, payload, time.Now().UTC())
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("DynamoDB request: %w", err)
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return fmt.Errorf("read DynamoDB response: %w", readErr)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	ddbErr := &DynamoError{Status: resp.StatusCode, Message: string(data), Raw: string(data)}
	_ = json.Unmarshal(data, ddbErr)
	return ddbErr
}

func (c *DynamoClient) sign(req *http.Request, payload []byte, now time.Time) {
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	payloadHash := sha256Hex(payload)
	req.Header.Set("X-Amz-Date", amzDate)
	if c.cfg.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", c.cfg.SessionToken)
	}
	headers := []string{"content-type", "host", "x-amz-date", "x-amz-target"}
	if c.cfg.SessionToken != "" {
		headers = append(headers, "x-amz-security-token")
	}
	var canonical strings.Builder
	for _, key := range headers {
		value := req.Header.Get(key)
		if key == "host" {
			value = req.URL.Host
		}
		canonical.WriteString(key)
		canonical.WriteByte(':')
		canonical.WriteString(strings.Join(strings.Fields(value), " "))
		canonical.WriteByte('\n')
	}
	signedHeaders := strings.Join(headers, ";")
	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalRequest := strings.Join([]string{
		req.Method, canonicalURI, req.URL.RawQuery, canonical.String(), signedHeaders, payloadHash,
	}, "\n")
	credentialScope := strings.Join([]string{date, c.cfg.Region, "dynamodb", "aws4_request"}, "/")
	stringToSign := strings.Join([]string{"AWS4-HMAC-SHA256", amzDate, credentialScope, sha256Hex([]byte(canonicalRequest))}, "\n")
	signingKey := hmacSHA256([]byte("AWS4"+c.cfg.SecretKey), date)
	signingKey = hmacSHA256(signingKey, c.cfg.Region)
	signingKey = hmacSHA256(signingKey, "dynamodb")
	signingKey = hmacSHA256(signingKey, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", c.cfg.AccessKey, credentialScope, signedHeaders, signature))
}

func hmacSHA256(key []byte, value string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(value))
	return h.Sum(nil)
}
func sha256Hex(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func attributeS(value string) map[string]string { return map[string]string{"S": value} }
func attributeN(value int64) map[string]string {
	return map[string]string{"N": strconv.FormatInt(value, 10)}
}
