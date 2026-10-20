package handlers

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/example/secretdock/internal/auth"
	"github.com/example/secretdock/internal/crypto"
	"github.com/example/secretdock/internal/db"
	"github.com/example/secretdock/internal/secpolicy"
	"github.com/example/secretdock/internal/sshmgr"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// ============ SSH 分类 ============

func (s *Server) handleSSHCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cats, err := s.DB.ListSSHCategories()
		if err != nil {
			fail(w, 500, "读取失败")
			return
		}
		ok(w, cats)
	case "POST":
		var req struct{ Name string `json:"name"` }
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "参数错误")
			return
		}
		id, err := s.DB.CreateSSHCategory(req.Name)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]int64{"id": id})
	default:
		fail(w, 405, "不支持")
	}
}

func (s *Server) handleSSHCategoryOne(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/ssh-categories/"), 10, 64)
	switch r.Method {
	case "PUT":
		var req struct{ Name string `json:"name"` }
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "参数错误")
			return
		}
		if err := s.DB.RenameSSHCategory(id, req.Name); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteSSHCategory(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "不支持")
	}
}

// ============ SSH 主机 ============

func (s *Server) handleSSHHosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		catID, _ := strconv.ParseInt(r.URL.Query().Get("categoryId"), 10, 64)
		hosts, err := s.DB.ListSSHHosts(catID)
		if err != nil {
			fail(w, 500, "读取失败")
			return
		}
		ok(w, hosts)
	case "POST":
		var req struct {
			CategoryID int64  `json:"categoryId"`
			Name       string `json:"name"`
			Host       string `json:"host"`
			Port       int    `json:"port"`
			Username   string `json:"username"`
			Password   string `json:"password"`
			PrivKey    string `json:"privKey"`
			KeyPass    string `json:"keyPass"`
			Country    string `json:"country"`
			Region     string `json:"region"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "参数错误")
			return
		}
		req.Host = strings.TrimSpace(req.Host)
		if req.Host == "" {
			fail(w, 400, "主机不能为空")
			return
		}
		if req.Password == "" && strings.TrimSpace(req.PrivKey) == "" {
			fail(w, 400, "密码和私钥至少填一个")
			return
		}
		if req.Port <= 0 || req.Port > 65535 {
			req.Port = 22
		}
		if req.Username == "" {
			req.Username = "root"
		}
		if req.CategoryID == 0 {
			id, _ := s.DB.FindOrCreateSSHCategory("未分类")
			req.CategoryID = id
		}
		enc, err := crypto.Encrypt(s.EncKey, []byte(req.Password))
		if err != nil {
			fail(w, 500, "加密失败")
			return
		}
		keyEnc, keyPassEnc := "", ""
		if strings.TrimSpace(req.PrivKey) != "" {
			k, err := crypto.Encrypt(s.EncKey, []byte(strings.TrimSpace(req.PrivKey)))
			if err != nil {
				fail(w, 500, "加密失败")
				return
			}
			keyEnc = k
			if req.KeyPass != "" {
				kp, err := crypto.Encrypt(s.EncKey, []byte(req.KeyPass))
				if err != nil {
					fail(w, 500, "加密失败")
					return
				}
				keyPassEnc = kp
			}
		}
		id, err := s.DB.CreateSSHHost(req.CategoryID, strings.TrimSpace(req.Name), req.Host, req.Port, req.Username, enc, keyEnc, keyPassEnc, strings.TrimSpace(req.Country), strings.TrimSpace(req.Region))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]int64{"id": id})
	default:
		fail(w, 405, "不支持")
	}
}

func (s *Server) handleSSHHostOne(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/ssh-hosts/"), 10, 64)
	switch r.Method {
	case "PUT":
		var req struct {
			CategoryID int64  `json:"categoryId"`
			Name       string `json:"name"`
			Host       string `json:"host"`
			Port       int    `json:"port"`
			Username   string `json:"username"`
			Password   string `json:"password"` // 空表示不改
			PrivKey    string `json:"privKey"`  // 空表示不改
			KeyPass    string `json:"keyPass"`  // 随私钥一同保存
			ClearKey   bool   `json:"clearKey"` // true 清除已存私钥
			Country    string `json:"country"`
			Region     string `json:"region"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "参数错误")
			return
		}
		enc := ""
		if req.Password != "" {
			e, err := crypto.Encrypt(s.EncKey, []byte(req.Password))
			if err != nil {
				fail(w, 500, "加密失败")
				return
			}
			enc = e
		}
		keyEnc, keyPassEnc := "", ""
		if strings.TrimSpace(req.PrivKey) != "" {
			k, err := crypto.Encrypt(s.EncKey, []byte(strings.TrimSpace(req.PrivKey)))
			if err != nil {
				fail(w, 500, "加密失败")
				return
			}
			keyEnc = k
			if req.KeyPass != "" {
				kp, err := crypto.Encrypt(s.EncKey, []byte(req.KeyPass))
				if err != nil {
					fail(w, 500, "加密失败")
					return
				}
				keyPassEnc = kp
			}
		}
		if err := s.DB.UpdateSSHHost(id, req.CategoryID, strings.TrimSpace(req.Name), strings.TrimSpace(req.Host), req.Port, req.Username, enc, keyEnc, keyPassEnc, strings.TrimSpace(req.Country), strings.TrimSpace(req.Region), req.ClearKey); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteSSHHost(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "不支持")
	}
}

