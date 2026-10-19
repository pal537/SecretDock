// Merchant logo resolution: Simple Icons CDN -> favicon service ->
// letter avatar fallback (rendered by the frontend).
package tg

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/example/secretdock/internal/db"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns a merchant name into a URL-safe slug.
func slugify(name string) string {
	s := slugRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "")
	return s
}

// domainMap maps well-known merchant names to their domains for favicons.
var domainMap = map[string]string{
	"cloudflare": "cloudflare.com",
	"backblaze":  "backblaze.com",
	"github":     "github.com",
	"aliyun":     "aliyun.com",
	"阿里云":        "aliyun.com",
	"腾讯云":        "cloud.tencent.com",
	"qiniu":      "qiniu.com",
	"七牛云":        "qiniu.com",
	"minio":      "min.io",
	"aws":        "aws.amazon.com",
	"amazon":     "aws.amazon.com",
	"vercel":     "vercel.com",
	"docker":     "docker.com",
	"vultr":      "vultr.com",
	"digitalocean": "digitalocean.com",
	"linode":     "linode.com",
	"hetzner":    "hetzner.com",
	"telegram":   "telegram.org",
	"openai":     "openai.com",
	"anthropic":  "anthropic.com",
	"google":     "google.com",
	"microsoft":  "microsoft.com",
	"cloudcone":  "cloudcone.com",
	"bandwagonhost": "bandwagonhost.com",
	"搬瓦工":         "bandwagonhost.com",
	"porkbun":    "porkbun.com",
	"namecheap":  "namecheap.com",
	"namesilo":   "namesilo.com",
	"dnspod":     "dnspod.com",
	"alibaba":    "alibaba.com",
	"tencent":    "cloud.tencent.com",
	"huawei":     "huaweicloud.com",
	"华为云":        "huaweicloud.com",
	"ucloud":     "ucloud.cn",
	"volcengine": "volcengine.com",
	"火山引擎":       "volcengine.com",
	"baidu":      "baidu.com",
	"百度云":        "cloud.baidu.com",
	"oracle":     "oracle.com",
	"甲骨文":        "oracle.com",
	"stripe":     "stripe.com",
	"paypal":     "paypal.com",
	"notion":     "notion.so",
	"slack":      "slack.com",
	"discord":    "discord.com",
	"zoom":       "zoom.us",
	"dropbox":    "dropbox.com",
	// ---- 更多国外 ----
	"b2":         "backblaze.com",
	"r2":         "cloudflare.com",
	"s3":         "aws.amazon.com",
	"gitlab":     "gitlab.com",
	"bitbucket":  "bitbucket.org",
	"heroku":     "heroku.com",
	"netlify":    "netlify.com",
	"render":     "render.com",
	"railway":    "railway.app",
	"flyio":      "fly.io",
	"fly":        "fly.io",
	"supabase":   "supabase.com",
	"neon":       "neon.tech",
	"planetscale": "planetscale.com",
	"twilio":     "twilio.com",
	"sendgrid":   "sendgrid.com",
	"mailgun":    "mailgun.com",
	"resend":     "resend.com",
	"npm":        "npmjs.com",
	"pypi":       "pypi.org",
	"gcp":        "cloud.google.com",
	"azure":      "azure.microsoft.com",
	"oraclecloud": "oracle.com",
	"scaleway":   "scaleway.com",
	"ovh":        "ovh.com",
	"contabo":    "contabo.com",
	// ---- 更多国内 ----
	"upyun":      "upyun.com",
	"又拍子":         "upyun.com",
	"jdcloud":    "jdcloud.com",
	"京东云":         "jdcloud.com",
	"ctyun":      "ctyun.cn",
	"天翼云":         "ctyun.cn",
	"ecloud":     "ecloud.10086.cn",
	"移动云":         "ecloud.10086.cn",
	"unicom":     "cloud.unicom.cn",
	"联通云":         "cloud.unicom.cn",
	"ksyun":      "ksyun.com",
	"金山云":         "ksyun.com",
	"qingcloud":  "qingcloud.com",
	"青云":          "qingcloud.com",
	"七牛":          "qiniu.com",
	"阿里":          "aliyun.com",
	"tencentcloud": "cloud.tencent.com",
	"百度":          "cloud.baidu.com",
	"bytedance":  "volcengine.com",
	"字节":          "volcengine.com",
	"dingtalk":   "dingtalk.com",
	"钉钉":          "dingtalk.com",
	"wechat":     "wechat.com",
	"微信":          "wechat.com",
	"alipay":     "alipay.com",
	"支付宝":         "alipay.com",
}

