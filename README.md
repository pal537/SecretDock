# SecretDock

**Your personal secret vault — with a Telegram bot as its second brain.**

**你的个人机密库 —— 配了一个 Telegram 机器人当第二大脑。**

---

## English

SecretDock is a self-hosted secret manager for developers and power users. The fastest way to save anything is through Telegram — just send it to the bot. Manage your servers via SSH right from the browser. Everything is encrypted, and everything stays on your server.

### ✨ Features

**🔐 Secrets Vault**
- Two types: account credentials & API secrets
- Auto-grouping by service name, with logo recognition
- Expiry reminders, pin to top, smart search

**📜 Scripts** — Store userscripts, Surge/Loon rules, with syntax highlighting and one-click copy

**🔗 Links** — Bookmark manager with auto title fetching and link preview

**📝 Notes** — Markdown notes with live preview, toolbar, and categories

**🖼️ Images / 📁 Files** — Upload, organize, and share via secure expiring links (`/s/xxxx`)

**🖥️ SSH Manager** — Connect to your servers right from the browser (web terminal + SFTP file browser)

**📡 Node Subscriptions** — Parse and test proxy subscription links (vless/vmess/trojan/ss/hysteria/tuic/wireguard…)

**⏰ Reminders** — Schedule recurring notifications via Telegram

**🤖 Telegram Bot** — The killer feature:
- **Add** secrets/scripts/links/notes/images/files by simply sending them to the bot
- **Query** secrets: send a service name, verify with your query password, get credentials instantly (auto-deletes after N minutes)
- **Security alerts**: login notifications with one-click IP ban

### 🔒 Security

- **AES-256-GCM** encryption for all sensitive data, with a server-side master key
- **Argon2id** password hashing (OWASP recommended)
- **Login Security** (optional): country allowlist, IP whitelist/blacklist, TOTP 2FA
- **Session IP check**: sessions are invalidated if the client IP changes (defense in depth)
- **Brute-force protection**: progressive lockout (3 → 5 → 10 min → … → permanent)
- **TG query protection**: separate query password, auto-ban on too many failures
- **No tracking, no telemetry, no cloud** — your data never leaves your server

### 🚀 Quick Start (Docker)

**Option A: Prebuilt image (fastest)**

```bash
docker run -d --name secretdock \
  -p 127.0.0.1:8080:8080 \
  -v ./data:/data \
  --restart unless-stopped \
  pal537/secretdock:latest
```

**Option B: Build from source**

```bash
git clone https://github.com/pal537/SecretDock.git
cd SecretDock
docker compose up -d --build
```

Open `http://localhost:8080` — you'll see the setup page. Create your admin account (one time only), and you're in.

All data lives in `./data/` (database, images, files, and the master key). Back up this directory for disaster recovery — but store the backup securely, as it contains the decryption key. For off-site backups, use the encrypted backup feature in Settings (password-protected).

For HTTPS in production, put nginx or Caddy in front. See [DEPLOY.md](DEPLOY.md) for details.

---

## 中文

SecretDock 是一个自托管的机密管理器，为开发者和重度用户打造。最方便的是随手录入：直接把内容发给 Telegram 机器人就入库了。在浏览器里就能 SSH 管理服务器。所有数据加密存储，不出你的服务器。

### ✨ 功能

**🔐 机密库**
- 两种类型：账号密码 & API 密钥
- 按服务商自动分组，LOGO 自动识别
- 过期提醒、置顶、智能搜索

**📜 脚本** — 存储油猴脚本、Surge/Loon 规则，语法高亮，一键复制

**🔗 网址** — 书签管理，自动抓取标题，链接预览

**📝 笔记** — Markdown 笔记，实时预览，工具栏，分类管理

**🖼️ 图片 / 📁 文件** — 上传整理，通过安全分享链接分享（`/s/xxxx`）

**🖥️ SSH 管理** — 直接在浏览器里连服务器（网页终端 + SFTP 文件管理）

**📡 订阅节点** — 解析测试代理订阅链接（vless/vmess/trojan/ss/hysteria/tuic/wireguard…）

**⏰ 提醒** — 定时 Telegram 推送提醒

**🤖 Telegram 机器人** — 核心功能：
- **录入**：直接把内容发给机器人，自动识别分类入库
- **查询**：发服务商名，验证查询密码，秒回凭证（N 分钟后自动删除）
- **安全提醒**：登录通知，TG 内一键封禁可疑 IP

### 🔒 安全性

- **AES-256-GCM** 加密所有敏感数据，服务器端主密钥管理
- **Argon2id** 密码哈希（OWASP 推荐）
- **登录安全**（可选）：国家白名单、IP 黑白名单、TOTP 双因素认证
- **会话 IP 检查**：客户端 IP 变化时会话失效（纵深防御）
- **防暴力破解**：渐进式锁定（3次→5分钟→…→永久）
- **TG 查询保护**：独立查询密码，错误过多自动封禁
- **无追踪、无遥测、无云端** —— 数据不出你的服务器

### 🚀 快速开始（Docker）

```bash
git clone https://github.com/pal537/SecretDock.git
cd SecretDock
docker compose up -d --build
```

打开 `http://localhost:8080` —— 看到初始设置页，创建一个管理员账号（仅一次），即可使用。

所有数据都在 `./data/` 目录下（数据库、图片、文件、主密钥）。备份这个目录可做灾难恢复 —— 但备份文件务必妥善保管，因为它包含解密钥匙。异地备份请用设置里的加密备份功能（密码保护）。

生产环境请用 nginx 或 Caddy 做 HTTPS 反代，详见 [DEPLOY.md](DEPLOY.md)。

---

## 📄 License

MIT