// ============ Ping 测试 ============

func (s *Server) handleSSHTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := readJSON(r, &req); err != nil || len(req.IDs) == 0 {
		fail(w, 400, "参数错误")
		return
	}
	var wg sync.WaitGroup
	for _, id := range req.IDs {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			h, err := s.DB.GetSSHHost(id)
			if err != nil {
				return
			}
			lat, err := sshmgr.PingTest(h.Host, h.Port)
			if err != nil {
				s.DB.UpdateSSHHostStatus(id, "fail", 0)
			} else {
				s.DB.UpdateSSHHostStatus(id, "ok", lat)
			}
		}(id)
	}
	wg.Wait()
	ok(w, map[string]bool{"ok": true})
}

// friendlySSHError 把 SSH 错误翻译成中文
func friendlySSHError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unable to authenticate"):
		return fmt.Errorf("认证失败：用户名或密码错误")
	case strings.Contains(msg, "connection refused"):
		return fmt.Errorf("连接被拒绝：目标端口未开放或防火墙拦截")
	case strings.Contains(msg, "i/o timeout"):
		return fmt.Errorf("连接超时：主机不可达或网络不通")
	case strings.Contains(msg, "no route to host"):
		return fmt.Errorf("无路由：主机不可达")
	case strings.Contains(msg, "指纹不匹配"):
		return err
	default:
		return fmt.Errorf("连接失败: %s", msg)
	}
}

// ============ 系统状态 ============

func (s *Server) sshClientFor(id int64) (*ssh.Client, db.SSHHost, error) {
	h, err := s.DB.GetSSHHost(id)
	if err != nil {
		return nil, h, err
	}
	pw, err := crypto.Decrypt(s.EncKey, h.PasswordEnc)
	if err != nil {
		return nil, h, fmt.Errorf("密码解密失败")
	}
	var privKey, keyPass string
	if h.PrivKeyEnc != "" {
		k, err := crypto.Decrypt(s.EncKey, h.PrivKeyEnc)
		if err != nil {
			return nil, h, fmt.Errorf("私钥解密失败")
		}
		privKey = string(k)
		if h.KeyPassEnc != "" {
			kp, err := crypto.Decrypt(s.EncKey, h.KeyPassEnc)
			if err == nil {
				keyPass = string(kp)
			}
		}
	}
	// 主机指纹存在 settings 里：ssh_fp_<id>
	fp := s.DB.GetSetting(fmt.Sprintf("ssh_fp_%d", id))
	client, gotFP, err := sshmgr.Dial(sshmgr.HostInfo{
		Host:     h.Host,
		Port:     h.Port,
		Username: h.Username,
		Password: string(pw),
		PrivKey:  privKey,
		KeyPass:  keyPass,
	}, fp)
	if err != nil {
		return nil, h, friendlySSHError(err)
	}
	// 首次连接：保存指纹
	if fp == "" && gotFP != "" {
		s.DB.SetSetting(fmt.Sprintf("ssh_fp_%d", id), gotFP)
	}
	return client, h, nil
}

