// SecretDock Telegram bot service: free-text add flow and password-gated
// query flow, with full chat-history cleanup.
package tg

import (
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/example/secretdock/internal/auth"
	"github.com/example/secretdock/internal/crypto"
	"github.com/example/secretdock/internal/db"
	"github.com/example/secretdock/internal/node"
	"github.com/example/secretdock/internal/img"
	"github.com/example/secretdock/internal/parse"
)

// pending kinds
const (
	pAddChoice  = "add_choice"
	pAddConfirm = "add_confirm"
	pQpwd       = "qpwd"
	pQChoice    = "q_choice"

	pAddNeedMerchant    = "add_need_merchant"
	pScriptNeedName     = "script_need_name"
	pManualPick         = "manual_pick"
	pManualWait         = "manual_wait"
	pSearchPick         = "search_pick"
	pSearchKeyword      = "search_keyword"
	pSearchQpwd         = "search_qpwd"
	pManualSection      = "manual_section"
	pSearchSection      = "search_section"
	pRemindTime         = "remind_time"
	pRemindConfirm      = "remind_confirm"
	pNodeChoice         = "node_choice"
	pNeedCategory       = "need_category"
	pNewCategory        = "new_category"
	pSSHNeedName        = "ssh_need_name"
)

type pending struct {
	kind         string
	merchantName string
	merchantID   int64
	entries      []parse.Entry
	cands        []db.Merchant
	qpTries      int
	expires      time.Time

	addText    string

	scriptName    string
	scriptContent string
	scriptVer     string
	scriptDesc    string
	scriptAuthor  string

	manualSection string // "merchant" | "link" | "image" | "file"
	manualID      int64
	manualName    string
	scriptCatID   int64
	// reminder wizard
	remTitle    string
	remNote     string
	remCatID    int64
	remCatName  string
	remDate     string // YYYY-MM-DD
	remTime     string // HH:mm
	remRepeat   string // none|daily|weekly|monthly|yearly
	remDetail   string
	// ssh wizard
	sshHost     string
	sshPort     int
	sshUser     string
	sshPassEnc  string
	// unified category selection
	catSection string
	catIDs     []int64
	catTimer   *time.Timer
}

type qpFail struct {
	count int
	first time.Time
}

// banDurations: escalating bans for repeated query-password failures.
var banDurations = []time.Duration{
	30 * time.Minute,
	3 * time.Hour,
	24 * time.Hour,
	3 * 24 * time.Hour,
	7 * 24 * time.Hour,
	0, // permanent
}

func banDurationText(d time.Duration) string {
	if d == 0 {
		return "永久"
	}
	if d >= 24*time.Hour {
		return strconv.Itoa(int(d.Hours() / 24)) + "天"
	}
	if d >= time.Hour {
		return strconv.Itoa(int(d.Hours())) + "小时"
	}
	return strconv.Itoa(int(d.Minutes())) + "分钟"
}

// banStatus returns (banned, remainingText).
func (s *Service) banStatus() (bool, string) {
	until, _ := strconv.ParseInt(s.db.GetSetting("tg_banned_until"), 10, 64)
	if until == -1 {
		return true, "永久"
	}
	if until > time.Now().Unix() {
		left := time.Until(time.Unix(until, 0)).Round(time.Minute)
		if left < time.Minute {
			left = time.Minute
		}
		return true, left.String()
	}
	return false, ""
}

func (s *Service) applyBan() string {
	n, _ := strconv.Atoi(s.db.GetSetting("tg_ban_count"))
	n++
	idx := n - 1
	if idx >= len(banDurations) {
		idx = len(banDurations) - 1
	}
	d := banDurations[idx]
	_ = s.db.SetSetting("tg_ban_count", strconv.Itoa(n))
	if d == 0 {
		_ = s.db.SetSetting("tg_banned_until", "-1")
	} else {
		_ = s.db.SetSetting("tg_banned_until", strconv.FormatInt(time.Now().Add(d).Unix(), 10))
	}
	return "第" + strconv.Itoa(n) + "次封禁（" + banDurationText(d) + "）"
}

// Service runs the long-poll loop.
type Service struct {
	db      *db.DB
	encKey  []byte
	dataDir string

	mu      sync.Mutex
	pending map[int64]*pending
	qpFails map[int64]*qpFail
	tgClient *Client

	// BanIPFunc bans an IP from the website (one-click ban from security
	// alerts). Wired by main.go to the handlers server.
	BanIPFunc func(ip, reason string) error

	reloadCh chan struct{}
	stopCh   chan struct{}

	lastExpCheck time.Time
}

// NewService creates the bot service.
func NewService(database *db.DB, encKey []byte, dataDir string) *Service {
	svc := &Service{
		db:       database,
		encKey:   encKey,
		dataDir:  dataDir,
		pending:  make(map[int64]*pending),
		qpFails:  make(map[int64]*qpFail),

		reloadCh: make(chan struct{}, 1),
		stopCh:   make(chan struct{}),
	}
	go svc.idleLoop()
	go svc.remindLoop()
	go svc.expiryLoop()
	return svc
}

// remindLoop 每分钟检查到期的提醒并发送
func (s *Service) remindLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.fireDueReminders()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Service) fireDueReminders() {
	now := time.Now().Unix()
	due, err := s.db.DueReminders(now)
	if err != nil || len(due) == 0 {
		return
	}
	// 提醒发给绑定的 chat
	chatStr := s.db.GetSetting("admin_chat_id")
	chatID, err := strconv.ParseInt(strings.TrimSpace(chatStr), 10, 64)
	if err != nil || chatID == 0 {
		return
	}
	s.mu.Lock()
	client := s.tgClient
	s.mu.Unlock()
	if client == nil {
		return
	}
	log.Printf("tg: firing %d due reminders", len(due))
	for _, r := range due {
		// 连发 N 条
		times := r.Times
		if times < 1 {
			times = 1
		}
		if times > 10 {
			times = 10
		}
		msg := "⏰ 提醒：" + r.Title
		if r.Note != "" {
			msg += "\n" + r.Note
		}
		for i := 0; i < times; i++ {
			id, _ := client.SendMessage(chatID, msg, "", nil)
			s.trackBot(chatID, id)
			if i < times-1 {
				time.Sleep(500 * time.Millisecond)
			}
		}
		s.db.Audit("tg_reminder_fired", fmt.Sprintf("id=%d title=%s times=%d", r.ID, r.Title, times), "telegram")
		// 更新下一次
		if r.RepeatType == "none" || r.RepeatType == "" {
			s.db.SetReminderActive(r.ID, false)
		} else {
			next := nextRemindTime(r.RemindAt, r.RepeatType, r.RepeatDetail)
			// 如果算出的下一次仍已过（比如关机很久），继续往后推
			for next <= now {
				next = nextRemindTime(next, r.RepeatType, r.RepeatDetail)
				if next == 0 {
					break
				}
			}
			if next == 0 {
				s.db.SetReminderActive(r.ID, false)
			} else {
				r.RemindAt = next
				s.db.UpdateReminder(r)
			}
		}
	}
}

// idleLoop silently wipes chats that have been silent for 10 minutes.
func (s *Service) idleLoop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			s.idleSweep()
		}
	}
}

// touchActive records admin activity for the idle-wipe clock. Every update
// from the admin chat counts — text messages and button taps alike. (Before
// this, only photos and callbacks refreshed the clock, so an active text
// conversation looked "idle" and the sweep ate live drafts and replies.)
func (s *Service) touchActive(chatID int64) {
	_ = s.db.SetSetting(fmt.Sprintf("tg_last_active_%d", chatID), strconv.FormatInt(time.Now().Unix(), 10))
}

// idleTimeout returns the configured idle-wipe timeout (default 5 minutes).
func (s *Service) idleTimeout() time.Duration {
	if v, err := strconv.Atoi(strings.TrimSpace(s.db.GetSetting("tg_idle_wipe_minutes"))); err == nil && v > 0 {
		return time.Duration(v) * time.Minute
	}
	return 5 * time.Minute
}

func (s *Service) idleSweep() {
	token, _, ok := s.loadConfig()
	if !ok {
		return
	}
	timeout := s.idleTimeout()
	now := time.Now().Unix()
	client := NewClient(token)
	for _, chatID := range s.db.ListTgChats() {
		lastKey := fmt.Sprintf("tg_last_active_%d", chatID)
		last, _ := strconv.ParseInt(s.db.GetSetting(lastKey), 10, 64)
		if last == 0 {
			// No activity record. Start the clock instead of wiping:
			// wipe resets this to 0, so wiping on 0 loops forever and
			// eats fresh messages on every tick.
			_ = s.db.SetSetting(lastKey, strconv.FormatInt(now, 10))
			continue
		}
		if now-last > int64(timeout.Seconds()) {
			n := len(s.db.ListTgMessages(chatID))
			if n == 0 {
				continue
			}
			s.wipe(client, chatID)
			_ = s.db.SetSetting(lastKey, "0")
			log.Printf("tg: idle wipe chat %d (%d messages, idle %ds)", chatID, n, now-last)
		}
	}
}

// Reload signals the loop to re-read bot configuration.
func (s *Service) Reload() {
	select {
	case s.reloadCh <- struct{}{}:
	default:
	}
}

// Stop terminates the loop.
func (s *Service) Stop() { close(s.stopCh) }

func (s *Service) loadConfig() (token string, chatID int64, ok bool) {
	enc := s.db.GetSetting("bot_token_enc")
	if enc == "" {
		return "", 0, false
	}
	raw, err := crypto.Decrypt(s.encKey, enc)
	if err != nil {
		return "", 0, false
	}
	token = strings.TrimSpace(string(raw))
	cid, _ := strconv.ParseInt(strings.TrimSpace(s.db.GetSetting("admin_chat_id")), 10, 64)
	if token == "" || cid == 0 {
		return "", 0, false
	}
	return token, cid, true
}

// Run starts the long-poll loop. It returns when Stop is called.
func (s *Service) Run() {
	var offset int64
	var client *Client
	var lastToken string
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}
		select {
		case <-s.reloadCh:
		default:
		}
		token, chatID, ok := s.loadConfig()
		if !ok {
			client, lastToken = nil, ""
			if s.sleepInterrupt(5 * time.Second) {
				return
			}
			continue
		}
		if client == nil || token != lastToken {
			c := NewClient(token)
			if user, err := c.GetMe(); err != nil {
				log.Printf("tg: bad bot token: %v", err)
				if s.sleepInterrupt(30 * time.Second) {
					return
				}
				continue
			} else {
				log.Printf("tg: bot @%s connected", user)
			}
			// install bot command menu (best effort)
			_ = c.SetMyCommands([]map[string]string{
				{"command": "start", "description": "使用说明"},
				{"command": "add", "description": "手动添加：选分类后发送内容入库"},
				{"command": "search", "description": "分类查找：选分类后关键词搜索"},
				{"command": "cancel", "description": "取消当前操作"},
			})
			client, lastToken = c, token
		s.mu.Lock()
		s.tgClient = c
		s.mu.Unlock()
		}
		updates, err := client.GetUpdates(offset, 30)
		if err != nil {
			if s.sleepInterrupt(5 * time.Second) {
				return
			}
			continue
		}
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			s.handleUpdate(client, chatID, u)
		}
		s.gc()
		s.maybeNotifyExpiring(client, chatID)
	}
}

// maybeNotifyExpiring sends a daily digest of soon-to-expire secrets to the
// admin chat (at most once every 20 hours).
func (s *Service) maybeNotifyExpiring(c *Client, chatID int64) {
	if time.Since(s.lastExpCheck) < 20*time.Hour {
		return
	}
	s.lastExpCheck = time.Now()
	list, err := s.db.ExpiringSecrets(7)
	if err != nil || len(list) == 0 {
		return
	}
	names := map[int64]string{}
	var b strings.Builder
	b.WriteString("【到期提醒】以下机密 7 天内到期：\n")
	for _, sec := range list {
		name, ok := names[sec.MerchantID]
		if !ok {
			if m, err := s.db.GetMerchant(sec.MerchantID); err == nil {
				name = m.Name
			}
			names[sec.MerchantID] = name
		}
		label := sec.Label
		if label == "" {
			label = "（未命名）"
		}
		d := int((sec.ExpiresAt - time.Now().Unix()) / 86400)
		when := fmt.Sprintf("还有%d天", d)
		if d < 0 {
			when = "已过期"
		} else if d == 0 {
			when = "今天到期"
		}
		fmt.Fprintf(&b, "• %s · %s（%s）\n", mdEsc(name), mdEsc(label), when)
	}
	mid, _ := c.SendMessage(chatID, b.String(), "", nil)
	s.trackBot(chatID, mid)
}

func (s *Service) sleepInterrupt(d time.Duration) bool {
	select {
	case <-s.stopCh:
		return true
	case <-s.reloadCh:
		return false
	case <-time.After(d):
		return false
	}
}

func (s *Service) gc() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for chat, p := range s.pending {
		if now.After(p.expires) {
			delete(s.pending, chat)
		}
	}
	for chat, f := range s.qpFails {
		if now.Sub(f.first) > 10*time.Minute {
			delete(s.qpFails, chat)
		}
	}
}

// ---- message tracking / cleanup ----

func (s *Service) trackUser(chatID, msgID int64) {
	if msgID != 0 {
		s.db.TrackTgMessage(chatID, msgID)
	}
}

func (s *Service) trackBot(chatID, msgID int64) {
	if msgID != 0 {
		s.db.TrackTgMessage(chatID, msgID)
	}
}

// wipe deletes every tracked message for the chat and clears its state.
// wipe deletes every tracked message for the chat and clears its state.
func (s *Service) wipe(client *Client, chatID int64) {
	s.mu.Lock()
	delete(s.pending, chatID)
	s.mu.Unlock()
	for _, id := range s.db.ListTgMessages(chatID) {
		if err := client.DeleteMessage(chatID, id); err == nil {
			s.db.UntrackTgMessage(chatID, id)
		}
	}
}

// wipeMsgs 只删除聊天消息，保留 pending（用于向导中途切换步骤）
func (s *Service) wipeMsgs(client *Client, chatID int64) {
	for _, id := range s.db.ListTgMessages(chatID) {
		if err := client.DeleteMessage(chatID, id); err == nil {
			s.db.UntrackTgMessage(chatID, id)
		}
	}
}

