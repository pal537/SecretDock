// VPS（SSH 主机）到期雷达：分级提醒 30/7/3/1 天、到期当天、过期后每日。
// 分级状态存 ssh_hosts.expiry_notify（JSON），发送成功才落库，重启不重复。
// 每天北京时间 9 点后才发送，多台合并一条。
package tg

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/example/secretdock/internal/db"
)

// expiryNotifyState 是每台主机的提醒状态：已发到的档位 + 过期阶段上次发送日期。
type expiryNotifyState struct {
	Band       string `json:"band"`
	OverdueDay string `json:"overdueDay"`
}

// expiryBand 把剩余天数映射到提醒档位，rank 越大越紧急。
func expiryBand(daysLeft int) (string, int) {
	switch {
	case daysLeft < 0:
		return "overdue", 6
	case daysLeft == 0:
		return "today", 5
	case daysLeft == 1:
		return "d1", 4
	case daysLeft <= 3:
		return "d3", 3
	case daysLeft <= 7:
		return "d7", 2
	case daysLeft <= 30:
		return "d30", 1
	default:
		return "", 0
	}
}

func expiryBandRank(band string) int {
	switch band {
	case "d30":
		return 1
	case "d7":
		return 2
	case "d3":
		return 3
	case "d1":
		return 4
	case "today":
		return 5
	case "overdue":
		return 6
	}
	return 0
}

func (s *Service) expiryLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.checkHostExpiries()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Service) checkHostExpiries() {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Now().In(loc)
	if now.Hour() < 9 {
		return
	}
	todayStr := now.Format("2006-01-02")

	hosts, err := s.db.ListSSHHosts(0)
	if err != nil || len(hosts) == 0 {
		return
	}

	type dueItem struct {
		host db.SSHHost
		band string
		next expiryNotifyState
	}
	var due []dueItem
	for _, h := range hosts {
		if !h.HasExpiry {
			continue
		}
		band, rank := expiryBand(h.DaysLeft)
		var st expiryNotifyState
		if h.ExpiryNotify != "" {
			_ = json.Unmarshal([]byte(h.ExpiryNotify), &st)
		}
		if rank == 0 {
			// 已不在提醒范围（如日期被改远）：静默清状态，不发消息
			if st.Band != "" {
				_ = s.db.SetSSHHostExpiryNotify(h.ID, "")
			}
			continue
		}
		need := rank > expiryBandRank(st.Band) || (band == "overdue" && st.OverdueDay != todayStr)
		if !need {
			continue
		}
		next := expiryNotifyState{Band: band, OverdueDay: st.OverdueDay}
		if band == "overdue" {
			next.OverdueDay = todayStr
		} else {
			next.OverdueDay = ""
		}
		due = append(due, dueItem{host: h, band: band, next: next})
	}
	if len(due) == 0 {
		return
	}

	chatStr := s.db.GetSetting("admin_chat_id")
	chatID, err := strconv.ParseInt(strings.TrimSpace(chatStr), 10, 64)
	if err != nil || chatID == 0 {
		return // TG 未配置：不落状态，配置好后会补发
	}
	s.mu.Lock()
	client := s.tgClient
	s.mu.Unlock()
	if client == nil {
		return
	}

	sort.Slice(due, func(i, j int) bool { return due[i].host.DaysLeft < due[j].host.DaysLeft })
	var b strings.Builder
	b.WriteString("⏰ VPS 到期提醒\n")
	for _, it := range due {
		h := it.host
		name := h.Name
		if name == "" {
			name = h.Host
		}
		switch {
		case h.DaysLeft < 0:
			fmt.Fprintf(&b, "• %s（%s）— ❗已过期 %d 天（%s 到期）\n", name, h.Host, -h.DaysLeft, h.ExpiryDate)
		case h.DaysLeft == 0:
			fmt.Fprintf(&b, "• %s（%s）— ⚠️ 今天到期\n", name, h.Host)
		default:
			fmt.Fprintf(&b, "• %s（%s）— 剩 %d 天（%s 到期）\n", name, h.Host, h.DaysLeft, h.ExpiryDate)
		}
	}
	b.WriteString("续费后请到机密库 SSH 页，点主机上的到期徽章「已续费」更新日期，提醒才会停。")

	log.Printf("tg: vps expiry alert: %d host(s) due", len(due))
	mid, err := client.SendMessage(chatID, b.String(), "", nil)
	if err != nil {
		log.Printf("tg: vps expiry alert send failed: %v", err)
		return
	}
	s.trackBot(chatID, mid)
	for _, it := range due {
		raw, _ := json.Marshal(it.next)
		_ = s.db.SetSSHHostExpiryNotify(it.host.ID, string(raw))
	}
	s.db.Audit("vps_expiry_notified", fmt.Sprintf("count=%d", len(due)), "telegram")
	log.Printf("tg: vps expiry alert sent for %d hosts", len(due))
}