func (s *Server) handleSSHStats(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	client, _, err := s.sshClientFor(id)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer client.Close()
	st, err := sshmgr.GetStats(client)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	procs := sshmgr.GetTopProcs(client)
	ok(w, map[string]interface{}{"stats": st, "procs": procs})
}

func (s *Server) handleSSHDetectOS(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	client, _, err := s.sshClientFor(id)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer client.Close()
	oi, err := sshmgr.DetectOS(client)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	// 存库
	_ = s.DB.UpdateSSHHostOS(id, oi.ID, oi.Version)
	ok(w, map[string]interface{}{"os": oi})
}

func (s *Server) handleSSHNetSpeed(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	client, _, err := s.sshClientFor(id)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer client.Close()
	ns, err := sshmgr.GetNetSpeed(client)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]interface{}{"netspeed": ns})
}

// ============ SFTP 文件 ============

func (s *Server) handleSSHFiles(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/root"
	}
	client, _, err := s.sshClientFor(id)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer client.Close()
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		fail(w, 500, "SFTP 启动失败: "+err.Error())
		return
	}
	defer sftpClient.Close()
	entries, err := sftpClient.ReadDir(path)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	type FE struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		IsDir   bool   `json:"isDir"`
		ModTime int64  `json:"modTime"`
		Mode    string `json:"mode"`
	}
	out := []FE{}
	for _, e := range entries {
		out = append(out, FE{
			Name:    e.Name(),
			Size:    e.Size(),
			IsDir:   e.IsDir(),
			ModTime: e.ModTime().Unix(),
			Mode:    e.Mode().String(),
		})
	}
	ok(w, map[string]interface{}{"path": path, "files": out})
}

func (s *Server) handleSSHFileDownload(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	path := r.URL.Query().Get("path")
	if path == "" {
		fail(w, 400, "缺少 path")
		return
	}
	client, _, err := s.sshClientFor(id)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer client.Close()
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		fail(w, 500, "SFTP 启动失败")
		return
	}
	defer sftpClient.Close()
	f, err := sftpClient.Open(path)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer f.Close()
	fi, _ := f.Stat()
	name := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		name = path[i+1:]
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	if fi != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	}
	io.Copy(w, f)
}

func (s *Server) handleSSHUpload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   int64  `json:"id"`
		Path string `json:"path"`
		Data string `json:"data"` // base64
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, 400, "参数错误")
		return
	}
	if req.Path == "" || req.Data == "" {
		fail(w, 400, "参数错误")
		return
	}
	// 限制 50MB
	if len(req.Data) > 70*1024*1024 {
		fail(w, 400, "文件太大（限50MB）")
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		fail(w, 400, "数据错误")
		return
	}
	client, _, err := s.sshClientFor(req.ID)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer client.Close()
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		fail(w, 500, "SFTP 启动失败")
		return
	}
	defer sftpClient.Close()
	f, err := sftpClient.Create(req.Path)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]bool{"ok": true})
}