func (s *Service) ensurePending(chatID int64) *pending {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.pending[chatID]
	if p == nil {
		p = &pending{}
		s.pending[chatID] = p
	}
	p.expires = time.Now().Add(10 * time.Minute)
	return p
}

func (s *Service) clearPending(chatID int64) {
	s.mu.Lock()
	delete(s.pending, chatID)
	s.mu.Unlock()
}

// ---- update handling ----

func (s *Service) handleUpdate(client *Client, adminChat int64, u Update) {
	if cb := u.CallbackQuery; cb != nil {
		if cb.Message == nil || cb.Message.Chat.ID != adminChat {
			return
		}
		s.touchActive(adminChat)
		s.handleCallback(client, adminChat, cb)
		return
	}
	m := u.Message
	if m == nil || m.Chat.ID != adminChat {
		return
	}
	s.touchActive(adminChat)
	if len(m.Photo) > 0 {
		s.handlePhoto(client, adminChat, m)
		return
	}
	if m.Document != nil {
		s.handleDocument(client, adminChat, m)
		return
	}
	if strings.TrimSpace(m.Text) == "" {
		return
	}
	s.handleMessage(client, adminChat, m)
}

// sniffText 判断内容是否为文本（无 NUL 字节且 UTF-8 有效）
func sniffText(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	chunk := data[:n]
	for _, b := range chunk {
		if b == 0 {
			return false
		}
	}
	// 简单 UTF-8 有效性检查
	i := 0
	for i < n {
		c := chunk[i]
		if c < 0x80 {
			i++
			continue
		}
		var need int
		switch {
		case c>>5 == 0x6:
			need = 1
		case c>>4 == 0xE:
			need = 2
		case c>>3 == 0x1E:
			need = 3
		default:
			return false
		}
		if i+need >= n {
			break
		}
		for j := 1; j <= need; j++ {
			if chunk[i+j]>>6 != 0x2 {
				return false
			}
		}
		i += need + 1
	}
	return true
}

