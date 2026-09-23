package maxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
)

type uploadReservation struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// UploadFile uploads an existing file to MAX and returns its message token.
func (c *Client) UploadFile(ctx context.Context, filename, contentType string, content io.Reader) (string, error) {
	var reservation uploadReservation
	if err := c.do(ctx, http.MethodPost, "/uploads", url.Values{"type": {"file"}}, nil, &reservation, false); err != nil {
		return "", err
	}
	if reservation.URL == "" {
		return "", fmt.Errorf("MAX upload reservation is incomplete")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="data"; filename=%q`, filepath.Base(filename)))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", fmt.Errorf("create MAX upload form: %w", err)
	}
	if _, err := io.Copy(part, content); err != nil {
		return "", fmt.Errorf("copy MAX upload content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close MAX upload form: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, reservation.URL, &body)
	if err != nil {
		return "", fmt.Errorf("create MAX upload request: %w", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", c.token)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("upload file to MAX: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("MAX upload returned status %d", response.StatusCode)
	}
	var uploaded struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBody)).Decode(&uploaded); err != nil && err != io.EOF {
		return "", fmt.Errorf("decode MAX upload response: %w", err)
	}
	if uploaded.Token != "" {
		return uploaded.Token, nil
	}
	if reservation.Token == "" {
		return "", fmt.Errorf("MAX upload response is missing token")
	}
	return reservation.Token, nil
}