// handleSSHDiag 诊断：走和 WS 一样的步骤，返回 JSON 看哪步失败
func (s *Server) handleSSHDiag(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	steps := []string{}
	client, _, err := s.sshClientFor(id)
	if err != nil {
		ok(w, map[string]interface{}{"ok": false, "step": "dial", "error": err.Error(), "steps": steps})
		return
	}
	defer client.Close()
	steps = append(steps, "dial")
	sess, err := client.NewSession()
	if err != nil {
		ok(w, map[string]interface{}{"ok": false, "step": "new_session", "error": err.Error(), "steps": steps})
		return
	}
	defer sess.Close()
	steps = append(steps, "new_session")
	modes := ssh.TerminalModes{ssh.ECHO: 1}
	if err := sess.RequestPty("xterm-256color", 30, 120, modes); err != nil {
		ok(w, map[string]interface{}{"ok": false, "step": "pty", "error": err.Error(), "steps": steps})
		return
	}
	steps = append(steps, "pty")
	if _, err := sess.StdinPipe(); err != nil {
		ok(w, map[string]interface{}{"ok": false, "step": "stdin", "error": err.Error(), "steps": steps})
		return
	}
	steps = append(steps, "stdin")
	if _, err := sess.StdoutPipe(); err != nil {
		ok(w, map[string]interface{}{"ok": false, "step": "stdout", "error": err.Error(), "steps": steps})
		return
	}
	steps = append(steps, "stdout")
	if err := sess.Shell(); err != nil {
		ok(w, map[string]interface{}{"ok": false, "step": "shell", "error": err.Error(), "steps": steps})
		return
	}
	steps = append(steps, "shell")
	ok(w, map[string]interface{}{"ok": true, "steps": steps})
}

// ============ WebSocket 终端 ============

var sshWSTerminalUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WS 协议：客户端发 JSON {type:"input", data:"..."} / {type:"resize", cols, rows}
// 服务端发 JSON {type:"output", data:"..."} / {type:"error", data:"..."}
func (s *Server) handleSSHWS(w http.ResponseWriter, r *http.Request) {
	// WebSocket 也要鉴权：检查 session cookie（绑定登录 IP）
	ip := secpolicy.ClientIP(r)
	if !s.Auth.Valid(auth.CookieToken(r), ip) {
		http.Error(w, "未登录", 401)
		return
	}
	if ok, reason := s.checkIPPolicy(ip); !ok {
		s.Auth.Revoke(auth.CookieToken(r))
		http.Error(w, "当前 IP 不允许访问："+reason, 403)
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 30
	}
	client, _, err := s.sshClientFor(id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		http.Error(w, "创建会话失败: "+err.Error(), 500)
		return
	}
	// PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		sess.Close()
		client.Close()
		http.Error(w, "PTY 失败: "+err.Error(), 500)
		return
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		sess.Close()
		client.Close()
		http.Error(w, "stdin 失败", 500)
		return
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		client.Close()
		http.Error(w, "stdout 失败", 500)
		return
	}
	stderr, _ := sess.StderrPipe()
	if err := sess.Shell(); err != nil {
		sess.Close()
		client.Close()
		http.Error(w, "shell 失败: "+err.Error(), 500)
		return
	}
	ws, err := sshWSTerminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		sess.Close()
		client.Close()
		return
	}
	defer ws.Close()
	defer sess.Close()
	defer client.Close()

	// 空闲超时：30 分钟无输入断开
	idleTimer := time.AfterFunc(30*time.Minute, func() {
		ws.Close()
	})
	defer idleTimer.Stop()
	resetIdle := func() {
		idleTimer.Reset(30 * time.Minute)
	}

	// SSH -> WS
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				ws.WriteJSON(map[string]string{"type": "output", "data": string(buf[:n])})
			}
			if err != nil {
				ws.WriteJSON(map[string]string{"type": "error", "data": "连接已断开"})
				ws.Close()
				return
			}
		}
	}()
	go func() {
		if stderr != nil {
			buf := make([]byte, 4096)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					ws.WriteJSON(map[string]string{"type": "output", "data": string(buf[:n])})
				}
				if err != nil {
					return
				}
			}
		}
	}()

	// WS -> SSH
	for {
		var msg struct {
			Type string `json:"type"`
			Data string `json:"data"`
			Cols int    `json:"cols"`
			Rows int    `json:"rows"`
		}
		if err := ws.ReadJSON(&msg); err != nil {
			return
		}
		resetIdle()
		switch msg.Type {
		case "input":
			stdin.Write([]byte(msg.Data))
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				sess.WindowChange(msg.Rows, msg.Cols)
			}
		case "ping":
			ws.WriteJSON(map[string]string{"type": "pong", "data": msg.Data})
		}
	}
}