// logoAlias maps short names/abbreviations to canonical provider names
// so logo lookup works for inputs like "cf".
var logoAlias = map[string]string{
	"cf":  "cloudflare",
	"b2":  "backblaze",
	"r2":  "cloudflare",
	"s3":  "aws",
	"gcp": "google",
}

// LogoCandidate is one possible logo for a merchant.
type LogoCandidate struct {
	Kind string `json:"kind"` // "simpleicons" | "favicon"
	URL  string `json:"url"`
}

// fuzzyKeys 从名称中提取可能的 logo 键（模糊匹配）
// 处理：backblazeB2 -> backblaze, cloudflare123 -> cloudflare, BackBlaze -> backblaze
func fuzzyKeys(name string) []string {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return nil
	}
	var keys []string
	seen := map[string]bool{}
	add := func(k string) {
		k = strings.TrimSpace(k)
		if k != "" && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	add(lower)
	// 别名
	if a, ok := logoAlias[lower]; ok {
		add(a)
	}
	// 去尾部数字：cloudflare123 -> cloudflare, backblazeB2 -> backblazeB
	trailingNum := regexp.MustCompile(`^(.*?)[0-9]+$`).FindStringSubmatch(lower)
	if trailingNum != nil {
		add(trailingNum[1])
		if a, ok := logoAlias[trailingNum[1]]; ok {
			add(a)
		}
	}
	// 驼峰/大小写分隔：backblazeB2 -> backblaze（按大写字母切分原名）
	var parts []string
	var cur strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	for _, p := range parts {
		pl := strings.ToLower(strings.TrimSpace(p))
		// 去掉纯数字部分
		if pl != "" && !regexp.MustCompile(`^[0-9]+$`).MatchString(pl) {
			add(pl)
			// 再去一次尾部数字（如 B2 -> B）
			if m := regexp.MustCompile(`^(.*?)[0-9]+$`).FindStringSubmatch(pl); m != nil {
				add(m[1])
			}
		}
	}
	// 空格分隔的 token
	for _, tok := range strings.Fields(lower) {
		add(tok)
	}
	// 最长前缀匹配：找 domainMap 中是输入前缀的最长键
	best := ""
	for k := range domainMap {
		if strings.HasPrefix(lower, k) && len(k) > len(best) {
			best = k
		}
	}
	if best != "" {
		add(best)
	}
	return keys
}

// LogoCandidates returns ordered logo candidates for a merchant name.
func LogoCandidates(name string) []LogoCandidate {
	var out []LogoCandidate
	seen := map[string]bool{}
	addSimple := func(slug string) {
		if slug == "" || seen["si:"+slug] {
			return
		}
		seen["si:"+slug] = true
		out = append(out, LogoCandidate{Kind: "simpleicons",
			URL: "https://cdn.jsdelivr.net/npm/simple-icons/icons/" + slug + ".svg"})
	}
	addFavicon := func(domain string) {
		if domain == "" || seen["fv:"+domain] {
			return
		}
		seen["fv:"+domain] = true
		out = append(out, LogoCandidate{Kind: "favicon",
			URL: "https://www.google.com/s2/favicons?domain=" + domain + "&sz=64"})
	}
	for _, key := range fuzzyKeys(name) {
		// Simple Icons
		if slug := slugify(key); slug != "" {
			addSimple(slug)
		}
		// favicon via domainMap
		if domain := domainMap[key]; domain != "" {
			addFavicon(domain)
		}
	}
	// last resort: slug.com
	if slug := slugify(name); slug != "" {
		addFavicon(slug + ".com")
	}
	return out
}

var httpHead = &http.Client{Timeout: 8 * time.Second}

func urlOK(url string) bool {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false
	}
	resp, err := httpHead.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// autoLogo picks the first working logo candidate and saves it on the merchant.
// Best-effort: failures are silently ignored.
func autoLogo(database *db.DB, merchantID int64, name string) {
	m, err := database.GetMerchant(merchantID)
	if err != nil {
		return
	}
	for _, c := range LogoCandidates(name) {
		if urlOK(c.URL) {
			_ = database.UpdateMerchant(merchantID, m.Name, m.Aliases, c.URL, "")
			return
		}
	}
}
