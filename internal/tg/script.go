package tg

import "strings"

// ScriptMeta holds the parsed Tampermonkey metadata block.
type ScriptMeta struct {
	Name        string
	Namespace   string
	Version     string
	Description string
	Author      string
	Matches     []string
}

// ParseUserScript extracts the metadata block from a pasted userscript.
// Returns false when the text is not a userscript.
func ParseUserScript(text string) (ScriptMeta, bool) {
	var meta ScriptMeta
	start := strings.Index(text, "==UserScript==")
	end := strings.Index(text, "==/UserScript==")
	if start < 0 || end < 0 || end <= start {
		return meta, false
	}
	block := text[start:end]
	found := false
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		// strip leading // markers
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "@") {
			continue
		}
		kv := strings.SplitN(line[1:], " ", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		switch strings.ToLower(key) {
		case "name":
			meta.Name = val
			found = true
		case "namespace":
			meta.Namespace = val
		case "version":
			meta.Version = val
		case "description":
			meta.Description = val
		case "author":
			meta.Author = val
		case "match", "include":
			meta.Matches = append(meta.Matches, val)
		}
	}
	if !found {
		return meta, false
	}
	return meta, true
}

// DetectedScript is the result of script name detection.
type DetectedScript struct {
	Name    string // "" when no name could be determined
	Content string // script body (name line stripped when it was given inline)
}

// DetectScriptName tries to find a script name in the payload:
//  1. userscript header (@name)
//  2. proxy-script metadata (#!name=) / filter-list header (! Title:)
//  3. first line when it looks like a plain name (e.g. "VPS配置查看")
func DetectScriptName(payload string) DetectedScript {
	if meta, ok := ParseUserScript(payload); ok {
		return DetectedScript{Name: meta.Name, Content: payload}
	}
	for _, line := range strings.Split(payload, "\n") {
		t := strings.TrimSpace(line)
		low := strings.ToLower(t)
		if strings.HasPrefix(low, "#!name=") {
			if v := strings.TrimSpace(t[len("#!name="):]); v != "" {
				return DetectedScript{Name: v, Content: payload}
			}
		}
		if strings.HasPrefix(low, "! title:") {
			if v := strings.TrimSpace(t[len("! Title:"):]); v != "" {
				return DetectedScript{Name: v, Content: payload}
			}
		}
	}
	lines := strings.SplitN(payload, "\n", 2)
	if looksLikeName(lines[0]) {
		content := ""
		if len(lines) == 2 {
			content = strings.TrimSpace(lines[1])
		}
		return DetectedScript{Name: strings.TrimSpace(lines[0]), Content: content}
	}
	return DetectedScript{Content: payload}
}

// looksLikeName reports whether s looks like a human-given script name
// rather than code.
func looksLikeName(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len([]rune(s)) > 40 {
		return false
	}
	if strings.Contains(s, "://") {
		return false
	}
	for _, p := range []string{"#!", "#", "//", "$", "`"} {
		if strings.HasPrefix(s, p) {
			return false
		}
	}
	if strings.ContainsAny(s, "(){};=$|`><&") {
		return false
	}
	return true
}
