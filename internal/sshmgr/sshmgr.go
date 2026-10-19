// Package sshmgr 管理 SSH 主机连接：终端、SFTP、系统状态。
package sshmgr

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// HostInfo SSH 主机连接信息
type HostInfo struct {
	Host     string
	Port     int
	Username string
	Password string
}

// Addr 返回 host:port，IPv6 自动加括号
func (h HostInfo) Addr() string {
	if strings.Contains(h.Host, ":") && !strings.HasPrefix(h.Host, "[") {
		return "[" + h.Host + "]:" + strconv.Itoa(h.Port)
	}
	return net.JoinHostPort(h.Host, strconv.Itoa(h.Port))
}

// Fingerprint 计算主机公钥 SHA256 指纹
func Fingerprint(key ssh.PublicKey) string {
	sum := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.StdEncoding.EncodeToString(sum[:])
}

// Dial 建立 SSH 连接。knownFP 为空表示首次连接，返回指纹供确认。
func Dial(info HostInfo, knownFP string) (*ssh.Client, string, error) {
	var hostKeyCallback ssh.HostKeyCallback
	var gotFP string
	if knownFP == "" {
		// 首次：记录指纹，不校验（由调用方展示给用户确认）
		hostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			gotFP = Fingerprint(key)
			return nil
		}
	} else {
		hostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fp := Fingerprint(key)
			gotFP = fp
			if fp != knownFP {
				return fmt.Errorf("主机指纹不匹配（可能是中间人攻击）")
			}
			return nil
		}
	}
	cfg := &ssh.ClientConfig{
		User: info.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(info.Password),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", info.Addr(), cfg)
	if err != nil {
		return nil, "", err
	}
	return client, gotFP, nil
}

// PingTest TCP 连通性测试，返回延迟毫秒
func PingTest(host string, port int) (int64, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	// IPv6 处理
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		addr = "[" + host + "]:" + strconv.Itoa(port)
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return time.Since(start).Milliseconds(), nil
}

