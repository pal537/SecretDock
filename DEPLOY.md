# SecretDock 安装说明

## 一、环境要求

- Docker 20.10+ 和 Docker Compose v2
- 1核 1G 内存即可运行
- 如需公网 HTTPS 访问：一个域名 + nginx

## 二、快速安装（Docker）

```bash
# 1. 克隆代码
git clone https://github.com/<you>/secretdock.git
cd secretdock

# 2. 启动（首次启动约 1-2 分钟编译）
docker compose up -d --build

# 3. 查看日志确认启动成功
docker compose logs -f
# 看到 "listening on :8080" 即成功
```

打开浏览器访问 `http://服务器IP:8080`，首次访问显示**初始设置页**，创建管理员账号：

- 账号：任意用户名
- 密码：必须 **>10 位**，且包含 **大写字母 + 小写字母 + 数字**
- 只能创建一个账号，创建后此页面永久消失

## 三、生产环境（HTTPS）

### 1. nginx 反代配置

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # WebSocket 超时（SSH 终端需要）
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

> **注意**：WebSocket 转发头（Upgrade/Connection）必须配置，否则 SSH 终端无法连接。

### 2. Let's Encrypt 证书

```bash
certbot --nginx -d your-domain.com
```

### 3. 启用安全 Cookie（可选）

在 `.env` 中添加：

```
SECRETDOCK_SECURE_COOKIES=1
```

然后重启：`docker compose up -d`

## 四、Telegram 机器人配置

1. 找 @BotFather 创建机器人，拿到 Token
2. 登录 SecretDock → 设置 → TG 机器人
3. 填入 Bot Token，设置管理员 Chat ID（给机器人发消息，用 @userinfobot 获取）
4. 设置查询密码（TG 查询机密时用）

## 五、数据备份

### 手动备份
设置 → 备份 → 导出备份（需设置备份密码）

### 定时备份
设置 → 备份 → 开启定时备份，每天凌晨自动备份到服务器本地

### 重要文件
务必备份以下文件（丢失 = 数据永久丢失）：
- `<dataDir>/.master_key` — 主加密密钥
- `<dataDir>/secretdock.db` — 数据库

## 六、升级

```bash
cd secretdock
git pull
docker compose up -d --build
```

数据库自动迁移，无需手动操作。

## 七、常见问题

**Q: 忘记管理员密码？**
A: 目前只能删除 `users` 表重建（会清空所有数据，请先备份）：
```bash
docker exec -it secretdock sqlite3 /data/secretdock.db "DELETE FROM users;"
# 重启后重新走初始设置
```

**Q: 开了登录安全后把自己锁在外面了？**
A: 保存时有防自锁检查，一般锁不住。万一还是进不去（比如换了国家后 IP 归属地识别漂移），在服务器上直接关掉：
```bash
docker exec -it secretdock sqlite3 /data/secretdock.db "UPDATE settings SET value='0' WHERE key='sec_enabled';"
# 然后重新登录，去 设置 → 登录安全 重新配置
```

**Q: 2FA 手机丢了 / 验证器没了？**
A: 用开通时抄下来的恢复码登录（登录页点"改用恢复码"）。恢复码也没了：在服务器上关闭 2FA
（会清空已信任 IP，下次登录重新验证）：
```bash
docker exec -it secretdock sqlite3 /data/secretdock.db "UPDATE settings SET value='' WHERE key IN ('sec_totp_enabled','sec_totp_secret_enc','sec_trusted_ips');"
```

**Q: 在 TG 里点"一键封禁"把自己的 IP 封了？**
A: 网站后台进不去了，用服务器命令行解封（容器里自带命令，不需要 sqlite3）：
```bash
docker exec secretdock /secretdock bans              # 查看封禁名单
docker exec secretdock /secretdock unban 1.2.3.4     # 解封指定 IP
```

## 八、登录安全（IP 策略 + 2FA）

设置 → 登录安全，默认关闭，手动开启。

- **IP 访问策略**（按顺序）：内网 IP 始终放行 → 黑名单拒绝 → 白名单放行 → 国家/地区 allowlist。白名单/黑名单支持 CIDR（`1.2.3.0/24`），IPv4/IPv6 均可。
- **国家/地区**：下拉多选+搜索，常用置顶（中国/香港/美国/新加坡/日本/韩国/德国）。IP 库是免费的 DB-IP，每月在设置页点"更新 IP 库"即可。
- **2FA**：TOTP 6 位动态码（Google/Microsoft Authenticator 等）。开通时扫码+抄恢复码，必须输对一次动态码才算开通成功。**只有新 IP 需要验证**，同一 IP 验证过一次就记住；换 IP 会自动掉线并要求重新验证。
- 注意：nginx 反代必须透传 `X-Forwarded-For`（本说明第三节的配置已包含），否则服务器看到的 IP 全是 127.0.0.1，国家策略会失效。

**Q: SSH 终端连不上？**
A: 检查 nginx 是否配置了 WebSocket 转发头（见第三节）。

**Q: 端口冲突？**
A: 在 `.env` 中设置 `HOST_PORT=18090` 改映射端口。
