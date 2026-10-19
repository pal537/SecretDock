package tg

import "testing"

const sampleScript = `// ==UserScript==
// @name         网盘去密码验证（三站合一）
// @namespace    https://pan.iosxn.cn/
// @version      2.0
// @description  跳过密码拦截，直接访问
// @author       Muse
// @match        https://pan.iosxn.cn/*
// @grant        none
// ==/UserScript==

(function () { 'use strict'; })();`

func TestParseUserScript(t *testing.T) {
	meta, ok := ParseUserScript(sampleScript)
	if !ok {
		t.Fatal("should detect userscript")
	}
	if meta.Name != "网盘去密码验证（三站合一）" {
		t.Errorf("name = %q", meta.Name)
	}
	if meta.Version != "2.0" {
		t.Errorf("version = %q", meta.Version)
	}
	if meta.Author != "Muse" {
		t.Errorf("author = %q", meta.Author)
	}
	if len(meta.Matches) != 1 {
		t.Errorf("matches = %v", meta.Matches)
	}
}

func TestParseUserScriptNegative(t *testing.T) {
	if _, ok := ParseUserScript("cloudflare\nAPI Token: xxx"); ok {
		t.Error("plain text should not be a userscript")
	}
	if _, ok := ParseUserScript("// ==UserScript==\n// @foo bar"); ok {
		t.Error("block without @name should not count")
	}
}

func TestDetectScriptName(t *testing.T) {
	// userscript: @name wins
	d := DetectScriptName(sampleScript)
	if d.Name != "网盘去密码验证（三站合一）" || d.Content != sampleScript {
		t.Errorf("userscript: %+v", d)
	}
	// proxy script: #!name=
	d = DetectScriptName("#!name=Cookie获取\n#!desc=获取cookie\n[rewrite]\n...")
	if d.Name != "Cookie获取" {
		t.Errorf("proxy: %+v", d)
	}
	// filter list: ! Title:
	d = DetectScriptName("! Title: 去广告规则\n! Description: xxx\n||ads.com^")
	if d.Name != "去广告规则" {
		t.Errorf("filter: %+v", d)
	}
	// inline name line + bash one-liner (PAPA's case)
	payload := "VPS配置查看\nbash <(wget -qO- --no-check-certificate https://gitlab.com/spiritysdx/za/-/raw/main/ecs.sh)"
	d = DetectScriptName(payload)
	if d.Name != "VPS配置查看" {
		t.Errorf("inline name: %+v", d)
	}
	if d.Content != "bash <(wget -qO- --no-check-certificate https://gitlab.com/spiritysdx/za/-/raw/main/ecs.sh)" {
		t.Errorf("inline content: %q", d.Content)
	}
	// bare code, no name
	d = DetectScriptName("bash <(wget -qO- https://example.com/x.sh)")
	if d.Name != "" {
		t.Errorf("bare code should have no name: %+v", d)
	}
}
