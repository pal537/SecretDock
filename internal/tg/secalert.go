// Security login alerts: Telegram notifications for login events with a
// one-click IP ban button. Alert messages self-destruct after 3 minutes.
package tg

import (
	"fmt"
	"log"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// Security alert kinds.
const (
	SecAlertLogin    = "login"     // successful website login
	SecAlertNewIP    = "new_ip"    // successful login from a never-seen IP
	SecAlertPassFail = "pass_fail" // wrong login password
	SecAlertLocked   = "locked"    // IP locked after too many failures
)

// secAlertTTL is how long an alert message stays in the chat before
// the bot deletes it.
const secAlertTTL = 3 * time.Minute

// SendSecurityAlert sends a login-security alert to the admin chat with a
// one-click ban button, then deletes the message after secAlertTTL.
// It is safe to call when the bot is not configured (no-op).
// kind is one of the SecAlert* constants.
func (s *Service) SendSecurityAlert(kind, ip, country string) {
	s.mu.Lock()
	client := s.tgClient
	s.mu.Unlock()
	if client == nil {
		return
	}
	chatStr := s.db.GetSetting("admin_chat_id")
	chatID, err := strconv.ParseInt(strings.TrimSpace(chatStr), 10, 64)
	if err != nil || chatID == 0 {
		return
	}
	if a, err := netip.ParseAddr(strings.TrimSpace(ip)); err == nil {
		ip = a.String()
	}

	var title string
	switch kind {
	case SecAlertNewIP:
		title = "🆕 新 IP 登录"
	case SecAlertPassFail:
		title = "⚠️ 登录密码错误"
	case SecAlertLocked:
		title = "🚫 登录错误过多，IP 已锁定"
	default:
		title = "✅ 网站登录成功"
	}
	loc := country
	if loc == "" {
		loc = "未知"
	}
	// 北京时间
	loc_CN, _ := time.LoadLocation("Asia/Shanghai")
	if loc_CN == nil {
		loc_CN = time.FixedZone("CST", 8*3600)
	}
	text := fmt.Sprintf("%s\nIP：<code>%s</code>\n归属地：%s\n时间：%s",
		title, ip, loc, time.Now().In(loc_CN).Format("2006-01-02 15:04:05"))
	kb := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		BtnRow(Btn("⛔ 一键封禁此 IP", "sec:ban:"+ip)),
	}}
	msgID, err := client.SendMessage(chatID, text, "HTML", kb)
	if err != nil {
		log.Printf("tg: security alert send failed: %v", err)
		return
	}
	// Self-destruct after 3 minutes.
	go func() {
		time.Sleep(secAlertTTL)
		if err := client.DeleteMessage(chatID, msgID); err != nil {
			log.Printf("tg: security alert delete failed: %v", err)
		}
	}()
}

// handleSecBanCallback handles the "sec:ban:<ip>" one-click ban button.
// It is called from handleCallback after the admin-chat check.
func (s *Service) handleSecBanCallback(client *Client, chatID int64, cb *CallbackQuery) {
	ip := strings.TrimPrefix(cb.Data, "sec:ban:")
	a, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		client.AnswerCallbackQuery(cb.ID, "IP 无效")
		return
	}
	ip = a.String()
	if s.BanIPFunc == nil {
		client.AnswerCallbackQuery(cb.ID, "封禁功能未接线")
		return
	}
	if err := s.BanIPFunc(ip, "TG 一键封禁"); err != nil {
		client.AnswerCallbackQuery(cb.ID, "封禁失败："+err.Error())
		return
	}
	client.AnswerCallbackQuery(cb.ID, "已封禁 "+ip)
	// Mark the alert message as actioned (it self-destructs in 3 min anyway).
	if cb.Message != nil {
		_, _ = client.SendMessage(chatID,
			fmt.Sprintf("⛔ 已封禁 IP <code>%s</code>，该 IP 无法再访问网站。\n网站后台「登录安全」→ 已封禁 IP 可解封；服务器上可用 <code>secretdock unban %s</code> 解封。",
				ip, ip), "HTML", nil)
	}
}