// handleDocument saves a sent file (non-image) into the files section.
// caption = category name, empty -> 未分类.
func (s *Service) handleDocument(client *Client, chatID int64, m *Message) {
	doc := m.Document
	s.trackUser(chatID, m.MessageID)
	// 手动添加等待文件：强制用选定的分类
	s.mu.Lock()
	mp := s.pending[chatID]
	s.mu.Unlock()
	var forceCatID int64
	var forceCatName string
	if mp != nil && mp.kind == pManualWait && mp.manualSection == "file" {
		forceCatID, forceCatName = mp.manualID, mp.manualName
	}
	dir := filepath.Join(s.dataDir, "files")
	if err := os.MkdirAll(dir, 0755); err != nil {
		id, _ := client.SendMessage(chatID, "保存文件失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	fp, err := client.GetFile(doc.FileID)
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存文件失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	data, err := client.DownloadFile(fp)
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存文件失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	ext := strings.ToLower(filepath.Ext(doc.FileName))
	base := fmt.Sprintf("%d", time.Now().UnixNano())
	name := base + ext
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		id, _ := client.SendMessage(chatID, "保存文件失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	origName := doc.FileName
	if origName == "" {
		origName = name
	}
	isText := sniffText(data)
	if forceCatID != 0 {
		if _, err := s.db.CreateFile(forceCatID, name, origName, name, doc.MimeType, isText, int64(len(data))); err != nil {
			os.Remove(filepath.Join(dir, name))
			id, _ := client.SendMessage(chatID, "保存文件失败："+err.Error(), "", nil)
			s.trackBot(chatID, id)
			return
		}
		s.wipe(client, chatID)
		id, _ := client.SendMessage(chatID, fmt.Sprintf("✅ 文件已入库（%s）：%s", forceCatName, origName), "", nil)
		s.trackBot(chatID, id)
		return
	}
	uncatID, _ := s.db.FindOrCreateFileCategory("")
	fid, err := s.db.CreateFile(uncatID, name, origName, name, doc.MimeType, isText, int64(len(data)))
	if err != nil {
		os.Remove(filepath.Join(dir, name))
		id, _ := client.SendMessage(chatID, "保存文件失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	s.wipeMsgs(client, chatID)
	s.askCategory(client, chatID, "file", []int64{fid}, "文件「"+origName+"」")
}

// handlePhoto saves a sent photo into the images section.
// Caption (if any) is the category name; otherwise 未分类.
func (s *Service) handlePhoto(client *Client, chatID int64, m *Message) {
	s.trackUser(chatID, m.MessageID)
	_ = s.db.SetSetting(fmt.Sprintf("tg_last_active_%d", chatID), strconv.FormatInt(time.Now().Unix(), 10))
	// 手动添加等待图片：强制用选定的分类
	s.mu.Lock()
	mp := s.pending[chatID]
	s.mu.Unlock()
	var forceCatID int64
	var forceCatName string
	if mp != nil && mp.kind == pManualWait && mp.manualSection == "image" {
		forceCatID = mp.manualID
		forceCatName = mp.manualName
	}
	// pick smallest and largest sizes
	small, large := m.Photo[0], m.Photo[0]
	for _, ps := range m.Photo[1:] {
		if ps.FileSize < small.FileSize {
			small = ps
		}
		if ps.FileSize > large.FileSize {
			large = ps
		}
	}
	dir := filepath.Join(s.dataDir, "images")
	if err := os.MkdirAll(dir, 0755); err != nil {
		id, _ := client.SendMessage(chatID, "保存图片失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	// download largest, save original + generate s/m/l thumbnails
	fp, err := client.GetFile(large.FileID)
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存图片失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	data, err := client.DownloadFile(fp)
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存图片失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	ext := strings.ToLower(filepath.Ext(fp))
	if ext == "" {
		ext = ".jpg"
	}
	base := fmt.Sprintf("%d", time.Now().UnixNano())
	fullName := base + "_orig" + ext
	if err := os.WriteFile(filepath.Join(dir, fullName), data, 0644); err != nil {
		id, _ := client.SendMessage(chatID, "保存图片失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	thumbS, thumbM, thumbL, err := img.MakeThumbs(data, dir, base)
	if err != nil {
		os.Remove(filepath.Join(dir, fullName))
		id, _ := client.SendMessage(chatID, "保存图片失败：图片格式不支持", "", nil)
		s.trackBot(chatID, id)
		return
	}
	_ = small
	caption := strings.TrimSpace(m.Caption)
	origName := caption
	if origName == "" {
		// 无中文：日期+随机4位+后缀
		randStr := fmt.Sprintf("%04x", time.Now().UnixNano()%65536)
		origName = time.Now().Format("20060102") + "_" + randStr + ext
	}
	if forceCatID != 0 {
		// 手动添加：已选定分类，直接存
		if _, err := s.db.CreateImage(forceCatID, caption, origName, fullName, thumbS, thumbM, thumbL, large.FileSize); err != nil {
			id, _ := client.SendMessage(chatID, "保存图片失败："+err.Error(), "", nil)
			s.trackBot(chatID, id)
			return
		}
		s.db.Audit("tg_image_add", "category="+forceCatName, "telegram")
		s.wipe(client, chatID)
		rid, _ := client.SendMessage(chatID, "✅ 图片已存入「"+forceCatName+"」", "", nil)
		s.trackBot(chatID, rid)
		return
	}
	// 普通流程：先存未分类，再选分类
	uncatID, _ := s.db.FindOrCreateImageCategory("")
	imgID, err := s.db.CreateImage(uncatID, caption, origName, fullName, thumbS, thumbM, thumbL, large.FileSize)
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存图片失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	s.db.Audit("tg_image_add", "category=未分类", "telegram")
	s.wipeMsgs(client, chatID)
	s.askCategory(client, chatID, "image", []int64{imgID}, "图片")
}

func (s *Service) handleMessage(client *Client, chatID int64, m *Message) {
	text := strings.TrimSpace(m.Text)
	s.trackUser(chatID, m.MessageID)
	_ = s.db.SetSetting(fmt.Sprintf("tg_last_active_%d", chatID), strconv.FormatInt(time.Now().Unix(), 10))
	// 命令在任何状态下都生效
	switch text {
	case "/cancel":
		s.wipe(client, chatID)
		return
	case "/start":
		s.wipe(client, chatID)
		s.sendStart(client, chatID)
		return
	case "/add":
		s.wipe(client, chatID)
		s.startManualAdd(client, chatID)
		return
	case "/search":
		s.wipe(client, chatID)
		s.startSearch(client, chatID)
		return
	}
	// any fresh message replaces a stale pending state (except password entry)
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if p != nil && p.kind == pAddNeedMerchant {
		s.handleMerchantReply(client, chatID, p, text)
		return
	}
	if p != nil && p.kind == pScriptNeedName {
		s.handleScriptNameReply(client, chatID, p, text)
		return
	}
	if p != nil && p.kind == pSSHNeedName {
		s.handleSSHNameReply(client, chatID, p, text)
		return
	}
	if p != nil && p.kind == pQpwd {
		if banned, left := s.banStatus(); banned {
			mid, _ := client.SendMessage(chatID, "查询功能已被封禁（剩余"+left+"），可在网站后台解除。", "", nil)
			s.trackBot(chatID, mid)
			return
		}
		s.handleQueryPassword(client, chatID, m)
		return
	}
	if p != nil && p.kind == pManualWait {
		s.handleManualContent(client, chatID, m, p)
		return
	}
	if p != nil && p.kind == pSearchKeyword {
		s.handleSearchKeyword(client, chatID, m, p)
		return
	}
	if p != nil && p.kind == pSearchQpwd {
		s.handleSearchQpwd(client, chatID, m, p)
		return
	}
	if p != nil && p.kind == pRemindTime {
		s.handleRemindText(client, chatID, m, p)
		return
	}
	if p != nil && p.kind == pNewCategory {
		s.handleNewCategoryInput(client, chatID, p, text)
		return
	}
	if p != nil && p.kind == pNeedCategory {
		// 等待点按钮时发了文字：忽略（或提示）
		id, _ := client.SendMessage(chatID, "请点击上方按钮选择分类，或点「✏️ 新分类」输入名称。", "", nil)
		s.trackBot(chatID, id)
		return
	}
	if p != nil {
		s.wipe(client, chatID) // abort previous flow
	}
	p = s.ensurePending(chatID)
	s.trackUser(chatID, m.MessageID)

	switch text {
	case "/start":
		s.sendStart(client, chatID)
		return
	}

	if !strings.Contains(text, "\n") {
		// 单行"提醒"：提示完整格式
		if text == "提醒" {
			id, _ := client.SendMessage(chatID, "请按格式发送：\n提醒\n标题\n备注（可选）", "", nil)
			s.trackBot(chatID, id)
			return
		}
		// 单个链接：节点协议弹出选择（订阅节点/机密库），否则进网址/未分类
		if strings.Contains(text, "://") {
			if isNodeURL(text) && !isHttpURL(text) {
				s.askNodeDest(client, chatID, text)
				return
			}
			s.startAdd(client, chatID, text)
			return
		}
		if banned, left := s.banStatus(); banned {
			mid, _ := client.SendMessage(chatID, "查询功能已被封禁（剩余"+left+"），可在网站后台解除。", "", nil)
			s.trackBot(chatID, mid)
			return
		}
		s.startQuery(client, chatID, text)
		return
	}
	s.startAdd(client, chatID, text)
}

// ---- add flow ----

func (s *Service) startAdd(client *Client, chatID int64, text string) {
	// script protocol: first line "脚本", then the script body
	firstLine := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	// 机密关键字：去掉第一行，剩余按机密格式解析
	if firstLine == "机密" || firstLine == "机密库" {
		rest := strings.TrimSpace(strings.TrimPrefix(text, firstLine))
		rest = strings.TrimLeft(rest, "\n")
		if rest == "" {
			id, _ := client.SendMessage(chatID, "请按格式发送：\n机密\n服务商名\n标签值", "", nil)
			s.trackBot(chatID, id)
			return
		}
		s.startAdd(client, chatID, rest)
		return
	}
	if firstLine == "脚本" {
		s.startScript(client, chatID, text)
		return
	}
	// links protocol: first line "网址", then [category lines and] name+URL lines
	if firstLine == "网址" {
		s.startLinks(client, chatID, text)
		return
	}
	// nodes protocol: "订阅" / "节点" / "订阅节点" / [name] / urls...
	if firstLine == "订阅" || firstLine == "节点" || firstLine == "订阅节点" {
		s.startNodes(client, chatID, text)
		return
	}
	// notes protocol: "笔记" / title / content...
	if firstLine == "笔记" {
		s.startNote(client, chatID, text)
		return
	}
	// reminder protocol: "提醒" / title / note...
	if firstLine == "提醒" {
		s.startReminder(client, chatID, text)
		return
	}
	// SSH protocol: "SSH" / host:port / username / password
	if firstLine == "SSH" {
		s.startSSH(client, chatID, text)
		return
	}
	// bare userscript paste (no marker): still auto-detected
	if meta, ok := ParseUserScript(text); ok {
		s.saveScriptToUncat(client, chatID, meta.Name, meta.Version, meta.Description, meta.Author, text)
		return
	}
	// 多行“名字 网址”自动识别为网址（不用必须带“网址”前缀，也不用必须带 http）
	if s.looksLikeLinks(text) {
		s.startLinks(client, chatID, "网址\n"+text)
		return
	}
	// 任何一行是节点链接（非 http）：弹出选择（订阅节点/机密库）
	// 覆盖"名称+节点链接"两行的情况
	hasNodeURL := false
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" && isNodeURL(ln) && !isHttpURL(ln) {
			hasNodeURL = true
			break
		}
	}
	if hasNodeURL {
		s.askNodeDest(client, chatID, text)
		return
	}
	// first line has a URL: either links or merchant-less secrets
	if strings.Contains(firstLine, "://") {
		allURL := true
		for _, ln := range strings.Split(text, "\n") {
			ln = strings.TrimSpace(ln)
			if ln != "" && !strings.Contains(ln, "://") {
				allURL = false
				break
			}
		}
		if allURL {
			// 全是节点协议链接 -> 进订阅节点
			allNode := true
			for _, ln := range strings.Split(text, "\n") {
				ln = strings.TrimSpace(ln)
				if ln != "" && (!isNodeURL(ln) || isHttpURL(ln)) {
					allNode = false
					break
				}
			}
			if allNode {
				s.askNodeDest(client, chatID, text)
				return
			}
			// 单网址直接进网址/未分类，自动抓取标题做名称
			catID, err := s.db.FindOrCreateLinkCategory("未分类")
			if err != nil {
				id, _ := client.SendMessage(chatID, "保存失败："+err.Error(), "", nil)
				s.trackBot(chatID, id)
				return
			}
			urls := []string{}
			for _, ln := range strings.Split(text, "\n") {
				ln = strings.TrimSpace(ln)
				if ln != "" {
					urls = append(urls, ln)
				}
			}
			// 并发抓标题
			titles := make([]string, len(urls))
			var wg sync.WaitGroup
			for i, u := range urls {
				wg.Add(1)
				go func(i int, u string) {
					defer wg.Done()
					titles[i] = fetchTitle(u)
				}(i, u)
			}
			wg.Wait()
			n := 0
			for i, u := range urls {
				name := titles[i]
				auto := 0
				if name == "" {
					name = u
				} else {
					auto = 1
				}
				if _, err := s.db.CreateLink(catID, name, u, auto); err == nil {
					n++
				}
			}
			s.wipe(client, chatID) // 清掉原文和过程消息
			id, _ := client.SendMessage(chatID, fmt.Sprintf("✅ 已入库 %d 条网址（未分类）", n), "", nil)
			s.trackBot(chatID, id)
			return
		}
		p := s.ensurePending(chatID)
		p.kind = pAddNeedMerchant
		p.addText = text
		id, _ := client.SendMessage(chatID, "没看到服务商名称，请先发一个服务商名：", "", nil)
		s.trackBot(chatID, id)
		return
	}
	labels, _ := s.db.CustomLabels()
	parsed := parse.ParseMessage(text, labels)
	if strings.TrimSpace(parsed.Merchant) == "" || len(parsed.Entries) == 0 {
		id, _ := client.SendMessage(chatID, "没解析出有效内容。第一行写服务商名，后面每行一条（标签+值）。", "", nil)
		s.trackBot(chatID, id)
		return
	}
	merchants, _ := s.db.ListMerchants()
	var infos []parse.MerchantInfo
	for _, m := range merchants {
		infos = append(infos, parse.MerchantInfo{ID: m.ID, Name: m.Name, Aliases: m.Aliases})
	}
	exact, cands := parse.MatchMerchant(parsed.Merchant, infos)
	p := s.ensurePending(chatID)
	p.kind = pAddChoice
	p.merchantName = strings.TrimSpace(parsed.Merchant)
	p.entries = parsed.Entries
	if exact != nil {
		p.kind = pAddConfirm
		p.merchantID = exact.ID
		s.sendAddConfirm(client, chatID, exact.Name, false)
		return
	}
	var dbCands []db.Merchant
	for _, c := range cands {
		if m, err := s.db.GetMerchant(c.ID); err == nil {
			dbCands = append(dbCands, m)
		}
	}
	p.cands = dbCands
	// ask: choose existing / create new
	var rows [][]InlineKeyboardButton
	for _, c := range dbCands {
		name := c.Name
		rows = append(rows, BtnRow(Btn("使用："+name, "m:use:"+strconv.FormatInt(c.ID, 10))))
	}
	rows = append(rows, BtnRow(Btn("新建服务商："+p.merchantName, "m:new")))
	rows = append(rows, BtnRow(Btn("取消", "m:cancel")))
	msg := fmt.Sprintf("服务商“%s”没有完全匹配。\n请选择：", p.merchantName)
	id, _ := client.SendMessage(chatID, msg, "", &InlineKeyboardMarkup{InlineKeyboard: rows})
	s.trackBot(chatID, id)
}

func previewValue(v string) string {
	r := []rune(v)
	if len(r) <= 14 {
		return v
	}
	return string(r[:6]) + "…" + string(r[len(r)-4:])
}

func (s *Service) sendAddConfirm(client *Client, chatID int64, merchantName string, isNew bool) {
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if p == nil {
		return
	}
	title := "服务商：" + merchantName + "（已有）"
	if isNew || p.merchantID == 0 {
		title = "新建服务商：" + p.merchantName
	}
	var b strings.Builder
	b.WriteString(title + "\n")
	fmt.Fprintf(&b, "将添加 %d 条：\n", len(p.entries))
	for _, e := range p.entries {
		label := e.Label
		if label == "" {
			label = "（未识别标签）"
		}
		b.WriteString("• " + label + "：" + previewValue(e.Value))
		if e.ExpiresDays > 0 {
			fmt.Fprintf(&b, "（%d天后到期）", e.ExpiresDays)
		}
		b.WriteString("\n")
	}
	kb := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		BtnRow(Btn("确认入库", "add:ok"), Btn("取消", "add:no")),
	}}
	id, _ := client.SendMessage(chatID, b.String(), "", kb)
	s.trackBot(chatID, id)
}

func (s *Service) doAddConfirm(client *Client, chatID int64) {
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if p == nil || p.kind != pAddConfirm {
		mid, _ := client.SendMessage(chatID, "⚠️ 这一步已过期（草稿已失效），请重新发送内容", "", nil)
		s.trackBot(chatID, mid)
		return
	}
	merchantID := p.merchantID
	if merchantID == 0 {
		id, err := s.db.CreateMerchant(p.merchantName, nil, "", "")
		if err != nil {
			mid, _ := client.SendMessage(chatID, "创建服务商失败："+err.Error(), "", nil)
			s.trackBot(chatID, mid)
			return
		}
		merchantID = id
		// auto-fetch logo in background; never blocks the flow
		go autoLogo(s.db, merchantID, p.merchantName)
	}
	now := time.Now()
	for _, e := range p.entries {
		enc, err := crypto.Encrypt(s.encKey, []byte(e.Value))
		if err != nil {
			continue
		}
		var exp int64
		if e.ExpiresDays > 0 {
			exp = now.Add(time.Duration(e.ExpiresDays) * 24 * time.Hour).Unix()
		}
		_, _ = s.db.CreateSecret(db.Secret{
			MerchantID: merchantID,
			Kind:       "secret",
			Label:      e.Label,
			Value:      enc,
			ExpiresAt:  exp,
		})
		if e.Label != "" {
			_ = s.db.AddCustomLabel(e.Label) // learn user labels
		}
	}
	s.db.Audit("tg_add", fmt.Sprintf("merchant=%s entries=%d", p.merchantName, len(p.entries)), "telegram")
	mName := p.merchantName
	n := len(p.entries)
	s.wipe(client, chatID) // chat history cleaned; data lives on the server only
	// leave a receipt so the chat doesn't look "unfollowed"
	rid, _ := client.SendMessage(chatID, "✅ 已入库 "+strconv.Itoa(n)+" 条（"+mName+"）", "", nil)
	s.trackBot(chatID, rid)
}

// handleMerchantReply processes the merchant name sent for a script or a
// merchant-less add. The pending state is NOT wiped here.
func (s *Service) handleMerchantReply(client *Client, chatID int64, p *pending, text string) {
	name := strings.TrimSpace(text)
	if name == "" || strings.Contains(name, "\n") || strings.Contains(name, "://") {
		id, _ := client.SendMessage(chatID, "服务商名称不合法，请重新发送一个名称：", "", nil)
		s.trackBot(chatID, id)
		return
	}
	// re-run the add flow with the merchant name as first line
	s.mu.Lock()
	addText := p.addText
	s.mu.Unlock()
	s.mu.Lock()
	delete(s.pending, chatID)
	s.mu.Unlock()
	s.startAdd(client, chatID, name+"\n"+addText)
}

var bareDomainRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?\.[a-zA-Z]{2,}(/\S*)?$`)

// splitLinkLine splits "name URL" into name and url.
// Supports http(s):// URLs and bare domains (https:// is prepended).
func splitLinkLine(line string) (name, url string, ok bool) {
	idx := strings.Index(line, "https://")
	if idx < 0 {
		idx = strings.Index(line, "http://")
	}
	if idx < 0 {
		return "", "", false
	}
	url = strings.TrimSpace(line[idx:])
	name = strings.TrimSpace(line[:idx])
	name = strings.TrimRight(name, "：: ")
	name = strings.TrimSpace(name)
	if url == "" {
		return "", "", false
	}
	return name, url, true
}

// bare domain fallback: "name example.com/path" (no protocol)
func splitBareDomain(line string) (name, url string, ok bool) {
	toks := strings.Fields(line)
	if len(toks) == 0 {
		return "", "", false
	}
	last := toks[len(toks)-1]
	if !bareDomainRe.MatchString(last) {
		return "", "", false
	}
	name = strings.TrimSpace(strings.TrimSuffix(line, last))
	name = strings.TrimRight(name, "：: ")
	name = strings.TrimSpace(name)
	return name, "https://" + last, true
}

// startLinks handles the "网址" protocol:
// lines without a URL switch the target category (auto-created),
// lines with a URL are stored under the current category (default 未分类).
// looksLikeLinks checks if text looks like multiple "name URL" lines.
func (s *Service) looksLikeLinks(text string) bool {
	lines := strings.Split(text, "\n")
	count := 0
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if _, _, ok := splitLinkLine(ln); ok {
			count++
			continue
		}
		if _, _, ok := splitBareDomain(ln); ok {
			count++
			continue
		}
		return false
	}
	return count >= 2
}

func (s *Service) startLinks(client *Client, chatID int64, text string) {
	lines := strings.Split(text, "\n")
	type entry struct{ name, url string }
	var entries []entry
	for _, ln := range lines[1:] {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if name, url, ok := splitLinkLine(ln); ok {
			entries = append(entries, entry{name, url})
			continue
		}
		if name, url, ok := splitBareDomain(ln); ok {
			entries = append(entries, entry{name, url})
			continue
		}
	}
	if len(entries) == 0 {
		id, _ := client.SendMessage(chatID, "没解析出网址。每行一个：名字+网址。", "", nil)
		s.trackBot(chatID, id)
		return
	}
	uncatID, _ := s.db.FindOrCreateLinkCategory("")
	var ids []int64
	for _, e := range entries {
		if id, err := s.db.CreateLink(uncatID, e.name, e.url, 0); err == nil {
			ids = append(ids, id)
		}
	}
	s.db.Audit("tg_links_add", fmt.Sprintf("total=%d", len(ids)), "telegram")
	s.wipeMsgs(client, chatID)
	s.askCategory(client, chatID, "link", ids, fmt.Sprintf("%d 条网址", len(ids)))
}

// startNote handles the "笔记" protocol: line 1 = marker, line 2 = title,
// everything after is content, verbatim.
func (s *Service) startNote(client *Client, chatID int64, text string) {
	parts := strings.SplitN(text, "\n", 4)
	title := "无标题"
	if len(parts) >= 2 {
		if t := strings.TrimSpace(parts[1]); t != "" {
			title = t
		}
	}
	rest := ""
	if len(parts) >= 3 {
		rest = parts[2]
	}
	if len(parts) == 4 {
		rest = parts[2] + "\n" + parts[3]
	}
	content := strings.Trim(rest, "\n")
	enc, err := crypto.Encrypt(s.encKey, []byte(content))
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存笔记失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	uncatID, _ := s.db.FindOrCreateNoteCategory("")
	id, err := s.db.CreateNote(title, enc, uncatID)
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存笔记失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	s.db.Audit("tg_note_add", "title="+title, "telegram")
	s.wipeMsgs(client, chatID)
	s.askCategory(client, chatID, "note", []int64{id}, "笔记《"+title+"》")
}

// startScript handles the "脚本" protocol: first line is the marker,
// the rest is the script. Name is auto-detected when possible,
// otherwise the user is asked for one.
func (s *Service) startScript(client *Client, chatID int64, text string) {
	parts := strings.SplitN(text, "\n", 2)
	payload := ""
	if len(parts) == 2 {
		payload = strings.TrimSpace(parts[1])
	}
	if payload == "" {
		id, _ := client.SendMessage(chatID, "请在“脚本”后面粘贴脚本内容。", "", nil)
		s.trackBot(chatID, id)
		return
	}
	det := DetectScriptName(payload)
	if det.Name != "" {
		// re-parse metadata for version/desc/author when it's a userscript
		ver, desc, author := "", "", ""
		if meta, ok := ParseUserScript(payload); ok {
			ver, desc, author = meta.Version, meta.Description, meta.Author
		}
		s.saveScriptToUncat(client, chatID, det.Name, ver, desc, author, det.Content)
		return
	}
	p := s.ensurePending(chatID)
	p.kind = pScriptNeedName
	p.scriptContent = det.Content
	id, _ := client.SendMessage(chatID, "没识别出脚本名称，请发送一个名称：", "", nil)
	s.trackBot(chatID, id)
}

// handleScriptNameReply saves the pending script with the user-given name.
func (s *Service) handleScriptNameReply(client *Client, chatID int64, p *pending, text string) {
	name := strings.TrimSpace(text)
	if name == "" || strings.Contains(name, "\n") || len([]rune(name)) > 40 {
		id, _ := client.SendMessage(chatID, "名称不合法，请重新发送一个名称：", "", nil)
		s.trackBot(chatID, id)
		return
	}
	s.mu.Lock()
	content := p.scriptContent
	s.mu.Unlock()
	s.mu.Lock()
	delete(s.pending, chatID)
	s.mu.Unlock()
	s.saveScriptToUncat(client, chatID, name, "", "", "", content)
}

// saveScriptDirect encrypts and stores a pasted userscript (top-level scripts section).
func (s *Service) saveScriptDirect(client *Client, chatID int64, name, ver, desc, author, content string, catID int64) {
	enc, err := crypto.Encrypt(s.encKey, []byte(content))
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存脚本失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	if _, err := s.db.CreateScript(name, ver, desc, author, enc, catID); err != nil {
		id, _ := client.SendMessage(chatID, "保存脚本失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	s.db.Audit("tg_script_add", "script="+name, "telegram")
	s.wipe(client, chatID)
	vs := ""
	if ver != "" {
		vs = " v" + ver
	}
	rid, _ := client.SendMessage(chatID, "✅ 脚本《"+name+"》"+vs+"已存入脚本栏", "", nil)
	s.trackBot(chatID, rid)
}



// ---- query flow ----

func (s *Service) startQuery(client *Client, chatID int64, name string) {
	merchants, _ := s.db.ListMerchants()
	var infos []parse.MerchantInfo
	for _, m := range merchants {
		infos = append(infos, parse.MerchantInfo{ID: m.ID, Name: m.Name, Aliases: m.Aliases})
	}
	exact, cands := parse.MatchMerchant(name, infos)
	p := s.ensurePending(chatID)
	if exact != nil {
		p.kind = pQpwd
		p.merchantID = exact.ID
		p.merchantName = exact.Name
		id, _ := client.SendMessage(chatID, "查询“"+exact.Name+"”\n请输入查询密码：", "", nil)
		s.trackBot(chatID, id)
		return
	}
	p.kind = pQChoice
	var rows [][]InlineKeyboardButton
	for _, c := range cands {
		if m, err := s.db.GetMerchant(c.ID); err == nil {
			p.cands = append(p.cands, m)
			rows = append(rows, BtnRow(Btn(m.Name, "q:use:"+strconv.FormatInt(m.ID, 10))))
		}
	}
	rows = append(rows, BtnRow(Btn("取消", "q:cancel")))
	msg := "没有完全匹配“" + name + "”的服务商，请选择："
	if len(rows) == 1 {
		msg = "没有找到服务商“" + name + "”。"
	}
	id, _ := client.SendMessage(chatID, msg, "", &InlineKeyboardMarkup{InlineKeyboard: rows})
	s.trackBot(chatID, id)
}

func (s *Service) handleQueryPassword(client *Client, chatID int64, m *Message) {
	s.trackUser(chatID, m.MessageID)
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if p == nil || p.kind != pQpwd {
		return
	}
	// cooldown: 5 wrong in 10 min
	s.mu.Lock()
	f := s.qpFails[chatID]
	now := time.Now()
	if f != nil && now.Sub(f.first) < 10*time.Minute && f.count >= 5 {
		s.mu.Unlock()
		id, _ := client.SendMessage(chatID, "尝试过多，请10分钟后再试。", "", nil)
		s.trackBot(chatID, id)
		return
	}
	s.mu.Unlock()

	maxTries := 3
	if v := s.db.GetSetting("query_pwd_max_tries"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 10 {
			maxTries = n
		}
	}
	s.mu.Lock()
	tries := 0
	if p.qpTries > 0 {
		tries = p.qpTries
	}
	s.mu.Unlock()
	hash := s.db.GetSetting("query_pwd_hash")
	if hash == "" || !auth.VerifySecret(hash, auth.NormalizeSecret(m.Text)) {
		tries++
		s.mu.Lock()
		p.qpTries = tries
		if s.qpFails[chatID] == nil || now.Sub(s.qpFails[chatID].first) > 10*time.Minute {
			s.qpFails[chatID] = &qpFail{first: now}
		}
		s.qpFails[chatID].count++
		s.mu.Unlock()
		if tries >= maxTries {
			banInfo := s.applyBan()
			s.wipe(client, chatID)
			bmid, _ := client.SendMessage(chatID, "查询密码错误已达 "+strconv.Itoa(maxTries)+" 次，"+banInfo+"。", "", nil)
			s.trackBot(chatID, bmid)
			return
		}
		left := maxTries - tries
		id, _ := client.SendMessage(chatID, "查询密码错误，还剩 "+strconv.Itoa(left)+" 次机会。", "", nil)
		s.trackBot(chatID, id)
		return
	}
	s.mu.Lock()
	delete(s.qpFails, chatID)
	s.mu.Unlock()
	s.sendQueryResult(client, chatID, p.merchantID)
}

func (s *Service) sendQueryResult(client *Client, chatID, merchantID int64) {
	merchant, err := s.db.GetMerchant(merchantID)
	if err != nil {
		return
	}
	secrets, _ := s.db.ListSecrets(merchantID)
	// wipe query + password + prompt messages first: no trace
	s.wipe(client, chatID)

	minutes := 3
	if v, err := strconv.Atoi(strings.TrimSpace(s.db.GetSetting("auto_delete_minutes"))); err == nil && v > 0 {
		minutes = v
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s（共%d条）\n", merchant.Name, len(secrets))
	now := time.Now()
	for _, sec := range secrets {
		raw, err := crypto.Decrypt(s.encKey, sec.Value)
		if err != nil {
			continue
		}
		val := strings.ReplaceAll(string(raw), "`", "'")
		b.WriteString("\n")
		if sec.Kind == "account" {
			b.WriteString(mdEsc(sec.Label) + "\n")
			if sec.Username != "" {
				b.WriteString("账号：`" + mdCodeEsc(sec.Username) + "`\n")
			}
			b.WriteString("密码：`" + mdCodeEsc(val) + "`\n")
		} else {
			label := sec.Label
			if label == "" {
				label = "（未命名）"
			}
			b.WriteString(mdEsc(label) + "\n")
			b.WriteString("`" + mdCodeEsc(val) + "`\n")
		}
		if sec.Note != "" {
			b.WriteString(mdEsc(sec.Note) + "\n")
		}
		meta := "添加于 " + time.Unix(sec.CreatedAt, 0).Format("2006-01-02")
		if sec.ExpiresAt > 0 {
			d := int((sec.ExpiresAt - now.Unix()) / 86400)
			switch {
			case d < 0:
				meta += " · 已过期"
			case d == 0:
				meta += " · 今天到期"
			default:
				meta += fmt.Sprintf(" · 还有%d天到期", d)
			}
		}
		b.WriteString(meta + "\n")
	}
	text := b.String()
	// split long messages at 4000 chars on entry boundaries
	var chunks []string
	for len(text) > 4000 {
		cut := strings.LastIndex(text[:4000], "\n\n")
		if cut <= 0 {
			cut = 4000
		}
		chunks = append(chunks, text[:cut])
		text = text[cut:]
	}
	chunks = append(chunks, text)
	var sent []int64
	for _, ch := range chunks {
		if id, err := client.SendMessage(chatID, ch, "Markdown", nil); err == nil {
			sent = append(sent, id)
			s.trackBot(chatID, id)
		}
	}
	s.db.Audit("tg_query", "merchant="+merchant.Name, "telegram")
	// auto-delete after N minutes
	time.AfterFunc(time.Duration(minutes)*time.Minute, func() {
		for _, id := range sent {
			if err := client.DeleteMessage(chatID, id); err == nil {
				s.db.UntrackTgMessage(chatID, id)
			}
		}
	})
}

// ---- callbacks ----

func (s *Service) handleCallback(client *Client, chatID int64, cb *CallbackQuery) {
	data := cb.Data
	_ = s.db.SetSetting(fmt.Sprintf("tg_last_active_%d", chatID), strconv.FormatInt(time.Now().Unix(), 10))
	// One-click IP ban from security alerts: no pending dialog state needed.
	if strings.HasPrefix(data, "sec:ban:") {
		s.handleSecBanCallback(client, chatID, cb)
		return
	}
	client.AnswerCallbackQuery(cb.ID, "")
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if p == nil {
		// Add-flow buttons with no live draft used to die silently here.
		if data == "add:ok" || data == "m:new" || strings.HasPrefix(data, "m:use:") {
			mid, _ := client.SendMessage(chatID, "⚠️ 这一步已过期（草稿已失效），请重新发送内容", "", nil)
			s.trackBot(chatID, mid)
		}
		return
	}
	switch {
	case strings.HasPrefix(data, "m:use:"):
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "m:use:"), 10, 64)
		p.kind = pAddConfirm
		p.merchantID = id
		p.expires = time.Now().Add(10 * time.Minute)
		if m, err := s.db.GetMerchant(id); err == nil {
			s.sendAddConfirm(client, chatID, m.Name, false)
		}
	case data == "m:new":
		p.kind = pAddConfirm
		p.merchantID = 0
		p.expires = time.Now().Add(10 * time.Minute)
		s.sendAddConfirm(client, chatID, p.merchantName, true)
	case data == "m:cancel" || data == "add:no" || data == "q:cancel" || data == "node:cancel":
		s.wipe(client, chatID)
	case data == "node:to_nodes":
		s.mu.Lock()
		p := s.pending[chatID]
		s.mu.Unlock()
		if p == nil || p.kind != pNodeChoice {
			return
		}
		text := p.addText
		s.wipeMsgs(client, chatID)
		s.mu.Lock()
		delete(s.pending, chatID)
		s.mu.Unlock()
		s.startNodes(client, chatID, "节点\n"+text)
	case strings.HasPrefix(data, "cat:"):
		s.handleCatCallback(client, chatID, data)
	case data == "node:to_merchant":
		s.mu.Lock()
		p := s.pending[chatID]
		s.mu.Unlock()
		if p == nil || p.kind != pNodeChoice {
			return
		}
		text := p.addText
		s.wipeMsgs(client, chatID)
		s.mu.Lock()
		delete(s.pending, chatID)
		s.mu.Unlock()
		s.startAdd(client, chatID, text)
	case data == "add:ok":
		s.doAddConfirm(client, chatID)
	case strings.HasPrefix(data, "mas:"):
		s.handleManualSectionCallback(client, chatID, data)
	case strings.HasPrefix(data, "ma:"):
		s.handleManualCallback(client, chatID, data)
	case strings.HasPrefix(data, "scs:"):
		s.handleSearchSectionCallback(client, chatID, data)
	case strings.HasPrefix(data, "sc:"):
		s.handleSearchCallback(client, chatID, data)
	case strings.HasPrefix(data, "rm:"):
		s.handleRemindCallback(client, chatID, data)
	case strings.HasPrefix(data, "q:use:"):
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "q:use:"), 10, 64)
		if m, err := s.db.GetMerchant(id); err == nil {
			p.kind = pQpwd
			p.merchantID = id
			p.merchantName = m.Name
			mid, _ := client.SendMessage(chatID, "查询“"+m.Name+"”\n请输入查询密码：", "", nil)
			s.trackBot(chatID, mid)
		}
	}
}

