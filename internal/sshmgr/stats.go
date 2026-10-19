package sshmgr

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SysStats 系统状态
type SysStats struct {
	CPUPercent  float64 `json:"cpuPercent"`
	MemPercent  float64 `json:"memPercent"`
	MemUsedMB   int64   `json:"memUsedMB"`
	MemTotalMB  int64   `json:"memTotalMB"`
	DiskPercent float64 `json:"diskPercent"`
	DiskUsedGB  float64 `json:"diskUsedGB"`
	DiskTotalGB float64 `json:"diskTotalGB"`
	Uptime      string  `json:"uptime"`
}

// runSSH 在已建立的 client 上执行命令并返回输出
func runSSH(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}

// GetStats 采集系统状态（CPU/内存/磁盘/运行时间）
func GetStats(client *ssh.Client) (*SysStats, error) {
	st := &SysStats{}
	// 内存：free -m
	if out, err := runSSH(client, "free -m | awk 'NR==2{print $2,$3}'"); err == nil {
		f := strings.Fields(out)
		if len(f) == 2 {
			total, _ := strconv.ParseInt(f[0], 10, 64)
			used, _ := strconv.ParseInt(f[1], 10, 64)
			st.MemTotalMB = total
			st.MemUsedMB = used
			if total > 0 {
				st.MemPercent = float64(used) / float64(total) * 100
			}
		}
	}
	// CPU：/proc/stat 两次采样算使用率
	if out, err := runSSH(client, "grep '^cpu ' /proc/stat; sleep 1; grep '^cpu ' /proc/stat"); err == nil {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) == 2 {
			p1 := strings.Fields(lines[0])
			p2 := strings.Fields(lines[1])
			if len(p1) >= 5 && len(p2) >= 5 {
				var t1, i1, t2, i2 float64
				for _, v := range p1[1:] {
					n, _ := strconv.ParseFloat(v, 64)
					t1 += n
				}
				i1, _ = strconv.ParseFloat(p1[4], 64)
				for _, v := range p2[1:] {
					n, _ := strconv.ParseFloat(v, 64)
					t2 += n
				}
				i2, _ = strconv.ParseFloat(p2[4], 64)
				if t2 > t1 {
					st.CPUPercent = (1 - (i2-i1)/(t2-t1)) * 100
				}
			}
		}
	}
	// 磁盘：根分区 df
	if out, err := runSSH(client, "df -BG / | awk 'NR==2{print $2,$3,$5}'"); err == nil {
		f := strings.Fields(out)
		if len(f) == 3 {
			total, _ := strconv.ParseFloat(strings.TrimSuffix(f[0], "G"), 64)
			used, _ := strconv.ParseFloat(strings.TrimSuffix(f[1], "G"), 64)
			pct, _ := strconv.ParseFloat(strings.TrimSuffix(f[2], "%"), 64)
			st.DiskTotalGB = total
			st.DiskUsedGB = used
			st.DiskPercent = pct
		}
	}
	// 运行时间
	if out, err := runSSH(client, "uptime -p 2>/dev/null || uptime"); err == nil {
		st.Uptime = strings.TrimSpace(out)
	}
	return st, nil
}

// TopProcesses 取占用最高的进程列表（给左侧面板用）
type ProcInfo struct {
	CPU  string `json:"cpu"`
	Mem  string `json:"mem"`
	Cmd  string `json:"cmd"`
}

// GetTopProcs 取 top 8 进程
func GetTopProcs(client *ssh.Client) []ProcInfo {
	out, err := runSSH(client, "ps -eo pcpu,pmem,comm --sort=-pcpu | head -9 | tail -8")
	if err != nil {
		return nil
	}
	var procs []ProcInfo
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 {
			procs = append(procs, ProcInfo{CPU: f[0], Mem: f[1], Cmd: strings.Join(f[2:], " ")})
		}
	}
	return procs
}

// NetStat 取网络流量（简化版）
func GetNetStat(client *ssh.Client) string {
	out, err := runSSH(client, "cat /proc/net/dev | awk -F'[: ]+' 'NR>2{rx+=$3;tx+=$11} END{print rx,tx}'")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

var _ = fmt.Sprintf // 占位，避免未使用

// NetSpeed 网速
type NetSpeed struct {
	RxKBs float64 `json:"rxKBs"`
	TxKBs float64 `json:"txKBs"`
}

// GetNetSpeed 取网速（两次采样 /proc/net/dev，间隔1秒）
func GetNetSpeed(client *ssh.Client) (*NetSpeed, error) {
	ns := &NetSpeed{}
	out, err := runSSH(client, "cat /proc/net/dev; sleep 1; cat /proc/net/dev")
	if err != nil {
		return ns, err
	}
	// 解析两次采样，取主网卡（非lo，收发包最多的）
	parse := func(text string) map[string][2]int64 {
		m := map[string][2]int64{}
		for _, ln := range strings.Split(text, "\n") {
			ln = strings.TrimSpace(ln)
			if !strings.Contains(ln, ":") {
				continue
			}
			parts := strings.SplitN(ln, ":", 2)
			iface := strings.TrimSpace(parts[0])
			if iface == "lo" {
				continue
			}
			f := strings.Fields(parts[1])
			if len(f) < 10 {
				continue
			}
			rx, _ := strconv.ParseInt(f[0], 10, 64)
			tx, _ := strconv.ParseInt(f[8], 10, 64)
			m[iface] = [2]int64{rx, tx}
		}
		return m
	}
	// 按空行或重复表头分割两次输出
	blocks := strings.Split(out, "Inter-")
	var b1, b2 string
	if len(blocks) >= 3 {
		b1 = "Inter-" + blocks[1]
		b2 = "Inter-" + blocks[2]
	} else {
		return ns, fmt.Errorf("parse failed")
	}
	m1, m2 := parse(b1), parse(b2)
	// 选流量最大的网卡
	var best string
	var bestTotal int64
	for iface, v := range m2 {
		if v1, ok := m1[iface]; ok {
			total := v[0] + v[1] - v1[0] - v1[1]
			if total > bestTotal {
				bestTotal = total
				best = iface
			}
		}
	}
	if best != "" {
		v1, v2 := m1[best], m2[best]
		ns.RxKBs = float64(v2[0]-v1[0]) / 1024
		ns.TxKBs = float64(v2[1]-v1[1]) / 1024
		if ns.RxKBs < 0 {
			ns.RxKBs = 0
		}
		if ns.TxKBs < 0 {
			ns.TxKBs = 0
		}
	}
	return ns, nil
}

// OSInfo 系统信息
type OSInfo struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// DetectOS 识别系统（读 /etc/os-release）
func DetectOS(client *ssh.Client) (*OSInfo, error) {
	oi := &OSInfo{}
	out, err := runSSH(client, "cat /etc/os-release 2>/dev/null | grep -E '^(ID|VERSION_ID)='")
	if err != nil {
		return oi, err
	}
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "ID=") {
			oi.ID = strings.Trim(strings.TrimPrefix(ln, "ID="), "\"'")
		} else if strings.HasPrefix(ln, "VERSION_ID=") {
			oi.Version = strings.Trim(strings.TrimPrefix(ln, "VERSION_ID="), "\"'")
		}
	}
	return oi, nil
}
