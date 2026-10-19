package tg

import "testing"

func TestSplitLinkLine(t *testing.T) {
	cases := []struct {
		in, wantName, wantURL string
	}{
		{"爱锋助手源 https://yuan.afzs.store/appstore", "爱锋助手源", "https://yuan.afzs.store/appstore"},
		{"全民源https://y.iosrj.cn/appstore", "全民源", "https://y.iosrj.cn/appstore"},
		{"一品软件源：http://sign.io31.top/appstore", "一品软件源", "http://sign.io31.top/appstore"},
		{"https://example.com", "", "https://example.com"},
		{"纯文本无链接", "", ""},
	}
	for _, c := range cases {
		name, url, ok := splitLinkLine(c.in)
		if c.wantURL == "" {
			if ok {
				t.Errorf("%q should not parse", c.in)
			}
			continue
		}
		if !ok || name != c.wantName || url != c.wantURL {
			t.Errorf("%q => name=%q url=%q", c.in, name, url)
		}
	}
}

func TestSplitBareDomain(t *testing.T) {
	name, url, ok := splitBareDomain("晨尘源 yuan.ioschen.cn/appstore")
	if !ok || name != "晨尘源" || url != "https://yuan.ioschen.cn/appstore" {
		t.Errorf("got %q %q %v", name, url, ok)
	}
	if _, _, ok := splitBareDomain("我的分类"); ok {
		t.Error("category should not match")
	}
	if _, _, ok := splitBareDomain("v2.0"); ok {
		t.Error("version should not match")
	}
}