// ---- markdown escaping (legacy Markdown) ----

var mdReplacer = strings.NewReplacer(
	"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]",
	"(", "\\(", ")", "\\)", "~", "\\~",
)

// mdEsc escapes text outside code spans.
func mdEsc(s string) string { return mdReplacer.Replace(s) }

// mdCodeEsc escapes backticks inside code spans.
func mdCodeEsc(s string) string { return strings.ReplaceAll(s, "`", "'") }

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// fetchTitle 抓取网页标题，失败返回 ""。标题按 rune 截断到 30 字。
func fetchTitle(rawURL string) string {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 300<<10))
	if err != nil {
		return ""
	}
	m := titleRe.FindSubmatch(body)
	if m == nil {
		return ""
	}
	t := strings.TrimSpace(html.UnescapeString(string(m[1])))
	t = strings.Map(func(r rune) rune {
		// 去掉不可见控制字符（LTR/RTL标记等）
		if r == 0x200e || r == 0x200f || r == 0xfeff {
			return -1
		}
		return r
	}, t)
	t = strings.Join(strings.Fields(t), " ") // 压缩空白
	r := []rune(t)
	if len(r) > 30 {
		t = string(r[:30]) + "…"
	}
	return t
}

// ---- manual add & category search ----

type catTarget struct {
	section string // "merchant" | "link" | "image" | "file"
	id      int64
	name    string
}

// 版块定义（一级菜单）
var sections = []struct {
	id    string
	name  string
	emoji string
}{
	{"merchant", "机密库", "🏦"},
	{"script", "脚本", "📜"},
	{"link", "网址", "🔗"},
	{"note", "笔记", "📝"},
	{"image", "图片", "🖼️"},
	{"file", "文件", "📄"},
	{"node", "订阅节点", "🌐"},
}

func sectionEmoji(section string) string {
	switch section {
	case "merchant":
		return "🏦"
	case "link":
		return "🔗"
	case "image":
		return "🖼️"
	case "file":
		return "📄"
	case "node":
		return "🌐"
	}
	return "📁"
}

// allTargets 返回所有可作为目标的分类（按版块分组）
func (s *Service) allTargets() []catTarget {
	var out []catTarget
	if ms, err := s.db.ListMerchants(); err == nil {
		for _, m := range ms {
			out = append(out, catTarget{"merchant", m.ID, m.Name})
		}
	}
	if cs, err := s.db.ListLinkCategories(); err == nil {
		for _, c := range cs {
			out = append(out, catTarget{"link", c.ID, c.Name})
		}
	}
	if cs, err := s.db.ListImageCategories(); err == nil {
		for _, c := range cs {
			out = append(out, catTarget{"image", c.ID, c.Name})
		}
	}
	if cs, err := s.db.ListFileCategories(); err == nil {
		for _, c := range cs {
			out = append(out, catTarget{"file", c.ID, c.Name})
		}
	}
	if cs, err := s.db.ListNodeCategories(); err == nil {
		for _, c := range cs {
			out = append(out, catTarget{"node", c.ID, c.Name})
		}
	}
	return out
}

// sendSectionPicker 发送一级版块选择器
func (s *Service) sendSectionPicker(client *Client, chatID int64, prefix, prompt string) {
	var rows [][]InlineKeyboardButton
	for _, sec := range sections {
		rows = append(rows, BtnRow(Btn(sec.emoji+" "+sec.name, prefix+":"+sec.id)))
	}
	rows = append(rows, BtnRow(Btn("❌ 取消", prefix+":cancel")))
	id, _ := client.SendMessage(chatID, prompt, "", &InlineKeyboardMarkup{InlineKeyboard: rows})
	s.trackBot(chatID, id)
}

