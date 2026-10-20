package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Media generation (image / video) for OpenAI-shaped providers.
// Endpoints are derived from the chat path:
//   .../chat/completions -> .../images/generations
//   .../chat/completions -> .../video/submit, .../video/status (async)
// Providers shape these slightly differently; response parsing accepts
// the common variants. Anthropic-format configs have no standard media
// API and report an error instead of guessing.

func mediaBase(cfg Config) (string, error) {
	if cfg.Format == "anthropic" {
		return "", fmt.Errorf("这个商家没有标准的图片/视频生成接口")
	}
	base := strings.TrimSuffix(cfg.ChatPath, "/chat/completions")
	return JoinURL(cfg.BaseURL, base), nil
}

func mediaHTTP(ctx context.Context, cfg Config, url string, body interface{}) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	cl := &http.Client{Timeout: 240 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		var e struct {
			Message string `json:"message"`
			Error   struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil {
			if e.Message != "" {
				msg = e.Message
			} else if e.Error.Message != "" {
				msg = e.Error.Message
			}
		}
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return nil, fmt.Errorf("商家接口返回 %d: %s", resp.StatusCode, msg)
	}
	return data, nil
}

// GenerateImage runs one image generation. It returns either a temporary
// provider URL (caller must download it promptly — they expire) or
// decoded base64 bytes when the provider answers inline. inputImage,
// when non-nil, is sent as the source image (edit / img2img models
// require one).
func GenerateImage(ctx context.Context, cfg Config, prompt string, inputImage []byte) (string, []byte, error) {
	base, err := mediaBase(cfg)
	if err != nil {
		return "", nil, err
	}
	body := map[string]interface{}{
		"model": cfg.Model, "prompt": prompt,
		"image_size": "1024x1024", "size": "1024x1024",
	}
	if len(inputImage) > 0 {
		mime := http.DetectContentType(inputImage)
		body["image"] = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(inputImage)
	}
	data, err := mediaHTTP(ctx, cfg, base+"/images/generations", body)
	if err != nil {
		return "", nil, err
	}
	var r struct {
		Images []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"images"`
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", nil, fmt.Errorf("图片接口返回无法解析")
	}
	pick := func(url, b64 string) (string, []byte, bool) {
		if url != "" {
			return url, nil, true
		}
		if b64 != "" {
			raw, err := base64.StdEncoding.DecodeString(b64)
			if err == nil {
				return "", raw, true
			}
		}
		return "", nil, false
	}
	for _, im := range r.Images {
		if u, b, ok := pick(im.URL, im.B64JSON); ok {
			return u, b, nil
		}
	}
	for _, im := range r.Data {
		if u, b, ok := pick(im.URL, im.B64JSON); ok {
			return u, b, nil
		}
	}
	return "", nil, fmt.Errorf("图片接口没有返回图片")
}

// SubmitVideo starts an async video generation job and returns its id.
// inputImage, when non-nil, is the source frame for image-to-video
// models.
func SubmitVideo(ctx context.Context, cfg Config, prompt string, inputImage []byte) (string, error) {
	base, err := mediaBase(cfg)
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{
		"model": cfg.Model, "prompt": prompt, "image_size": "1280x720",
	}
	if len(inputImage) > 0 {
		mime := http.DetectContentType(inputImage)
		body["image"] = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(inputImage)
	}
	data, err := mediaHTTP(ctx, cfg, base+"/video/submit", body)
	if err != nil {
		return "", err
	}
	var r struct {
		RequestID string `json:"requestId"`
		ID        string `json:"id"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", fmt.Errorf("视频接口返回无法解析")
	}
	if r.RequestID != "" {
		return r.RequestID, nil
	}
	if r.ID != "" {
		return r.ID, nil
	}
	return "", fmt.Errorf("视频接口没有返回任务编号")
}

// VideoStatus polls one video job. status is the provider's word
// ("Succeed" / "Failed" / anything else = still running).
func VideoStatus(ctx context.Context, cfg Config, reqID string) (status, videoURL, reason string, err error) {
	base, err := mediaBase(cfg)
	if err != nil {
		return "", "", "", err
	}
	data, err := mediaHTTP(ctx, cfg, base+"/video/status", map[string]interface{}{"requestId": reqID})
	if err != nil {
		return "", "", "", err
	}
	var r struct {
		Status  string `json:"status"`
		Reason  string `json:"reason"`
		Results struct {
			Videos []struct {
				URL string `json:"url"`
			} `json:"videos"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", "", "", fmt.Errorf("视频状态返回无法解析")
	}
	if len(r.Results.Videos) > 0 {
		videoURL = r.Results.Videos[0].URL
	}
	return r.Status, videoURL, r.Reason, nil
}
