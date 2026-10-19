// 连通性测试：单节点 TCP 拨测，订阅 HTTP 拨测
package node

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Result struct {
	OK      bool
	Latency int64 // ms
	Err     string
}

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

// Test 测试节点/订阅连通性
// 单节点：TCP 连接 host:port；订阅：HTTP GET
func Test(rawURL string) Result {
	info := Parse(rawURL)
	if info.Kind == "sub" {
		return testHTTP(rawURL)
	}
	if info.Host == "" || info.Port == 0 {
		return Result{OK: false, Err: "无法解析地址"}
	}
	return testTCP(info.Host, info.Port)
}

func testTCP(host string, port int) Result {
	start := time.Now()
	d := net.Dialer{Timeout: 8 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	lat := time.Since(start).Milliseconds()
	if err != nil {
		return Result{OK: false, Latency: lat, Err: shortErr(err)}
	}
	conn.Close()
	return Result{OK: true, Latency: lat}
}

func testHTTP(rawURL string) Result {
	start := time.Now()
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return Result{OK: false, Err: "URL 无效"}
	}
	req.Header.Set("User-Agent", "clash")
	resp, err := httpClient.Do(req)
	lat := time.Since(start).Milliseconds()
	if err != nil {
		return Result{OK: false, Latency: lat, Err: shortErr(err)}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return Result{OK: true, Latency: lat}
	}
	return Result{OK: false, Latency: lat, Err: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

func shortErr(err error) string {
	s := err.Error()
	// 精简常见错误
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return "连接超时"
	}
	lower := s
	for _, sub := range []string{"connection refused", "no such host", "network is unreachable", "i/o timeout"} {
		if contains(lower, sub) {
			switch sub {
			case "connection refused":
				return "连接被拒绝"
			case "no such host":
				return "域名解析失败"
			case "network is unreachable":
				return "网络不可达"
			case "i/o timeout":
				return "连接超时"
			}
		}
	}
	if len(s) > 60 {
		s = s[:60] + "…"
	}
	return s
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