// sendTargetPicker 发送指定版块的子分类选择器，prefix 决定回调前缀（ma=手动添加，sc=查找）
// includeAll 为 true 时首个选项为"全部"（仅搜索用）
func (s *Service) sendTargetPicker(client *Client, chatID int64, prefix, section, prompt string, includeAll bool) {
	var targets []catTarget
	switch section {
	case "merchant":
		if ms, err := s.db.ListMerchants(); err == nil {
			for _, m := range ms {
				targets = append(targets, catTarget{"merchant", m.ID, m.Name})
			}
		}
	case "script":
		if cs, err := s.db.ListScriptCategories(); err == nil {
			for _, c := range cs {
				targets = append(targets, catTarget{"script", c.ID, c.Name})
			}
		}
	case "note":
		if cs, err := s.db.ListNoteCategories(); err == nil {
			for _, c := range cs {
				targets = append(targets, catTarget{"note", c.ID, c.Name})
			}
		}
	case "link":
		if cs, err := s.db.ListLinkCategories(); err == nil {
			for _, c := range cs {
				targets = append(targets, catTarget{"link", c.ID, c.Name})
			}
		}
	case "image":
		if cs, err := s.db.ListImageCategories(); err == nil {
			for _, c := range cs {
				targets = append(targets, catTarget{"image", c.ID, c.Name})
			}
		}
	case "file":
		if cs, err := s.db.ListFileCategories(); err == nil {
			for _, c := range cs {
				targets = append(targets, catTarget{"file", c.ID, c.Name})
			}
		}
	}
	var rows [][]InlineKeyboardButton
	if includeAll {
		rows = append(rows, BtnRow(Btn("📂 全部", prefix+":"+section+":0")))
	}
	for _, t := range targets {
		label := t.name
		if len(label) > 28 {
			label = string([]rune(label)[:28]) + "…"
		}
		cb := prefix + ":" + t.section + ":" + strconv.FormatInt(t.id, 10)
		rows = append(rows, BtnRow(Btn(label, cb)))
		if len(rows) >= 40 {
			break
		}
	}
	rows = append(rows, BtnRow(Btn("❌ 取消", prefix+":cancel")))
	id, _ := client.SendMessage(chatID, prompt, "", &InlineKeyboardMarkup{InlineKeyboard: rows})
	s.trackBot(chatID, id)
}

func (s *Service) startManualAdd(client *Client, chatID int64) {
	p := s.ensurePending(chatID)
	p.kind = pManualSection
	s.sendSectionPicker(client, chatID, "mas", "请选择要添加到的版块：")
}

func (s *Service) startSearch(client *Client, chatID int64) {
	p := s.ensurePending(chatID)
	p.kind = pSearchSection
	s.sendSectionPicker(client, chatID, "scs", "请选择要搜索的版块：")
}

// handleManualContent 处理手动添加等待中的用户内容
func (s *Service) handleManualContent(client *Client, chatID int64, m *Message, p *pending) {
	text := strings.TrimSpace(m.Text)
	s.trackUser(chatID, m.MessageID)
	section, targetID, targetName := p.manualSection, p.manualID, p.manualName

	cancelBtn := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		BtnRow(Btn("❌ 取消", "ma:cancel")),
	}}

	switch section {
	case "merchant":
		// 文本按机密解析入库
		if text == "" {
			id, _ := client.SendMessage(chatID, "请发送文本内容（每行一条：标签+值），或取消。", "", cancelBtn)
			s.trackBot(chatID, id)
			return
		}
		parsed := parse.ParseMessage(targetName+"\n"+text, nil)
		if len(parsed.Entries) == 0 {
			id, _ := client.SendMessage(chatID, "没解析到有效内容，请重新发送，或取消。", "", cancelBtn)
			s.trackBot(chatID, id)
			return
		}
		now := time.Now()
		n := 0
		for _, e := range parsed.Entries {
			enc, err := crypto.Encrypt(s.encKey, []byte(e.Value))
			if err != nil {
				continue
			}
			var exp int64
			if e.ExpiresDays > 0 {
				exp = now.Add(time.Duration(e.ExpiresDays) * 24 * time.Hour).Unix()
			}
			if _, err := s.db.CreateSecret(db.Secret{
				MerchantID: targetID,
				Kind:       "secret",
				Label:      e.Label,
				Value:      enc,
				ExpiresAt:  exp,
			}); err == nil {
				n++
				if e.Label != "" {
					_ = s.db.AddCustomLabel(e.Label)
				}
			}
		}
		s.db.Audit("tg_manual_add", fmt.Sprintf("merchant=%s entries=%d", targetName, n), "telegram")
		s.wipe(client, chatID)
		id, _ := client.SendMessage(chatID, fmt.Sprintf("✅ 已入库 %d 条（%s）", n, targetName), "", nil)
		s.trackBot(chatID, id)
	case "script":
		if text == "" {
			id, _ := client.SendMessage(chatID, "请粘贴脚本内容，或取消。", "", cancelBtn)
			s.trackBot(chatID, id)
			return
		}
		det := DetectScriptName(text)
		name := det.Name
		if name == "" {
			name = "未命名脚本"
		}
		ver, desc, author := "", "", ""
		if meta, ok := ParseUserScript(det.Content); ok {
			ver, desc, author = meta.Version, meta.Description, meta.Author
		}
		s.saveScriptDirect(client, chatID, name, ver, desc, author, det.Content, targetID)
	case "link":
		if text == "" {
			id, _ := client.SendMessage(chatID, "请发送网址（每行一个，可带名称），或取消。", "", cancelBtn)
			s.trackBot(chatID, id)
			return
		}
		n := s.saveLinksToCategory(client, chatID, targetID, text)
		s.wipe(client, chatID)
		id, _ := client.SendMessage(chatID, fmt.Sprintf("✅ 已入库 %d 条网址（%s）", n, targetName), "", nil)
		s.trackBot(chatID, id)
	case "note":
		if text == "" {
			id, _ := client.SendMessage(chatID, "请发送笔记（第一行是标题，后面是正文），或取消。", "", cancelBtn)
			s.trackBot(chatID, id)
			return
		}
		parts := strings.SplitN(text, "\n", 2)
		title := strings.TrimSpace(parts[0])
		if title == "" {
			title = "无标题"
		}
		content := ""
		if len(parts) == 2 {
			content = strings.Trim(parts[1], "\n")
		}
		enc, err := crypto.Encrypt(s.encKey, []byte(content))
		if err != nil {
			id, _ := client.SendMessage(chatID, "保存失败："+err.Error(), "", nil)
			s.trackBot(chatID, id)
			return
		}
		if _, err := s.db.CreateNote(title, enc, targetID); err != nil {
			id, _ := client.SendMessage(chatID, "保存失败："+err.Error(), "", nil)
			s.trackBot(chatID, id)
			return
		}
		s.wipe(client, chatID)
		id, _ := client.SendMessage(chatID, fmt.Sprintf("✅ 笔记《%s》已保存（%s）", title, targetName), "", nil)
		s.trackBot(chatID, id)
	case "image":
		id, _ := client.SendMessage(chatID, "请直接发送图片（可带配文），不要发文本。", "", cancelBtn)
		s.trackBot(chatID, id)
	case "file":
		id, _ := client.SendMessage(chatID, "请直接发送文件，不要发文本。", "", cancelBtn)
		s.trackBot(chatID, id)
	case "node":
		if text == "" {
			id, _ := client.SendMessage(chatID, "请发送节点/订阅链接（每行一个，可带名称），或取消。", "", cancelBtn)
			s.trackBot(chatID, id)
			return
		}
		n := s.saveNodesToCategory(client, chatID, targetID, text)
		s.wipe(client, chatID)
		id, _ := client.SendMessage(chatID, fmt.Sprintf("✅ 已入库 %d 个节点/订阅（%s）", n, targetName), "", nil)
		s.trackBot(chatID, id)
	}
	_ = targetID
}

// saveNodesToCategory 解析文本中的节点/订阅链接并存入指定分类，返回成功条数
func (s *Service) saveNodesToCategory(client *Client, chatID int64, catID int64, text string) int {
	n := 0
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		// 支持 "名称 URL" 或纯 URL
		name, url := "", ln
		if isNodeURL(ln) {
			// 纯链接
		} else if parts := strings.Fields(ln); len(parts) >= 2 && isNodeURL(parts[len(parts)-1]) {
			url = parts[len(parts)-1]
			name = strings.TrimSpace(strings.TrimSuffix(ln, url))
		} else {
			continue
		}
		info := node.Parse(url)
		if name == "" {
			name = info.Name
		}
		if name == "" {
			name = "未命名"
		}
		if _, err := s.db.CreateNode(catID, name, url, info.Kind, info.Proto, info.Host, info.Port); err == nil {
			n++
		}
	}
	return n
}

// saveLinksToCategory 解析文本中的网址并存入指定分类，返回成功条数
func (s *Service) saveLinksToCategory(client *Client, chatID int64, catID int64, text string) int {
	n := 0
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		name, url, ok := splitLinkLine(ln)
		if !ok {
			name, url, ok = splitBareDomain(ln)
		}
		if !ok || url == "" {
			continue
		}
		auto := 0
		if name == "" {
			name = fetchTitle(url)
			if name != "" {
				auto = 1
			} else {
				name = url
			}
		}
		if _, err := s.db.CreateLink(catID, name, url, auto); err == nil {
			n++
		}
	}
	return n
}

// handleSearchKeyword 处理分类查找的关键词
func (s *Service) handleSearchKeyword(client *Client, chatID int64, m *Message, p *pending) {
	keyword := strings.TrimSpace(m.Text)
	s.trackUser(chatID, m.MessageID)
	if keyword == "" {
		id, _ := client.SendMessage(chatID, "请输入关键词，或取消。", "", &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
			BtnRow(Btn("❌ 取消", "sc:cancel")),
		}})
		s.trackBot(chatID, id)
		return
	}
	// 机密库需要先验证查询密码
	if p.manualSection == "merchant" {
		p.kind = pSearchQpwd
		id, _ := client.SendMessage(chatID, "查询“"+p.manualName+"”\n请输入查询密码：", "", &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
			BtnRow(Btn("❌ 取消", "sc:cancel")),
		}})
		s.trackBot(chatID, id)
		// 暂存关键词
		p.addText = keyword
		return
	}
	s.doSearch(client, chatID, p, keyword)
}

// handleSearchQpwd 处理搜索流程中的查询密码（复用与查询相同的验证/封禁逻辑）
func (s *Service) handleSearchQpwd(client *Client, chatID int64, m *Message, p *pending) {
	s.trackUser(chatID, m.MessageID)
	now := time.Now()

	maxTries := 3
	if v := s.db.GetSetting("query_pwd_max_tries"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 10 {
			maxTries = n
		}
	}
	tries := p.qpTries
	hash := s.db.GetSetting("query_pwd_hash")
	if hash == "" || !auth.VerifySecret(hash, auth.NormalizeSecret(m.Text)) {
		tries++
		p.qpTries = tries
		s.mu.Lock()
		if s.qpFails[chatID] == nil || now.Sub(s.qpFails[chatID].first) > 10*time.Minute {
			s.qpFails[chatID] = &qpFail{first: now}
		}
		s.qpFails[chatID].count++
		s.mu.Unlock()
		if tries >= maxTries {
			banInfo := s.applyBan()
			s.wipe(client, chatID)
			bmid, _ := client.SendMessage(chatID, "查询密码错误已达 "+strconv.Itoa(maxTries)+" 次，"+banInfo+"。", "", nil)
			s.trackBot(chatID, bmid)
			return
		}
		left := maxTries - tries
		id, _ := client.SendMessage(chatID, "查询密码错误，还剩 "+strconv.Itoa(left)+" 次机会。", "", &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
			BtnRow(Btn("❌ 取消", "sc:cancel")),
		}})
		s.trackBot(chatID, id)
		return
	}
	s.mu.Lock()
	delete(s.qpFails, chatID)
	s.mu.Unlock()
	keyword := p.addText
	p.addText = ""
	s.doSearch(client, chatID, p, keyword)
}

// doSearch 执行模糊搜索并展示结果
func (s *Service) doSearch(client *Client, chatID int64, p *pending, keyword string) {
	kw := strings.ToLower(keyword)
	var lines []string
	section, targetID, targetName := p.manualSection, p.manualID, p.manualName

	match := func(s string) bool {
		return strings.Contains(strings.ToLower(s), kw)
	}

	switch section {
	case "merchant":
		var allSecrets []db.Secret
		if targetID == 0 {
			// 全部服务商
			merchants, _ := s.db.ListMerchants()
			for _, m := range merchants {
				ss, _ := s.db.ListSecrets(m.ID)
				allSecrets = append(allSecrets, ss...)
			}
		} else {
			allSecrets, _ = s.db.ListSecrets(targetID)
		}
		for _, sc := range allSecrets {
			if !match(sc.Label) && !match(sc.Username) && !match(sc.Note) {
				continue
			}
			val, _ := crypto.Decrypt(s.encKey, sc.Value)
			label := sc.Label
			if label == "" {
				label = "（未命名）"
			}
			prefix := ""
			if targetID == 0 {
				if m, err := s.db.GetMerchant(sc.MerchantID); err == nil {
					prefix = "[" + m.Name + "] "
				}
			}
			lines = append(lines, "🔑 "+prefix+label+"\n<code>"+escapeHTML(string(val))+"</code>")
			if len(lines) >= 20 {
				break
			}
		}
	case "script":
		var scripts []db.Script
		var err error
		if targetID == 0 {
			scripts, err = s.db.ListAllScripts()
		} else {
			scripts, err = s.db.ListScriptsByCategory(targetID)
		}
		if err != nil {
			break
		}
		for _, sc := range scripts {
			if !match(sc.Name) && !match(sc.Description) {
				continue
			}
			lines = append(lines, "📜 "+escapeHTML(sc.Name))
			if len(lines) >= 20 {
				break
			}
		}
	case "link":
		var links []*db.Link
		var err error
		if targetID == 0 {
			links, err = s.db.ListAllLinks()
		} else {
			links, err = s.db.ListLinks(targetID)
		}
		if err != nil {
			break
		}
		for _, l := range links {
			if !match(l.Name) && !match(l.URL) {
				continue
			}
			lines = append(lines, "🔗 "+escapeHTML(l.Name)+"\n"+escapeHTML(l.URL))
			if len(lines) >= 20 {
				break
			}
		}
	case "note":
		notes, err := s.db.ListNotesByCategory(targetID)
		if err != nil {
			break
		}
		for _, n := range notes {
			matched := match(n.Title)
			if !matched {
				raw, _ := crypto.Decrypt(s.encKey, n.ContentEnc)
				matched = match(string(raw))
			}
			if !matched {
				continue
			}
			lines = append(lines, "📝 "+escapeHTML(n.Title))
			if len(lines) >= 20 {
				break
			}
		}
	case "image":
		var images []*db.Image
		var err error
		if targetID == 0 {
			images, err = s.db.ListAllImages()
		} else {
			images, err = s.db.ListImages(targetID)
		}
		if err != nil {
			break
		}
		for _, im := range images {
			if !match(im.Caption) {
				continue
			}
			lines = append(lines, "🖼️ "+escapeHTML(im.Caption))
			if len(lines) >= 20 {
				break
			}
		}
	case "file":
		var files []db.File
		var err error
		if targetID == 0 {
			files, err = s.db.ListAllFiles()
		} else {
			files, err = s.db.ListFiles(targetID)
		}
		if err != nil {
			break
		}
		for _, f := range files {
			if !match(f.OrigName) {
				continue
			}
			lines = append(lines, "📄 "+escapeHTML(f.OrigName))
			if len(lines) >= 20 {
				break
			}
		}
	case "node":
		var allNodes []db.Node
		if targetID == 0 {
			cats, _ := s.db.ListNodeCategories()
			for _, c := range cats {
				ns, _ := s.db.ListNodes(c.ID)
				allNodes = append(allNodes, ns...)
			}
		} else {
			allNodes, _ = s.db.ListNodes(targetID)
		}
		for _, n := range allNodes {
			if !match(n.Name) {
				continue
			}
			lines = append(lines, "🌐 "+escapeHTML(n.Name))
			if len(lines) >= 20 {
				break
			}
		}
	}

	s.wipe(client, chatID)
	if len(lines) == 0 {
		id, _ := client.SendMessage(chatID, fmt.Sprintf("在“%s”中没有找到包含“%s”的内容。", targetName, keyword), "", nil)
		s.trackBot(chatID, id)
		return
	}
	msg := fmt.Sprintf("在“%s”中找到 %d 条：\n\n%s", targetName, len(lines), strings.Join(lines, "\n\n"))
	id, _ := client.SendMessage(chatID, msg, "HTML", nil)
	s.trackBot(chatID, id)
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// handleManualCallback 处理手动添加的分类选择
func (s *Service) handleManualCallback(client *Client, chatID int64, data string) {
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if data == "ma:cancel" {
		s.wipe(client, chatID)
		return
	}
	if p == nil || p.kind != pManualPick {
		return
	}
	parts := strings.SplitN(data, ":", 3)
	if len(parts) != 3 {
		return
	}
	section, idStr := parts[1], parts[2]
	if section != p.manualSection {
		return
	}
	id, _ := strconv.ParseInt(idStr, 10, 64)
	name := s.categoryName(section, id)
	if name == "" {
		return
	}
	p.kind = pManualWait
	p.manualID = id
	p.manualName = name
	hint := ""
	switch section {
	case "merchant":
		hint = "请发送文本（每行一条：标签+值）。"
	case "script":
		hint = "请直接粘贴脚本内容。"
	case "link":
		hint = "请发送网址（每行一个，可带名称）。"
	case "note":
		hint = "请发送笔记（第一行是标题，后面是正文）。"
	case "image":
		hint = "请直接发送图片。"
	case "file":
		hint = "请直接发送文件。"
	}
	// 删掉选择器消息
	s.wipeMsgs(client, chatID)
	id2, _ := client.SendMessage(chatID, fmt.Sprintf("已选择 %s%s\n%s", sectionEmoji(section), name, hint), "", &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		BtnRow(Btn("❌ 取消", "ma:cancel")),
	}})
	s.trackBot(chatID, id2)
}

