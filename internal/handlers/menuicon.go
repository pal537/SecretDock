package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// POST /api/menu-icon {id, url} — fetch the site's favicon ONCE, store it under
// DataDir/menu-icons/, and stamp the link's icon field (unix ts) in the
// menu_links setting. The sidebar then serves the local copy; the origin site
// is never contacted again on page loads.
func (s *Server) handleMenuIconFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := readJSON(r, &req); err != nil || !strings.HasPrefix(req.ID, "lnk-") || req.URL == "" {
		fail(w, 400, "id and url required")
		return
	}
	data, ext, err := fetchFavicon(req.URL)
	if err != nil {
		fail(w, 502, "icon fetch failed")
		return
	}
	dir := filepath.Join(s.DataDir, "menu-icons")
	_ = os.MkdirAll(dir, 0o700)
	if old, _ := filepath.Glob(filepath.Join(dir, req.ID+".*")); old != nil {
		for _, f := range old {
			_ = os.Remove(f)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, req.ID+ext), data, 0o600); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ts := time.Now().Unix()
	s.stampMenuLinkIcon(req.ID, ts)
	ok(w, map[string]interface{}{"icon": ts})
}

func (s *Server) stampMenuLinkIcon(id string, ts int64) {
	raw := s.DB.GetSetting("menu_links")
	var links []map[string]interface{}
	if json.Unmarshal([]byte(raw), &links) != nil {
		return
	}
	changed := false
	for _, l := range links {
		if l["id"] == id {
			l["icon"] = ts
			changed = true
		}
	}
	if !changed {
		return
	}
	if b, err := json.Marshal(links); err == nil {
		_ = s.DB.SetSetting("menu_links", string(b))
	}
}

// GET /menu-icons/<id> — serve the stored favicon (whatever extension is on disk).
func (s *Server) handleMenuIconFile(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/menu-icons/")
	if !strings.HasPrefix(id, "lnk-") || strings.Contains(id, "/") || strings.Contains(id, "..") {
		http.NotFound(w, r)
		return
	}
	matches, _ := filepath.Glob(filepath.Join(s.DataDir, "menu-icons", id+".*"))
	if len(matches) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=604800")
	http.ServeFile(w, r, matches[0])
}

var (
	iconLinkRe = regexp.MustCompile(`(?i)<link[^>]*>`)
	hrefRe     = regexp.MustCompile(`(?i)href\s*=\s*"([^"]+)"|href\s*=\s*'([^']+)'`)
	relRe      = regexp.MustCompile(`(?i)rel\s*=\s*"([^"]+)"|rel\s*=\s*'([^']+)'`)
	sizesRe    = regexp.MustCompile(`(?i)sizes\s*=\s*"(\d+)x\d+"`)
)

func firstGroup(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

func fetchFavicon(pageURL string) ([]byte, string, error) {
	u, err := url.Parse(pageURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, "", fmt.Errorf("bad url")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	get := func(raw string) ([]byte, *url.URL, error) {
		req, err := http.NewRequest("GET", raw, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, nil, fmt.Errorf("status %d", resp.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 600<<10))
		return b, resp.Request.URL, err
	}

	var candidates []string
	if html, finalURL, err := get(pageURL); err == nil && finalURL != nil {
		type cand struct {
			href  string
			score int
		}
		var cs []cand
		for _, tag := range iconLinkRe.FindAllString(string(html), -1) {
			rel := strings.ToLower(firstGroup(relRe, tag))
			if !strings.Contains(rel, "icon") {
				continue
			}
			href := firstGroup(hrefRe, tag)
			if href == "" {
				continue
			}
			score := 0
			if strings.Contains(rel, "apple-touch") {
				score += 1000
			}
			if m := sizesRe.FindStringSubmatch(tag); m != nil {
				var n int
				fmt.Sscanf(m[1], "%d", &n)
				score += n
			}
			cs = append(cs, cand{href, score})
		}
		sort.Slice(cs, func(i, j int) bool { return cs[i].score > cs[j].score })
		for _, c := range cs {
			if ref, err := url.Parse(c.href); err == nil {
				candidates = append(candidates, finalURL.ResolveReference(ref).String())
			}
		}
	}
	candidates = append(candidates, u.Scheme+"://"+u.Host+"/favicon.ico")
	for _, c := range candidates {
		data, _, err := get(c)
		if err != nil || len(data) == 0 || len(data) > 300<<10 {
			continue
		}
		if ext := sniffIconExt(data); ext != "" {
			return data, ext, nil
		}
	}
	return nil, "", fmt.Errorf("no icon found")
}

func sniffIconExt(b []byte) string {
	trimmed := strings.TrimSpace(string(b))
	switch {
	case len(b) >= 4 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G':
		return ".png"
	case len(b) >= 4 && b[0] == 0 && b[1] == 0 && b[2] == 1 && b[3] == 0:
		return ".ico"
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return ".jpg"
	case len(b) >= 4 && string(b[:4]) == "GIF8":
		return ".gif"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return ".webp"
	case strings.HasPrefix(trimmed, "<svg") || strings.HasPrefix(trimmed, "<?xml"):
		return ".svg"
	}
	return ""
}