// categoryName 取子分类名称
func (s *Service) categoryName(section string, id int64) string {
	switch section {
	case "merchant":
		if m, err := s.db.GetMerchant(id); err == nil {
			return m.Name
		}
	case "script":
		if cs, err := s.db.ListScriptCategories(); err == nil {
			for _, c := range cs {
				if c.ID == id {
					return c.Name
				}
			}
		}
	case "link":
		if cs, err := s.db.ListLinkCategories(); err == nil {
			for _, c := range cs {
				if c.ID == id {
					return c.Name
				}
			}
		}
	case "note":
		if cs, err := s.db.ListNoteCategories(); err == nil {
			for _, c := range cs {
				if c.ID == id {
					return c.Name
				}
			}
		}
	case "image":
		if cs, err := s.db.ListImageCategories(); err == nil {
			for _, c := range cs {
				if c.ID == id {
					return c.Name
				}
			}
		}
	case "file":
		if cs, err := s.db.ListFileCategories(); err == nil {
			for _, c := range cs {
				if c.ID == id {
					return c.Name
				}
			}
		}
	}
	return ""
}

// handleSearchCallback 处理分类查找的分类选择
func (s *Service) handleSearchCallback(client *Client, chatID int64, data string) {
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if data == "sc:cancel" {
		s.wipe(client, chatID)
		return
	}
	if p == nil || p.kind != pSearchPick {
		return
	}
	parts := strings.SplitN(data, ":", 3)
	if len(parts) != 3 {
		return
	}
	section, idStr := parts[1], parts[2]
	if section != p.manualSection {
		return
	}
	id, _ := strconv.ParseInt(idStr, 10, 64)
	name := "全部"
	if id != 0 {
		name = s.categoryName(section, id)
		if name == "" {
			return
		}
	}
	p.kind = pSearchKeyword
	p.manualID = id
	p.manualName = name
	s.wipeMsgs(client, chatID)
	id2, _ := client.SendMessage(chatID, fmt.Sprintf("在 %s%s 中搜索\n请输入关键词：", sectionEmoji(section), name), "", &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		BtnRow(Btn("❌ 取消", "sc:cancel")),
	}})
	s.trackBot(chatID, id2)
}

// handleManualSectionCallback 手动添加：版块选择后，显示子分类
func (s *Service) handleManualSectionCallback(client *Client, chatID int64, data string) {
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if data == "mas:cancel" {
		s.wipe(client, chatID)
		return
	}
	if p == nil || p.kind != pManualSection {
		return
	}
	section := strings.TrimPrefix(data, "mas:")
	valid := false
	for _, sec := range sections {
		if sec.id == section {
			valid = true
			break
		}
	}
	if !valid {
		return
	}
	p.kind = pManualPick
	p.manualSection = section
	s.wipeMsgs(client, chatID)
	s.sendTargetPicker(client, chatID, "ma", section, "请选择分类：", false)
}

// handleSearchSectionCallback 查找：版块选择后，显示子分类（含全部）
func (s *Service) handleSearchSectionCallback(client *Client, chatID int64, data string) {
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if data == "scs:cancel" {
		s.wipe(client, chatID)
		return
	}
	if p == nil || p.kind != pSearchSection {
		return
	}
	section := strings.TrimPrefix(data, "scs:")
	valid := false
	for _, sec := range sections {
		if sec.id == section {
			valid = true
			break
		}
	}
	if !valid {
		return
	}
	p.kind = pSearchPick
	p.manualSection = section
	s.wipeMsgs(client, chatID)
	s.sendTargetPicker(client, chatID, "sc", section, "请选择分类（可搜全部）：", true)
}

func (s *Service) sendStart(client *Client, chatID int64) {
	id, _ := client.SendMessage(chatID, "SecretDock 录入助手\n\n添加：多行文本，第一行为服务商名，后面每行一条（标签+值）。\n脚本：第一行发“脚本”，后面粘贴脚本内容。\n网址：第一行发“网址”，后面每行 名字+网址（不带网址的行是分类）。\n笔记：第一行发“笔记”，第二行标题，后面全是内容。\n提醒：第一行发“提醒”，第二行标题（是分类名则自动归类），然后输入时间如“明天12点”。\n查询：只发服务商名一行，我会向你索取查询密码。\n手动添加：/add，先选版块再选分类。\n分类查找：/search，先选版块再输关键词。\n取消：/cancel", "", nil)
	s.trackBot(chatID, id)
}

// ---------- reminder wizard ----------

func shanghaiLoc() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Local
	}
	return loc
}
func remindRepeatText(repeat, detail string) string {
	switch repeat {
	case "daily":
		return "每天"
	case "weekly":
		wd := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
		if i, err := strconv.Atoi(detail); err == nil && i >= 0 && i < 7 {
			return "每周" + wd[i]
		}
		return "每周"
	case "monthly":
		return "每月" + detail + "号"
	case "yearly":
		return "每年"
	}
	return "不重复"
}
func nextRemindTime(prev int64, repeat, detail string) int64 {
	loc := shanghaiLoc()
	t := time.Unix(prev, 0).In(loc)
	switch repeat {
	case "daily":
		return t.AddDate(0, 0, 1).Unix()
	case "weekly":
		wd, err := strconv.Atoi(detail)
		if err != nil {
			wd = int(t.Weekday())
		}
		// 找到下一个该周几
		days := (wd - int(t.Weekday()) + 7) % 7
		if days == 0 {
			days = 7
		}
		return t.AddDate(0, 0, days).Unix()
	case "monthly":
		day, err := strconv.Atoi(detail)
		if err != nil || day < 1 || day > 31 {
			day = t.Day()
		}
		// 下个月的这一天（处理月末）
		y, m, _ := t.Date()
		m++
		if m > 12 {
			m = 1
			y++
		}
		lastDay := time.Date(y, m+1, 0, 0, 0, 0, 0, loc).Day()
		if day > lastDay {
			day = lastDay
		}
		return time.Date(y, m, day, t.Hour(), t.Minute(), 0, 0, loc).Unix()
	case "yearly":
		y, m, d := t.Date()
		return time.Date(y+1, m, d, t.Hour(), t.Minute(), 0, 0, loc).Unix()
	}
	return 0
}

// ---------- reminder (simplified) ----------

func (s *Service) startReminder(client *Client, chatID int64, text string) {
	parts := strings.SplitN(text, "\n", 3)
	title := ""
	note := ""
	if len(parts) >= 2 {
		title = strings.TrimSpace(parts[1])
	}
	if len(parts) == 3 {
		note = strings.Trim(parts[2], "\n")
	}
	if title == "" {
		id, _ := client.SendMessage(chatID, "提醒标题不能为空，请重新发送：\n提醒\n标题", "", nil)
		s.trackBot(chatID, id)
		return
	}
	p := s.ensurePending(chatID)
	p.kind = pRemindTime
	p.remTitle = title
	p.remNote = note
	s.wipeMsgs(client, chatID)
	id, _ := client.SendMessage(chatID, "⏰ 「"+title+"」\n请输入提醒时间，例如：\n• 明天12点 / 后天18:00\n• 10月20 18点30 / 20261020 18:30\n• 每天8点 / 每周一9点 / 每月5号10点", "", &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		BtnRow(Btn("❌ 取消", "rm:cancel")),
	}})
	s.trackBot(chatID, id)
}

// handleRemindText 解析自然语言时间
func (s *Service) handleRemindText(client *Client, chatID int64, m *Message, p *pending) {
	text := strings.TrimSpace(m.Text)
	s.trackUser(chatID, m.MessageID)
	if p.kind != pRemindTime {
		return
	}
	ts, repeat, detail, err := parseRemindTime(text)
	if err != nil {
		id, _ := client.SendMessage(chatID, "没理解这个时间，请换个说法，例如：\n• 明天12点 / 后天18:00\n• 10月20 18点30 / 20261020 18:30\n• 每天8点 / 每周一9点 / 每月5号10点", "", &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
			BtnRow(Btn("❌ 取消", "rm:cancel")),
		}})
		s.trackBot(chatID, id)
		return
	}
	p.remDate = time.Unix(ts, 0).In(shanghaiLoc()).Format("2006-01-02")
	p.remTime = time.Unix(ts, 0).In(shanghaiLoc()).Format("15:04")
	p.remRepeat = repeat
	p.remDetail = detail
	p.kind = pRemindConfirm
	// 存时间戳供 saveReminder 用
	s.mu.Lock()
	p.manualID = ts
	s.mu.Unlock()
	s.wipeMsgs(client, chatID)
	s.sendRemindConfirm(client, chatID, p)
}

// parseRemindTime 解析自然语言时间，返回 (unix时间戳, repeat类型, repeat详情, 错误)

func (s *Service) handleRemindCallback(client *Client, chatID int64, data string) {
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if data == "rm:cancel" {
		s.wipe(client, chatID)
		return
	}
	if p == nil || p.kind != pRemindConfirm {
		return
	}
	if data == "rm:confirm" {
		s.saveReminder(client, chatID, p)
	}
}

func (s *Service) sendRemindConfirm(client *Client, chatID int64, p *pending) {
	s.mu.Lock()
	ts := p.manualID
	s.mu.Unlock()
	dt := time.Unix(ts, 0).In(shanghaiLoc())
	times := 3
	if v := s.db.GetSetting("reminder_default_times"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 10 {
			times = n
		}
	}
	msg := fmt.Sprintf("⏰ 确认创建提醒\n\n标题：%s\n时间：%s\n重复：%s\n连发：%d 条",
		p.remTitle, dt.Format("2006-01-02 15:04"),
		remindRepeatText(p.remRepeat, p.remDetail), times)
	if p.remNote != "" {
		msg += "\n备注：" + p.remNote
	}
	rows := [][]InlineKeyboardButton{
		BtnRow(Btn("✅ 确认创建", "rm:confirm")),
		BtnRow(Btn("❌ 取消", "rm:cancel")),
	}
	id, _ := client.SendMessage(chatID, msg, "", &InlineKeyboardMarkup{InlineKeyboard: rows})
	s.trackBot(chatID, id)
}

func (s *Service) saveReminder(client *Client, chatID int64, p *pending) {
	s.mu.Lock()
	ts := p.manualID
	s.mu.Unlock()
	times := 3
	if v := s.db.GetSetting("reminder_default_times"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 10 {
			times = n
		}
	}
	// 重复提醒首次时间已过，推到下一次
	if ts <= time.Now().Unix() && p.remRepeat != "none" {
		ts = nextRemindTime(ts, p.remRepeat, p.remDetail)
	}
	uncatID, _ := s.db.FindOrCreateReminderCategory("")
	rid, err := s.db.CreateReminder(db.Reminder{
		CategoryID:   uncatID,
		Title:        p.remTitle,
		Note:         p.remNote,
		RemindAt:     ts,
		RepeatType:   p.remRepeat,
		RepeatDetail: p.remDetail,
		Times:        times,
		IsActive:     true,
	})
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	s.db.Audit("tg_reminder_add", "title="+p.remTitle, "telegram")
	s.wipeMsgs(client, chatID)
	s.askCategory(client, chatID, "reminder", []int64{rid}, "提醒「"+p.remTitle+"」")
}

// normTimeInput 全角转半角、统一标点
func normTimeInput(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '０' && r <= '９':
			b.WriteRune(r - '０' + '0')
		case r == '：':
			b.WriteRune(':')
		case r == '，' || r == '、':
			b.WriteRune(' ')
		case r == '。':
			b.WriteRune('.')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// parseRemindTime 解析自然语言时间，返回 (unix时间戳, repeat类型, repeat详情, 错误)
// 支持的日期格式：
//   今天/明天/后天/大后天
//   20261020 / 2026-10-20 / 2026/10/20 / 2026.10.20 / 2026年10月20日
//   26年10月20日 / 26年10月20
//   10月20日 / 10月20 / 10-20 / 1020
// 支持的时间格式：
//   18:30 / 18：30 / 18点30分 / 18点半 / 18点 / 18
//   2026 10 20 18 30（空格分隔）
// 支持的重复：每天 / 每周X(一二三四五六日天) / 每月X号 / 每年
func parseRemindTime(input string) (int64, string, string, error) {
	loc := shanghaiLoc()
	now := time.Now().In(loc)
	s := normTimeInput(input)
	if s == "" {
		return 0, "", "", fmt.Errorf("empty")
	}

	repeat := "none"
	detail := ""

	// ---- 重复规则 ----
	if strings.Contains(s, "每天") {
		repeat = "daily"
		s = strings.Replace(s, "每天", " ", 1)
	} else if idx := strings.Index(s, "每周"); idx >= 0 {
		rest := strings.TrimSpace(s[idx+len("每周"):])
		wdMap := []struct {
			k string
			v int
		}{{"一", 1}, {"二", 2}, {"三", 3}, {"四", 4}, {"五", 5}, {"六", 6}, {"日", 0}, {"天", 0}, {"七", 0}}
		found := -1
		skip := 0
		for _, w := range wdMap {
			if strings.HasPrefix(rest, w.k) {
				found = w.v
				skip = len(w.k)
				break
			}
		}
		if found < 0 {
			return 0, "", "", fmt.Errorf("weekly needs weekday")
		}
		repeat = "weekly"
		detail = strconv.Itoa(found)
		s = strings.TrimSpace(s[:idx] + " " + rest[skip:])
	} else if idx := strings.Index(s, "每月"); idx >= 0 {
		rest := strings.TrimSpace(s[idx+len("每月"):])
		m := regexp.MustCompile(`^(\d{1,2})`).FindStringSubmatch(rest)
		if m == nil {
			return 0, "", "", fmt.Errorf("monthly needs day")
		}
		day, _ := strconv.Atoi(m[1])
		if day < 1 || day > 31 {
			return 0, "", "", fmt.Errorf("monthly day 1-31")
		}
		repeat = "monthly"
		detail = strconv.Itoa(day)
		rest = strings.TrimSpace(rest[len(m[1]):])
		rest = strings.TrimPrefix(rest, "号")
		rest = strings.TrimPrefix(rest, "日")
		s = strings.TrimSpace(s[:idx] + " " + rest)
	} else if strings.Contains(s, "每年") {
		repeat = "yearly"
		s = strings.Replace(s, "每年", " ", 1)
	}
	s = strings.Join(strings.Fields(s), " ")

	// ---- 日期 ----
	y, mo, d := now.Date()
	hasDate := false

	// 相对日期
	relMap := []struct {
		k string
		n int
	}{{"大后天", 3}, {"后天", 2}, {"明天", 1}, {"今日", 0}, {"今天", 0}}
	// 下周X
	if !hasDate {
		if idx := strings.Index(s, "下周"); idx >= 0 {
			rest := strings.TrimSpace(s[idx+len("下周"):])
			wdMap := map[string]int{"一": 1, "二": 2, "三": 3, "四": 4, "五": 5, "六": 6, "日": 0, "天": 0}
			for k, v := range wdMap {
				if strings.HasPrefix(rest, k) {
					days := (v - int(now.Weekday()) + 7) % 7
					if days == 0 {
						days = 7
					}
					days += 7 // 下周
					t := now.AddDate(0, 0, days)
					y, mo, d = t.Date()
					hasDate = true
					s = strings.TrimSpace(s[:idx] + " " + rest[len(k):])
					break
				}
			}
		}
	}
	for _, r := range relMap {
		if strings.Contains(s, r.k) {
			t := now.AddDate(0, 0, r.n)
			y, mo, d = t.Date()
			hasDate = true
			s = strings.TrimSpace(strings.Replace(s, r.k, " ", 1))
			break
		}
	}

	if !hasDate {
		// YYYYMMDD
		if m := regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})\b`).FindStringSubmatch(s); m != nil {
			y, _ = strconv.Atoi(m[1])
			moI, _ := strconv.Atoi(m[2])
			dI, _ := strconv.Atoi(m[3])
			mo, d = time.Month(moI), dI
			hasDate = true
			s = strings.TrimSpace(s[len(m[0]):])
		} else if m := regexp.MustCompile(`^(\d{4})[/.\-年](\d{1,2})[/.\-月](\d{1,2})日?`).FindStringSubmatch(s); m != nil {
			y, _ = strconv.Atoi(m[1])
			moI, _ := strconv.Atoi(m[2])
			dI, _ := strconv.Atoi(m[3])
			mo, d = time.Month(moI), dI
			hasDate = true
			s = strings.TrimSpace(s[len(m[0]):])
		} else if m := regexp.MustCompile(`^(\d{2})年(\d{1,2})月(\d{1,2})日?`).FindStringSubmatch(s); m != nil {
			yy, _ := strconv.Atoi(m[1])
			y = 2000 + yy
			moI, _ := strconv.Atoi(m[2])
			dI, _ := strconv.Atoi(m[3])
			mo, d = time.Month(moI), dI
			hasDate = true
			s = strings.TrimSpace(s[len(m[0]):])
		} else if m := regexp.MustCompile(`^(\d{1,2})月(\d{1,2})[日号]?`).FindStringSubmatch(s); m != nil {
			moI, _ := strconv.Atoi(m[1])
			dI, _ := strconv.Atoi(m[2])
			mo, d = time.Month(moI), dI
			hasDate = true
			s = strings.TrimSpace(s[len(m[0]):])
		} else if m := regexp.MustCompile(`^(\d{4})\s+(\d{1,2})\s+(\d{1,2})\b`).FindStringSubmatch(s); m != nil {
			// YYYY MM DD 空格分隔
			y, _ = strconv.Atoi(m[1])
			moI, _ := strconv.Atoi(m[2])
			dI, _ := strconv.Atoi(m[3])
			mo, d = time.Month(moI), dI
			hasDate = true
			s = strings.TrimSpace(s[len(m[0]):])
		} else if m := regexp.MustCompile(`^(\d{1,2})[/.\-](\d{1,2})\b`).FindStringSubmatch(s); m != nil {
			moI, _ := strconv.Atoi(m[1])
			dI, _ := strconv.Atoi(m[2])
			// 排除 YYYY-MM 误判（已在前面处理），这里要求后面不紧跟更多数字
			mo, d = time.Month(moI), dI
			hasDate = true
			s = strings.TrimSpace(s[len(m[0]):])
		} else if m := regexp.MustCompile(`^(\d{4})\b`).FindStringSubmatch(s); m != nil && len(m[1]) == 4 {
			// MMDD 紧凑格式，如 1020
			moI, _ := strconv.Atoi(m[1][:2])
			dI, _ := strconv.Atoi(m[1][2:])
			if moI >= 1 && moI <= 12 && dI >= 1 && dI <= 31 {
				mo, d = time.Month(moI), dI
				hasDate = true
				s = strings.TrimSpace(s[len(m[0]):])
			}
		}
	}
	// 日期合法性
	if hasDate {
		if mo < 1 || mo > 12 || d < 1 || d > 31 {
			return 0, "", "", fmt.Errorf("bad date")
		}
		// 处理月末（如2月30日）
		lastDay := time.Date(y, mo+1, 0, 0, 0, 0, 0, loc).Day()
		if d > lastDay {
			d = lastDay
		}
	}
	s = strings.Join(strings.Fields(s), " ")

	// ---- 时间 ----
	hour, minute := 9, 0
	hasTime := false
	if s != "" {
		// HH:MM
		if m := regexp.MustCompile(`^(\d{1,2}):(\d{1,2})`).FindStringSubmatch(s); m != nil {
			h, _ := strconv.Atoi(m[1])
			mi, _ := strconv.Atoi(m[2])
			if h < 24 && mi < 60 {
				hour, minute = h, mi
				hasTime = true
				s = strings.TrimSpace(s[len(m[0]):])
			}
		} else if m := regexp.MustCompile(`^(\d{1,2})点半`).FindStringSubmatch(s); m != nil {
			h, _ := strconv.Atoi(m[1])
			if h < 24 {
				hour, minute = h, 30
				hasTime = true
				s = strings.TrimSpace(s[len(m[0]):])
			}
		} else if m := regexp.MustCompile(`^(\d{1,2})点(\d{1,2})分?`).FindStringSubmatch(s); m != nil {
			h, _ := strconv.Atoi(m[1])
			mi, _ := strconv.Atoi(m[2])
			if h < 24 && mi < 60 {
				hour, minute = h, mi
				hasTime = true
				s = strings.TrimSpace(s[len(m[0]):])
			}
		} else if m := regexp.MustCompile(`^(\d{1,2})点`).FindStringSubmatch(s); m != nil {
			h, _ := strconv.Atoi(m[1])
			if h < 24 {
				hour, minute = h, 0
				hasTime = true
				s = strings.TrimSpace(s[len(m[0]):])
			}
		} else if m := regexp.MustCompile(`^(\d{1,2})\s+(\d{1,2})$`).FindStringSubmatch(strings.TrimSpace(s)); m != nil {
			// "18 30" 空格分隔的时间
			h, _ := strconv.Atoi(m[1])
			mi, _ := strconv.Atoi(m[2])
			if h < 24 && mi < 60 {
				hour, minute = h, mi
				hasTime = true
				s = ""
			}
		} else if m := regexp.MustCompile(`^(\d{1,2})$`).FindStringSubmatch(strings.TrimSpace(s)); m != nil {
			// 纯数字：可能是 HH 或 HHMM
			if len(m[1]) <= 2 {
				h, _ := strconv.Atoi(m[1])
				if h < 24 {
					hour, minute = h, 0
					hasTime = true
					s = ""
				}
			} else if len(m[1]) == 4 {
				h, _ := strconv.Atoi(m[1][:2])
				mi, _ := strconv.Atoi(m[1][2:])
				if h < 24 && mi < 60 {
					hour, minute = h, mi
					hasTime = true
					s = ""
				}
			}
		}
	}
	// 还有残留无法解析的内容
	if strings.TrimSpace(s) != "" {
		return 0, "", "", fmt.Errorf("unparsed: %s", s)
	}

	// ---- 组合 ----
	if !hasDate {
		switch repeat {
		case "daily":
			// 今天
		case "weekly":
			wd, _ := strconv.Atoi(detail)
			days := (wd - int(now.Weekday()) + 7) % 7
			t := now.AddDate(0, 0, days)
			y, mo, d = t.Date()
		case "monthly":
			day, _ := strconv.Atoi(detail)
			yy, mm, _ := now.Date()
			y, mo = yy, mm
			lastDay := time.Date(y, mo+1, 0, 0, 0, 0, 0, loc).Day()
			if day > lastDay {
				day = lastDay
			}
			d = day
		}
	}
	ts := time.Date(y, mo, d, hour, minute, 0, 0, loc).Unix()

	// 不重复且时间已过 → 报错，让用户明确指定
	if repeat == "none" && ts <= now.Unix() {
		return 0, "", "", fmt.Errorf("time passed, specify explicitly")
	}
	_ = hasTime
	return ts, repeat, detail, nil
}

// startNodes handles the "订阅"/"节点"/"订阅节点" protocol:
// line 1 = marker, line 2 = optional name (if not a URL), rest = node/subscription URLs.
// URLs are auto-detected: http(s):// = subscription, others (vless://, vmess://...) = single node.
func (s *Service) startNodes(client *Client, chatID int64, text string) {
	lines := strings.Split(text, "\n")
	name := ""
	var urls []string
	for _, ln := range lines[1:] {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if isNodeURL(ln) {
			urls = append(urls, ln)
		} else if name == "" && len(urls) == 0 {
			// first non-URL line = name
			name = ln
		} else {
			// 非 URL 行夹在中间，忽略
		}
	}
	if len(urls) == 0 {
		id, _ := client.SendMessage(chatID, "没解析出链接。请这样发：\n订阅\nXX云\nhttps://xxx.com/sub", "", nil)
		s.trackBot(chatID, id)
		return
	}
	uncatID, _ := s.db.FindOrCreateNodeCategory("")
	var ids []int64
	for i, u := range urls {
		nm := name
		if len(urls) > 1 {
			if nm != "" {
				nm = fmt.Sprintf("%s-%d", nm, i+1)
			} else {
				info := node.Parse(u)
				nm = info.Name
				if nm == "" {
					nm = "未命名"
				}
			}
		} else if nm == "" {
			info := node.Parse(u)
			nm = info.Name
			if nm == "" {
				nm = "未命名"
			}
		}
		info := node.Parse(u)
		if id, err := s.db.CreateNode(uncatID, nm, u, info.Kind, info.Proto, info.Host, info.Port); err == nil {
			ids = append(ids, id)
		}
	}
	s.db.Audit("tg_nodes_add", fmt.Sprintf("total=%d", len(ids)), "telegram")
	s.wipeMsgs(client, chatID)
	label := name
	if label == "" {
		label = fmt.Sprintf("%d 个节点/订阅", len(ids))
	}
	s.askCategory(client, chatID, "node", ids, label)
}

// startSSH handles the "SSH" protocol:
// line1 = "SSH", line2 = host:port, line3 = username, line4 = password
func (s *Service) startSSH(client *Client, chatID int64, text string) {
	lines := strings.Split(text, "\n")
	var parts []string
	for _, ln := range lines[1:] {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			parts = append(parts, ln)
		}
	}
	if len(parts) < 3 {
		id, _ := client.SendMessage(chatID, "格式不对。请这样发：\nSSH\n110.110.110.110:22\nroot\n密码", "", nil)
		s.trackBot(chatID, id)
		return
	}
	host, port := parseSSHAddr(parts[0])
	username := parts[1]
	password := parts[2]
	if host == "" || password == "" {
		id, _ := client.SendMessage(chatID, "主机和密码不能为空", "", nil)
		s.trackBot(chatID, id)
		return
	}
	if username == "" {
		username = "root"
	}
	enc, err := crypto.Encrypt(s.encKey, []byte(password))
	if err != nil {
		id, _ := client.SendMessage(chatID, "加密失败", "", nil)
		s.trackBot(chatID, id)
		return
	}
	// 先问名称，再选分类
	s.wipeMsgs(client, chatID)
	pp := s.ensurePending(chatID)
	pp.kind = pSSHNeedName
	pp.sshHost = host
	pp.sshPort = port
	pp.sshUser = username
	pp.sshPassEnc = enc
	id, _ := client.SendMessage(chatID, fmt.Sprintf("主机 %s 已解析，请发送一个名称（如 RN圣何塞）：", host), "", nil)
	s.trackBot(chatID, id)
}

// handleSSHNameReply 用户输入 SSH 主机名称后，保存并选分类
func (s *Service) handleSSHNameReply(client *Client, chatID int64, p *pending, text string) {
	name := strings.TrimSpace(text)
	if name == "" || strings.Contains(name, "\n") || len([]rune(name)) > 40 {
		id, _ := client.SendMessage(chatID, "名称不合法，请重新发送一个名称：", "", nil)
		s.trackBot(chatID, id)
		return
	}
	uncatID, _ := s.db.FindOrCreateSSHCategory("")
	id, err := s.db.CreateSSHHost(uncatID, name, p.sshHost, p.sshPort, p.sshUser, p.sshPassEnc, "", "", "", "", "", "")
	if err != nil {
		id2, _ := client.SendMessage(chatID, "保存失败: "+err.Error(), "", nil)
		s.trackBot(chatID, id2)
		return
	}
	s.db.Audit("tg_ssh_add", fmt.Sprintf("host=%s name=%s", p.sshHost, name), "telegram")
	s.wipeMsgs(client, chatID)
	s.askCategory(client, chatID, "ssh", []int64{id}, name)
}

// parseSSHAddr 解析 "host:port" / "[ipv6]:port" / "host" / "ipv6"
func parseSSHAddr(s string) (string, int) {
	s = strings.TrimSpace(s)
	port := 22
	// [ipv6]:port
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "]"); i > 0 {
			host := s[1:i]
			rest := strings.TrimPrefix(s[i+1:], ":")
			if p, err := strconv.Atoi(rest); err == nil {
				port = p
			}
			return host, port
		}
	}
	// 裸 IPv6（含多个冒号）不带端口
	if strings.Count(s, ":") > 1 {
		return s, port
	}
	// host:port
	if i := strings.LastIndex(s, ":"); i > 0 {
		if p, err := strconv.Atoi(s[i+1:]); err == nil {
			return s[:i], p
		}
	}
	return s, port
}

// isHttpURL 判断是不是 http(s) 链接
func isHttpURL(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// isNodeURL 判断一行是不是节点/订阅链接
func isNodeURL(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	// 常见代理协议
	for _, p := range []string{"vless://", "vmess://", "trojan://", "ss://", "ssr://",
		"hysteria://", "hysteria2://", "hy2://", "tuic://", "wireguard://", "socks5://", "socks://"} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// askNodeDest 检测到裸节点链接时，让用户选择存入订阅节点还是机密库
func (s *Service) askNodeDest(client *Client, chatID int64, text string) {
	s.mu.Lock()
	s.pending[chatID] = &pending{kind: pNodeChoice, addText: text, expires: time.Now().Add(10 * time.Minute)}
	s.mu.Unlock()
	// 预览：只显示第一行，太长截断
	first := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	if len([]rune(first)) > 40 {
		first = string([]rune(first)[:40]) + "…"
	}
	msg := fmt.Sprintf("检测到可能是节点链接：\n%s\n请选择存入位置：", first)
	id, _ := client.SendMessage(chatID, msg, "", &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		BtnRow(Btn("🌐 存入订阅节点", "node:to_nodes")),
		BtnRow(Btn("🏦 存入机密库", "node:to_merchant")),
		BtnRow(Btn("取消", "node:cancel")),
	}})
	s.trackBot(chatID, id)
}

// ---- 统一分类选择 ----

// askCategory 统一分类选择：内容已先存入未分类，显示分类按钮供选择
// section: script|link|note|image|file|node|reminder
// ids: 已存入未分类的记录 ID 列表
// label: 用于显示的内容描述（如脚本名、笔记标题）
func (s *Service) askCategory(client *Client, chatID int64, section string, ids []int64, label string) {
	// 取消旧的 timer
	s.mu.Lock()
	if old := s.pending[chatID]; old != nil && old.catTimer != nil {
		old.catTimer.Stop()
	}
	s.mu.Unlock()

	cats := s.listCatsForSection(section)
	// 构建按钮：每行2个分类
	var rows [][]InlineKeyboardButton
	for i := 0; i < len(cats); i += 2 {
		row := []InlineKeyboardButton{Btn(cats[i].name, fmt.Sprintf("cat:%s:%d", section, cats[i].id))}
		if i+1 < len(cats) {
			row = append(row, Btn(cats[i+1].name, fmt.Sprintf("cat:%s:%d", section, cats[i+1].id)))
		}
		rows = append(rows, BtnRow(row...))
	}
	rows = append(rows, BtnRow(Btn("✏️ 新分类", fmt.Sprintf("cat:%s:new", section))))
	rows = append(rows, BtnRow(Btn("取消", "cat:cancel")))

	p := &pending{
		kind:       pNeedCategory,
		catSection: section,
		catIDs:     ids,
		expires:    time.Now().Add(1 * time.Minute),
	}
	// 1分钟超时：已在未分类，无需操作，只清理 pending 并通知
	p.catTimer = time.AfterFunc(1*time.Minute, func() {
		s.mu.Lock()
		cur := s.pending[chatID]
		if cur == nil || cur.kind != pNeedCategory || cur.catSection != section {
			s.mu.Unlock()
			return
		}
		s.pending[chatID] = nil
		s.mu.Unlock()
		s.wipeMsgs(client, chatID)
		id, _ := client.SendMessage(chatID, "⏰ 1分钟无操作，已存入「未分类」", "", nil)
		s.trackBot(chatID, id)
	})
	s.mu.Lock()
	s.pending[chatID] = p
	s.mu.Unlock()

	msg := fmt.Sprintf("「%s」请选择分类（1分钟无操作自动存入未分类）：", label)
	id, _ := client.SendMessage(chatID, msg, "", &InlineKeyboardMarkup{InlineKeyboard: rows})
	s.trackBot(chatID, id)
}


type catInfo struct{ id int64; name string }

func (s *Service) listCatsForSection(section string) []catInfo {
	var out []catInfo
	switch section {
	case "script":
		if cs, err := s.db.ListScriptCategories(); err == nil {
			for _, c := range cs {
				out = append(out, catInfo{c.ID, c.Name})
			}
		}
	case "link":
		if cs, err := s.db.ListLinkCategories(); err == nil {
			for _, c := range cs {
				out = append(out, catInfo{c.ID, c.Name})
			}
		}
	case "note":
		if cs, err := s.db.ListNoteCategories(); err == nil {
			for _, c := range cs {
				out = append(out, catInfo{c.ID, c.Name})
			}
		}
	case "image":
		if cs, err := s.db.ListImageCategories(); err == nil {
			for _, c := range cs {
				out = append(out, catInfo{c.ID, c.Name})
			}
		}
	case "file":
		if cs, err := s.db.ListFileCategories(); err == nil {
			for _, c := range cs {
				out = append(out, catInfo{c.ID, c.Name})
			}
		}
	case "node":
		if cs, err := s.db.ListNodeCategories(); err == nil {
			for _, c := range cs {
				out = append(out, catInfo{c.ID, c.Name})
			}
		}
	case "reminder":
		if cs, err := s.db.ListReminderCategories(); err == nil {
			for _, c := range cs {
				out = append(out, catInfo{c.ID, c.Name})
			}
		}
	case "ssh":
		if cs, err := s.db.ListSSHCategories(); err == nil {
			for _, c := range cs {
				out = append(out, catInfo{c.ID, c.Name})
			}
		}
	}
	return out
}

func (s *Service) findOrCreateUncat(section string) (int64, error) {
	switch section {
	case "script":
		return s.db.FindOrCreateScriptCategory("")
	case "link":
		return s.db.FindOrCreateLinkCategory("")
	case "note":
		return s.db.FindOrCreateNoteCategory("")
	case "image":
		return s.db.FindOrCreateImageCategory("")
	case "file":
		return s.db.FindOrCreateFileCategory("")
	case "node":
		return s.db.FindOrCreateNodeCategory("")
	case "reminder":
		return s.db.FindOrCreateReminderCategory("")
	}
	return 0, fmt.Errorf("unknown section")
}




// handleCatCallback 处理分类选择回调
// data 格式: cat:<section>:<id> | cat:<section>:new | cat:cancel
func (s *Service) handleCatCallback(client *Client, chatID int64, data string) {
	s.mu.Lock()
	p := s.pending[chatID]
	s.mu.Unlock()
	if p == nil || p.kind != pNeedCategory {
		return
	}
	if p.catTimer != nil {
		p.catTimer.Stop()
	}
	parts := strings.Split(data, ":")
	if len(parts) < 2 {
		return
	}
	if parts[1] == "cancel" {
		// 取消：内容已在未分类，直接清理
		s.mu.Lock()
		s.pending[chatID] = nil
		s.mu.Unlock()
		s.wipe(client, chatID)
		id, _ := client.SendMessage(chatID, "已存入「未分类」", "", nil)
		s.trackBot(chatID, id)
		return
	}
	section := parts[1]
	if section != p.catSection {
		return
	}
	if len(parts) >= 3 && parts[2] == "new" {
		// 输入新分类
		s.mu.Lock()
		p.kind = pNewCategory
		p.expires = time.Now().Add(1 * time.Minute)
		// 新分类输入也1分钟超时
		if p.catTimer != nil {
			p.catTimer.Stop()
		}
		p.catTimer = time.AfterFunc(1*time.Minute, func() {
			s.mu.Lock()
			cur := s.pending[chatID]
			if cur == nil || cur.kind != pNewCategory {
				s.mu.Unlock()
				return
			}
			s.pending[chatID] = nil
			s.mu.Unlock()
			s.wipeMsgs(client, chatID)
			id, _ := client.SendMessage(chatID, "⏰ 1分钟无操作，已存入「未分类」", "", nil)
			s.trackBot(chatID, id)
		})
		s.mu.Unlock()
		s.wipeMsgs(client, chatID)
		id, _ := client.SendMessage(chatID, "请输入新分类名称（1分钟无操作自动存入未分类）：", "", nil)
		s.trackBot(chatID, id)
		return
	}
	// 选择现有分类
	if len(parts) < 3 {
		return
	}
	catID, _ := strconv.ParseInt(parts[2], 10, 64)
	catName := s.getCatName(section, catID)
	if catName == "" {
		return
	}
	s.moveToCategory(client, chatID, p, catID, catName)
}

func (s *Service) getCatName(section string, catID int64) string {
	for _, c := range s.listCatsForSection(section) {
		if c.id == catID {
			return c.name
		}
	}
	return ""
}

// moveToCategory 将待选记录移动到指定分类
func (s *Service) moveToCategory(client *Client, chatID int64, p *pending, catID int64, catName string) {
	var err error
	switch p.catSection {
	case "script":
		err = s.db.MoveScriptsToCategory(p.catIDs, catID)
	case "link":
		err = s.db.MoveLinks(p.catIDs, catID)
	case "note":
		err = s.db.MoveNotesToCategory(p.catIDs, catID)
	case "image":
		err = s.db.MoveImages(p.catIDs, catID)
	case "file":
		err = s.db.MoveFilesToCategory(p.catIDs, catID)
	case "node":
		err = s.db.MoveNodesToCategory(p.catIDs, catID)
	case "reminder":
		err = s.db.MoveRemindersToCategory(p.catIDs, catID)
	case "ssh":
		err = s.db.MoveSSHHostsToCategory(p.catIDs, catID)
	}
	s.mu.Lock()
	s.pending[chatID] = nil
	s.mu.Unlock()
	s.wipe(client, chatID)
	if err != nil {
		id, _ := client.SendMessage(chatID, "移动失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	id, _ := client.SendMessage(chatID, fmt.Sprintf("✅ 已存入「%s」", catName), "", nil)
	s.trackBot(chatID, id)
}

// handleNewCategoryInput 用户输入新分类名称
func (s *Service) handleNewCategoryInput(client *Client, chatID int64, p *pending, text string) {
	name := strings.TrimSpace(text)
	if name == "" || len([]rune(name)) > 20 {
		id, _ := client.SendMessage(chatID, "分类名称无效（1-20字），请重新输入：", "", nil)
		s.trackBot(chatID, id)
		return
	}
	if p.catTimer != nil {
		p.catTimer.Stop()
	}
	// 创建分类
	var catID int64
	var err error
	switch p.catSection {
	case "script":
		catID, err = s.db.FindOrCreateScriptCategory(name)
	case "link":
		catID, err = s.db.CreateLinkCategory(name)
	case "note":
		catID, err = s.db.FindOrCreateNoteCategory(name)
	case "image":
		catID, err = s.db.CreateImageCategory(name)
	case "file":
		catID, err = s.db.CreateFileCategory(name)
	case "node":
		catID, err = s.db.CreateNodeCategory(name)
	case "reminder":
		catID, err = s.db.FindOrCreateReminderCategory(name)
	case "ssh":
		catID, err = s.db.CreateSSHCategory(name)
	}
	if err != nil {
		id, _ := client.SendMessage(chatID, "创建失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	s.moveToCategory(client, chatID, p, catID, name)
}

// saveScriptToUncat 保存脚本到未分类，然后询问分类
func (s *Service) saveScriptToUncat(client *Client, chatID int64, name, ver, desc, author, content string) {
	enc, err := crypto.Encrypt(s.encKey, []byte(content))
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存脚本失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	uncatID, _ := s.db.FindOrCreateScriptCategory("")
	id, err := s.db.CreateScript(name, ver, desc, author, enc, uncatID)
	if err != nil {
		id, _ := client.SendMessage(chatID, "保存脚本失败："+err.Error(), "", nil)
		s.trackBot(chatID, id)
		return
	}
	s.db.Audit("tg_script_add", "script="+name, "telegram")
	s.wipeMsgs(client, chatID)
	s.askCategory(client, chatID, "script", []int64{id}, "脚本《"+name+"》")
}
