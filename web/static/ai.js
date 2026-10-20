/* SecretDock AI module — deep-chat as the chat engine, SecretDock shell around it. */
'use strict';

/* ---------- state ---------- */
const aiState = {
  presets: [], configs: [], threads: [], meta: null,
  cur: 0, messages: [], branchPick: {}, op: null,
  abort: null, streaming: false, filter: '',
};
const AI_LOGO = p => '/static/ai-logos/' + p.logo + '?v=__APPV__';

/* ---------- helpers ---------- */
function aiPreset(id) { return aiState.presets.find(p => p.id === id) || null; }
function aiCfg(id) { return aiState.configs.find(c => c.id === id) || null; }
function aiCfgLogo(cfg, size) {
  const s = size || 22;
  const p = cfg && aiPreset(cfg.providerId);
  if (p && p.logo) return `<img class="ai-logo" style="width:${s}px;height:${s}px" src="${AI_LOGO(p)}" alt="">`;
  const name = (cfg && cfg.name) || '?';
  return `<span class="ai-logo ai-logo-letter" style="width:${s}px;height:${s}px;font-size:${Math.round(s * 0.55)}px;background:${avatarColor(name)};color:${avatarFg(name)}">${esc(name[0] || '?')}</span>`;
}
function aiFmtBytes(n) {
  n = +n || 0;
  if (n < 1024) return n + ' B';
  if (n < 1048576) return (n / 1024).toFixed(1) + ' KB';
  if (n < 1073741824) return (n / 1048576).toFixed(1) + ' MB';
  return (n / 1073741824).toFixed(2) + ' GB';
}
function aiFmtNum(n) { return (+n || 0).toLocaleString('en-US'); }
function aiChain() {
  const byParent = {};
  for (const m of aiState.messages) (byParent[m.parentId] = byParent[m.parentId] || []).push(m);
  for (const k in byParent) byParent[k].sort((a, b) => a.id - b.id);
  const chain = [];
  let pid = 0;
  for (;;) {
    const kids = byParent[pid];
    if (!kids || !kids.length) break;
    const pick = aiState.branchPick[pid];
    const m = kids.find(k => k.id === pick) || kids[kids.length - 1];
    chain.push(m);
    pid = m.id;
  }
  return chain;
}
function aiSiblings(m) { return aiState.messages.filter(x => x.parentId === m.parentId).sort((a, b) => a.id - b.id); }
function aiLeafId() { const c = aiChain(); return c.length ? c[c.length - 1].id : 0; }
function aiCurThread() { return aiState.threads.find(t => t.id === aiState.cur) || null; }
function aiReasonBlock(reasoning) {
  if (!reasoning) return '';
  return `<details class="ai-reason"><summary>思考过程</summary><div class="ai-reason-body">${esc(reasoning)}</div></details>\n\n`;
}
function aiUsageLine(u) {
  if (!u) return '';
  let o = null;
  try { o = typeof u === 'string' ? JSON.parse(u) : u; } catch (e) { return ''; }
  if (!o || (!o.in && !o.out)) return '';
  let s = `输入 ${aiFmtNum(o.in)} · 输出 ${aiFmtNum(o.out)}`;
  if (o.cacheHit) s += ` · 缓存命中 ${aiFmtNum(o.cacheHit)}`;
  if (o.cost) s += ` · ¥${o.cost.toFixed(4)}`;
  return `<div class="ai-usage">${s}</div>`;
}
function aiBranchNav(m) {
  const sibs = aiSiblings(m);
  if (sibs.length < 2) return '';
  const idx = sibs.findIndex(x => x.id === m.id);
  return `<span class="ai-branch"><button data-ai-act="bprev" data-pid="${m.parentId}">‹</button><span>${idx + 1}/${sibs.length}</span><button data-ai-act="bnext" data-pid="${m.parentId}">›</button></span>`;
}
function aiMsgFoot(m) {
  // User asked to drop the edit/fork buttons (2026-10-20): AI messages keep
  // regenerate + copy; user messages show only the branch switcher when the
  // message has multiple versions.
  const acts = m.role === 'ai'
    ? `<button data-ai-act="regen" data-id="${m.id}">重新生成</button><button data-ai-act="copy" data-id="${m.id}">复制</button>`
    : '';
  const nav = aiBranchNav(m);
  if (!nav && !acts) return '';
  return `<div class="ai-msgfoot">${nav}${acts}</div>`;
}
function aiToDCMessage(m) {
  if (m.role === 'user') {
    let text = m.content || '';
    let atts = [];
    try { atts = JSON.parse(m.attachments || '[]'); } catch (e) {}
    const files = [];
    const notes = [];
    for (const a of atts) {
      if (a.kind === 'image') files.push({ name: a.name, type: 'image', src: '/api/ai/files/' + a.id });
      else if (a.kind === 'audio') files.push({ name: a.name, type: 'audio', src: '/api/ai/files/' + a.id });
      else notes.push(a.name);
    }
    if (notes.length) text += `\n\n[附件：${notes.join('、')}]`;
    const out = { role: 'user', text };
    if (files.length) out.files = files;
    out.text += '\n\n' + aiMsgFoot(m);
    return out;
  }
  let text = aiReasonBlock(m.reasoning) + (m.content || '');
  let uo = null;
  try { uo = JSON.parse(typeof (m.usageJson || m.usage) === 'string' ? (m.usageJson || m.usage) : JSON.stringify(m.usageJson || m.usage || {})); } catch (e) {}
  let atts = [];
  try { atts = JSON.parse(m.attachments || '[]'); } catch (e) {}
  const files = [];
  for (const a of atts) {
    if (a.kind === 'image') files.push({ name: a.name, type: 'image', src: '/api/ai/files/' + a.id });
    else if (a.kind === 'video') text += `\n\n<video src="/api/ai/files/${a.id}" controls preload="metadata" style="max-width:100%;border-radius:12px"></video>`;
    else if (a.kind === 'audio') files.push({ name: a.name, type: 'audio', src: '/api/ai/files/' + a.id });
  }
  if (uo && uo.video && uo.video.status === 'pending') text += '视频生成中，好了会自动出现在这里，一般要几分钟。';
  if (uo && uo.error) text += `\n\n> 出错了：${uo.error}`;
  const foot = aiUsageLine(m.usageJson || m.usage) + aiMsgFoot(m);
  if (foot) text += '\n\n' + foot;
  const out = { role: 'ai', text };
  if (files.length) out.files = files;
  return out;
}
function aiSetStatus(s) {
  const el = $('#aiStatus');
  if (el) el.textContent = s || '';
}

/* ---------- view ---------- */
async function renderAI() {
  const v = $('#view');
  if (!aiState.presets.length) {
    try { aiState.presets = await (await fetch('/static/ai-providers.json?v=__APPV__')).json(); } catch (e) { aiState.presets = []; }
  }
  try {
    const [cfgs, threads, meta] = await Promise.all([
      api('GET', '/api/ai/configs'),
      api('GET', '/api/ai/threads'),
      api('GET', '/api/ai/meta'),
    ]);
    aiState.configs = cfgs; aiState.threads = threads; aiState.meta = meta;
  } catch (e) { toast('AI 加载失败：' + e.message, true); return; }
  v.innerHTML = `
  <div class="ai-wrap">
    <aside class="ai-side">
      <button class="btn ai-new" id="aiNew">${ic('plus')} 新建会话</button>
      <input class="ai-tsearch" id="aiTSearch" placeholder="搜索会话…">
      <div class="ai-threads" id="aiThreads"></div>
    </aside>
    <section class="ai-main">
      <div class="ai-head">
        <button class="ai-switch" id="aiSwitch"><span class="muted">加载中…</span></button>
        <span style="flex:1"></span>
        <button class="ai-pill" id="aiSearchPill" title="联网搜索开关">${ic('globe')} 联网</button>
        <button class="ai-pill" id="aiPromptsBtn">${ic('zap')} 提示词</button>
        <button class="ai-pill" id="aiDataBtn">${ic('archive')} 数据管理</button>
        <button class="btn" id="aiManageBtn">API 管理</button>
      </div>
      <div class="ai-chatbox" id="aiChatBox"></div>
      <div class="ai-status" id="aiStatus"></div>
    </section>
  </div>`;
  $('#aiNew').onclick = aiNewThread;
  let aiSearchTimer = null;
  $('#aiTSearch').oninput = e => {
    const q = e.target.value.trim();
    clearTimeout(aiSearchTimer);
    aiSearchTimer = setTimeout(async () => {
      aiState.filter = q;
      if (!q) { aiState.searchResults = null; aiDrawThreads(); return; }
      try {
        // Server-side search covers message content, not just titles.
        const res = await api('GET', '/api/ai/threads?q=' + encodeURIComponent(q));
        if (aiState.filter === q) { aiState.searchResults = res; aiDrawThreads(res); }
      } catch (err) { aiDrawThreads(); }
    }, 250);
  };
  $('#aiSwitch').onclick = aiOpenSwitcher;
  $('#aiSearchPill').onclick = aiToggleSearch;
  $('#aiPromptsBtn').onclick = aiOpenPrompts;
  $('#aiDataBtn').onclick = aiOpenData;
  $('#aiManageBtn').onclick = () => aiOpenManager();
  aiDrawThreads();
  if (!aiState.configs.length) {
    $('#aiChatBox').innerHTML = `<div class="empty" style="padding-top:12vh">${ic('bot')}<div>还没有配置 AI 商家</div>
      <div class="muted" style="font-size:12px;margin:6px 0 14px">先添加一个 API，才能开始对话</div>
      <button class="btn" onclick="aiOpenManager(true)">添加 API</button></div>`;
    aiDrawSwitcher();
    return;
  }
  let target = aiState.cur && aiCurThread() ? aiState.cur : (aiState.threads[0] && aiState.threads[0].id);
  if (!target) {
    await aiNewThread(true);
    return;
  }
  await aiOpenThread(target);
}

function aiDrawThreads(searchResults) {
  const box = $('#aiThreads');
  if (!box) return;
  let list;
  if (Array.isArray(searchResults)) {
    list = searchResults.filter(t => !t.archived);
  } else if (aiState.filter && Array.isArray(aiState.searchResults)) {
    // Redraws while a content search is active (e.g. after opening a
    // result) must keep showing the server results, not a local
    // title-only filter that would empty the list.
    list = aiState.searchResults.filter(t => !t.archived);
  } else {
    list = aiState.threads.filter(t => !t.archived);
    if (aiState.filter) list = list.filter(t => (t.title || '').includes(aiState.filter));
  }
  if (!list.length) {
    box.innerHTML = `<div class="muted" style="padding:14px;font-size:12px">${aiState.filter ? '没有匹配的会话' : '还没有会话'}</div>`;
    return;
  }
  box.innerHTML = list.map(t => {
    const cfg = aiCfg(t.configId);
    return `<div class="ai-thread ${t.id === aiState.cur ? 'active' : ''}" data-id="${t.id}">
      <div class="ai-thread-t">${t.pinned ? '<span class="ai-pin">' + ic('pin') + '</span>' : ''}${esc(t.title || '新会话')}</div>
      <div class="ai-thread-m">${cfg ? aiCfgLogo(cfg, 14) : ''}<span>${esc(t.model || (cfg ? cfg.defaultModel : '') || '')}</span><span style="flex:1"></span><span>${fmtDate(t.updatedAt).slice(5, 16)}</span></div>
      <button class="ai-thread-menu" data-menu="${t.id}" title="更多操作">···</button>
    </div>`;
  }).join('');
  $$('.ai-thread', box).forEach(el => {
    el.onclick = e => {
      if (e.target.closest('[data-menu]')) return;
      aiOpenThread(+el.dataset.id);
    };
  });
  $$('[data-menu]', box).forEach(b => b.onclick = e => {
    e.stopPropagation();
    aiThreadMenu(+b.dataset.menu, e.clientX, e.clientY);
  });
}

function aiThreadMenu(id, x, y) {
  const t = aiState.threads.find(z => z.id === id);
  if (!t) return;
  aiPopup(x, y, [
    { label: t.pinned ? '取消置顶' : '置顶', fn: async () => { await api('PUT', '/api/ai/threads/' + id, { pinned: !t.pinned }); await aiReloadThreads(); } },
    { label: '重命名', fn: () => aiRenameThread(t) },
    { label: '导出 Markdown', fn: () => aiExportThread(t) },
    { label: '归档', fn: async () => { await api('PUT', '/api/ai/threads/' + id, { archived: true }); if (aiState.cur === id) aiState.cur = 0; await aiReloadThreads(); if (!aiState.cur) renderAI(); } },
    { label: '删除', danger: true, fn: () => aiDeleteThread(t) },
  ]);
}
function aiRenameThread(t) {
  openModal(`<h3>重命名会话</h3>
    <div class="frow"><input id="aiRenName" value="${esc(t.title || '')}" maxlength="60"></div>
    <div class="foot"><button class="btn ghost" data-close>取消</button><button class="btn" id="aiRenSave">保存</button></div>`);
  $('#aiRenSave').onclick = async () => {
    const name = $('#aiRenName').value.trim();
    if (!name) return;
    await api('PUT', '/api/ai/threads/' + t.id, { title: name });
    closeModal();
    await aiReloadThreads();
  };
}
function aiDeleteThread(t) {
  openModal(`<h3>删除会话「${esc(t.title || '新会话')}」？</h3>
    <p class="muted">该会话的全部消息和附件都会被删除，不可恢复。</p>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn danger" id="aiDelYes">删除</button></div>`);
  $('#aiDelYes').onclick = async () => {
    await api('DELETE', '/api/ai/threads/' + t.id);
    closeModal(); toast('已删除');
    if (aiState.cur === t.id) aiState.cur = 0;
    await aiReloadThreads();
    if (!aiState.cur) renderAI();
  };
}
async function aiExportThread(t) {
  let msgs = aiState.messages, cur = aiState.cur;
  if (t.id !== cur) {
    const d = await api('GET', '/api/ai/threads/' + t.id + '/messages');
    msgs = d.messages;
  }
  const byParent = {};
  for (const m of msgs) (byParent[m.parentId] = byParent[m.parentId] || []).push(m);
  for (const k in byParent) byParent[k].sort((a, b) => a.id - b.id);
  const chain = [];
  let pid = 0;
  for (;;) { const kids = byParent[pid]; if (!kids || !kids.length) break; const m = kids[kids.length - 1]; chain.push(m); pid = m.id; }
  const cfg = aiCfg(t.configId);
  let md = `# ${t.title || '新会话'}\n\n`;
  for (const m of chain) {
    if (m.role === 'user') md += `## 我\n\n${m.content}\n\n`;
    else md += `## AI（${cfg ? cfg.name : ''} / ${t.model || ''}）\n\n${m.content}\n\n`;
  }
  const blob = new Blob([md], { type: 'text/markdown' });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = (t.title || 'chat') + '.md';
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 4000);
}

/* tiny popup menu */
function aiPopup(x, y, items) {
  aiClosePopup();
  const d = document.createElement('div');
  d.className = 'ai-popup';
  d.style.left = Math.min(x, innerWidth - 170) + 'px';
  d.style.top = Math.min(y, innerHeight - items.length * 40 - 20) + 'px';
  d.innerHTML = items.map((it, i) => `<button data-i="${i}" class="${it.danger ? 'danger-t' : ''}">${esc(it.label)}</button>`).join('');
  document.body.appendChild(d);
  $$('button', d).forEach(b => b.onclick = () => { aiClosePopup(); items[+b.dataset.i].fn(); });
  aiArmPopupClose();
}
function aiArmPopupClose() {
  setTimeout(() => document.addEventListener('click', aiOutsideClose), 0);
}
function aiOutsideClose(e) {
  if (e.target.closest && e.target.closest('.ai-popup')) return;
  aiClosePopup();
}
function aiClosePopup() {
  document.removeEventListener('click', aiOutsideClose);
  $$('.ai-popup').forEach(e => e.remove());
}
document.addEventListener('keydown', e => { if (e.key === 'Escape') aiClosePopup(); });

async function aiReloadThreads() {
  aiState.threads = await api('GET', '/api/ai/threads');
  aiDrawThreads();
}

async function aiNewThread(silent) {
  const defId = (aiState.meta && aiState.meta.defaultConfigId) || (aiState.configs[0] && aiState.configs[0].id);
  const cfg = aiCfg(defId) || aiState.configs[0];
  if (!cfg) { toast('先添加一个 API', true); return; }
  const t = await api('POST', '/api/ai/threads', { configId: cfg.id, model: cfg.defaultModel });
  aiState.threads.unshift(t);
  await aiOpenThread(t.id);
  if (!silent) toast('已新建会话');
}

async function aiOpenThread(id) {
  aiState.cur = id;
  aiState.branchPick = {};
  aiState.op = null;
  const d = await api('GET', '/api/ai/threads/' + id + '/messages');
  aiState.messages = d.messages || [];
  const t = aiCurThread();
  if (t) { const fresh = d.thread; Object.assign(t, fresh); }
  aiDrawThreads();
  aiDrawSwitcher();
  aiDrawSearchPill();
  aiMountChat();
}

function aiDrawSwitcher() {
  const btn = $('#aiSwitch');
  if (!btn) return;
  const t = aiCurThread();
  const cfg = t && aiCfg(t.configId);
  if (!cfg) { btn.innerHTML = `<span class="muted">未选择商家</span>`; return; }
  btn.innerHTML = `${aiCfgLogo(cfg, 24)}<span class="ai-switch-name">${esc(cfg.name)}</span><span class="ai-switch-model">${esc((t && t.model) || cfg.defaultModel || '未选模型')}</span><span class="ai-switch-chev">${ic('chevD')}</span>`;
}
function aiDrawSearchPill() {
  const pill = $('#aiSearchPill');
  if (!pill) return;
  const t = aiCurThread();
  pill.classList.toggle('on', !!(t && t.searchOn));
}
function aiToggleSearch() {
  const t = aiCurThread();
  if (!t) return;
  const next = !t.searchOn;
  if (next) {
    const cfg = aiCfg(t.configId);
    const hasNative = cfg && cfg.nativeSearch;
    const ready = aiState.meta && ((aiState.meta.searchBackend === 'searxng' && aiState.meta.searxngUrl) || aiState.meta.tavilyKeySet);
    if (!hasNative && !ready) {
      toast('还没配置搜索引擎，去 API 管理里填 Tavily Key 或 SearXNG 地址', true);
      aiOpenManager(false, 'search');
      return;
    }
  }
  api('PUT', '/api/ai/threads/' + t.id, { searchOn: next }).then(() => {
    t.searchOn = next; aiDrawSearchPill();
    toast(next ? '联网搜索已开' : '联网搜索已关');
  });
}

/* ---------- deep-chat mounting ---------- */
let aiChatEl = null;
function aiNoCamNote() {
  if (sessionStorage.getItem('ai-nocam-note')) return;
  sessionStorage.setItem('ai-nocam-note', '1');
  setTimeout(() => toast('未检测到摄像头，拍照入口已隐藏；可用 ⊕ 里的图片/文件代替'), 800);
}

function aiMountChat() {
  const box = $('#aiChatBox');
  if (!box) return;
  box.innerHTML = '';
  const t = aiCurThread();
  const cfg = t && aiCfg(t.configId);
  const el = document.createElement('deep-chat');
  aiChatEl = el;
  el.style.cssText = 'width:100%;height:100%;border:none;background:transparent;font-family:inherit';
  const preset = cfg && aiPreset(cfg.providerId);
  el.avatars = {
    ai: preset && preset.logo
      ? { src: AI_LOGO(preset), styles: { avatar: { width: '30px', height: '30px', borderRadius: '8px', backgroundColor: 'transparent' } } }
      : { styles: { avatar: { width: '30px', height: '30px', borderRadius: '8px' } } },
    user: { styles: { container: { display: 'none' }, avatar: { display: 'none' } } },
  };
  // no el.names on purpose: deep-chat only renders name labels when the
  // names property is set — the user doesn't want name tags on messages.
  el.messageStyles = {
    default: {
      shared: { bubble: { maxWidth: '94%', padding: '10px 14px', borderRadius: '14px', fontSize: '14px', lineHeight: '1.7', textAlign: 'left' } },
      ai: { bubble: { backgroundColor: 'var(--card)', color: 'var(--text)', border: '1px solid var(--border)', borderTopLeftRadius: '4px' } },
      user: { bubble: { backgroundColor: '#3b6fe0', color: '#fff', borderTopRightRadius: '4px' } },
    },
    loading: { bubble: { backgroundColor: 'var(--card)', border: '1px solid var(--border)' } },
    error: { bubble: { backgroundColor: 'var(--danger-soft)', color: 'var(--danger)' } },
  };
  const phThread = aiCurThread();
  const phKind = phThread ? aiKindOfModel(aiCfg(phThread.configId), phThread.model || '') : 'chat';
  const phText = phKind === 'image' ? '描述你想画的图…' : phKind === 'video' ? '描述你想生成的视频…' : '输入消息…';
  el.textInput = {
    placeholder: { text: phText },
    styles: {
      container: { borderRadius: '18px', border: '1px solid var(--border)', backgroundColor: 'var(--card)', boxShadow: '0 2px 10px rgba(30,45,90,.05)', padding: '3px 5px' },
      text: { fontSize: '14px', color: 'var(--text)' },
      focus: { border: '1px solid #3b6fe0', boxShadow: '0 0 0 3px rgba(59,111,224,.15)' },
    },
  };
  // NOTE: submit-button look is done in auxiliaryStyle CSS below — the
  // submitButtonStyles config shape silently no-ops when mis-keyed, CSS
  // against the real shadow-DOM classes (.submit-button/.loading-button/
  // .disabled-button) is deterministic.
  el.auxiliaryStyle = `
    .ai-reason { border: 1px solid var(--border); border-radius: 10px; padding: 6px 10px; margin: 2px 0 8px; background: color-mix(in srgb, var(--muted) 6%, transparent); }
    .ai-reason summary { cursor: pointer; color: var(--muted); font-size: 12px; user-select: none; }
    .ai-reason-body { white-space: pre-wrap; font-size: 13px; color: var(--muted); margin-top: 6px; line-height: 1.6; }
    .ai-usage { font-size: 11px; color: var(--muted); margin-top: 8px; }
    .ai-msgfoot { display: flex; gap: 4px; align-items: center; margin-top: 8px; flex-wrap: wrap; }
    .ai-msgfoot button { border: 1px solid var(--border); background: transparent; color: var(--muted); font-size: 11px; padding: 2px 8px; border-radius: 999px; cursor: pointer; line-height: 1.5; }
    .ai-msgfoot button:hover { color: #3b6fe0; border-color: #3b6fe0; }
    .ai-branch { display: inline-flex; align-items: center; gap: 2px; margin-right: 6px; }
    .ai-branch button { padding: 0 6px; }
    .ai-branch span { font-size: 11px; color: var(--muted); }
    pre { border-radius: 10px; }
    .input-button { border-radius: 50%; color: var(--muted); }
    .input-button:hover { background-color: color-mix(in srgb, var(--muted) 14%, transparent); color: var(--text); }
    .submit-button, .loading-button, .disabled-button { width: 34px !important; height: 34px !important; border-radius: 50% !important; display: inline-flex !important; align-items: center !important; justify-content: center !important; flex: none !important; margin-left: 2px; }
    .submit-button { background-color: #3b6fe0 !important; }
    .submit-button:hover { background-color: #2f5ec4 !important; }
    .submit-button svg { filter: brightness(0) invert(1); }
    .disabled-button { background-color: color-mix(in srgb, var(--muted) 16%, transparent) !important; }
    .disabled-button svg { opacity: .5; }
    .loading-button { background-color: #3d4454 !important; }
    .loading-button:hover { background-color: #2f3542 !important; }
    #stop-icon { background-color: #fff !important; width: 11px !important; height: 11px !important; inset-inline-start: 50% !important; inset-block-end: 50% !important; transform: translate(-50%, 50%); border-radius: 2px !important; }
  `;
  // Camera: hide the Photo entry when the device has no camera (probed
  // async; applies from the next mount) and say so once per session.
  if (aiState.hasCamera === undefined && navigator.mediaDevices && navigator.mediaDevices.enumerateDevices) {
    navigator.mediaDevices.enumerateDevices()
      .then(devs => {
        aiState.hasCamera = devs.some(d => d.kind === 'videoinput');
        if (!aiState.hasCamera) {
          // Probe resolved after mount: hide the Photo entry live too.
          if (aiChatEl) aiChatEl.camera = false;
          aiNoCamNote();
        }
      })
      .catch(() => { aiState.hasCamera = true; });
  }
  el.camera = aiState.hasCamera !== false;
  if (aiState.hasCamera === false) aiNoCamNote();
  // NOTE: do NOT set el.microphone — per deep-chat docs the microphone
  // recorder DISABLES speechToText. STT alone gives the mic button.
  el.speechToText = { webSpeech: { lang: 'zh-CN', language: 'zh-CN' } };
  el.onError = e => {
    const m = (e && (e.message || e.reason)) || '';
    if (/speech|recogni|microphone|mic|语音|麦克风/i.test(m)) toast('语音输入出错：' + m + '（请检查浏览器麦克风权限）', true);
  };
  el.dragAndDrop = true;
  el.mixedFiles = true;
  el.images = true;
  el.introMessage = { text: '你好，我是你的 AI 助手。输入问题开始对话（联网搜索默认开启，不带搜索能力的商家会自动联网查证）。' };
  el.remarkable = {
    html: true,
    highlight: (code, lang) => {
      try {
        if (window.hljs && lang && hljs.getLanguage(lang)) return hljs.highlight(code, { language: lang }).value;
      } catch (e) {}
      return '';
    },
  };
  el.connect = { handler: aiChatHandler, stream: true };
  el.addEventListener('click', aiOnChatClick);
  box.appendChild(el);
  aiRenderHistory();
}
function aiRenderHistory() {
  if (!aiChatEl) return;
  const chain = aiChain();
  aiChatEl.history = chain.map(aiToDCMessage);
  aiMaybePollVideo();
}
// While a video job is pending in the open thread, quietly refetch the
// thread every 15s so the finished video appears on its own.
function aiMaybePollVideo() {
  clearTimeout(aiState.videoPoll);
  if (!aiState.cur) return;
  const pending = (aiState.messages || []).some(m => {
    try { const u = JSON.parse(m.usageJson || '{}'); return u.video && u.video.status === 'pending'; } catch (e) { return false; }
  });
  if (!pending) return;
  const tid = aiState.cur;
  aiState.videoPoll = setTimeout(async () => {
    if (aiState.cur !== tid) return;
    const d = await api('GET', '/api/ai/threads/' + tid + '/messages').catch(() => null);
    if (d && aiState.cur === tid) { aiState.messages = d.messages || []; aiRenderHistory(); }
  }, 15000);
}
/* clicks inside chat bubbles (action footers) */
function aiOnChatClick(e) {
  const path = e.composedPath ? e.composedPath() : [];
  const btn = path.find(n => n && n.dataset && n.dataset.aiAct);
  if (!btn) return;
  const act = btn.dataset.aiAct;
  if (act === 'bprev' || act === 'bnext') {
    const pid = +btn.dataset.pid;
    const sibs = aiState.messages.filter(x => x.parentId === pid).sort((a, b) => a.id - b.id);
    const curId = (aiChain().find(m => m.parentId === pid) || {}).id;
    let idx = sibs.findIndex(s => s.id === curId);
    idx = act === 'bprev' ? Math.max(0, idx - 1) : Math.min(sibs.length - 1, idx + 1);
    aiState.branchPick[pid] = sibs[idx].id;
    aiRenderHistory();
    return;
  }
  const id = +btn.dataset.id;
  const msg = aiState.messages.find(m => m.id === id);
  if (!msg) return;
  if (act === 'copy') { copyText(msg.content || ''); return; }
  if (act === 'fork') {
    api('POST', `/api/ai/threads/${msg.threadId}/fork`, { messageId: id }).then(nt => {
      aiState.threads.unshift(nt);
      toast('已分叉为新会话');
      aiOpenThread(nt.id);
    }).catch(err => toast('分叉失败：' + err.message, true));
    return;
  }
  if (act === 'regen') {
    const userMsg = aiState.messages.find(m => m.id === msg.parentId);
    if (!userMsg) return;
    aiState.op = { kind: 'regen', id };
    aiState.branchPick[msg.parentId] = undefined;
    aiChatEl.submitUserMessage({ text: userMsg.content || '' });
    return;
  }
  if (act === 'edit') {
    openModal(`<h3>编辑消息</h3>
      <div class="frow"><textarea id="aiEditText" rows="5">${esc(msg.content || '')}</textarea></div>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn" id="aiEditSend">重新发送</button></div>`);
    $('#aiEditSend').onclick = () => {
      const nt = $('#aiEditText').value;
      closeModal();
      aiState.op = { kind: 'edit', id };
      aiChatEl.submitUserMessage({ text: nt });
    };
    return;
  }
}

/* ---------- chat handler (streams from SecretDock relay) ---------- */
async function aiUploadFile(fileObj) {
  const fd = new FormData();
  const raw = fileObj.file || fileObj;
  fd.append('file', raw, fileObj.name || raw.name || 'file');
  const r = await fetch('/api/ai/upload', { method: 'POST', body: fd });
  if (r.status === 401) { location.reload(); throw new Error('unauthorized'); }
  const d = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(d.error || '附件上传失败');
  return d;
}
async function aiChatHandler(body, signals) {
  const lastUser = [...(body.messages || [])].reverse().find(m => m.role === 'user') || {};
  const text = lastUser.text || '';
  const files = lastUser.files || [];
  let metas = [];
  try {
    if (files.length) aiSetStatus('正在上传附件…');
    for (const f of files) metas.push(await aiUploadFile(f));
  } catch (e) {
    aiSetStatus('');
    await signals.onResponse({ error: e.message || '附件上传失败' });
    return;
  }
  const op = aiState.op; aiState.op = null;
  const payload = { threadId: aiState.cur, content: text, attachments: metas };
  if (op && op.kind === 'regen') payload.regenerate = op.id;
  else if (op && op.kind === 'edit') payload.editFrom = op.id;
  else payload.parentId = aiLeafId();
  const ac = new AbortController();
  aiState.abort = ac;
  aiState.streaming = true;
  if (signals && signals.stopClicked) signals.stopClicked.listener = () => { try { ac.abort(); } catch (e) {} };
  let resp;
  try {
    resp = await fetch('/api/ai/chat', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload), signal: ac.signal,
    });
  } catch (e) {
    aiState.streaming = false; aiSetStatus('');
    await signals.onResponse({ error: '网络错误，发送失败' });
    return;
  }
  if (!resp.ok) {
    aiState.streaming = false; aiSetStatus('');
    const d = await resp.json().catch(() => ({}));
    await signals.onResponse({ error: d.error || ('发送失败 HTTP ' + resp.status) });
    return;
  }
  const reader = resp.body.getReader();
  const decoder = new TextDecoder();
  let buf = '', reasonAll = '', textAll = '', reasonSent = false;
  let userMsgId = 0, aiMsgId = 0, usage = null, errMsg = '', genFiles = null, genVideoPending = false;
  const handleEvent = ev => {
    if (ev.t === 'user') userMsgId = ev.id;
    else if (ev.t === 'reason') { reasonAll += ev.d; aiSetStatus('思考中…'); }
    else if (ev.t === 'tool') aiSetStatus(ev.d + '…');
    else if (ev.t === 'text') { textAll += ev.d; aiSetStatus(''); }
    else if (ev.t === 'usage') usage = ev;
    else if (ev.t === 'done') { aiMsgId = ev.aiId; usage = ev.usage; if (ev.files) genFiles = ev.files; if (ev.videoPending) genVideoPending = true; }
    else if (ev.t === 'error') { errMsg = ev.d; aiMsgId = ev.aiId || 0; }
  };
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      let idx;
      while ((idx = buf.indexOf('\n\n')) >= 0) {
        const raw = buf.slice(0, idx); buf = buf.slice(idx + 2);
        if (!raw.startsWith('data:')) continue;
        let ev = null;
        try { ev = JSON.parse(raw.slice(5)); } catch (e) { continue; }
        const before = textAll;
        handleEvent(ev);
        if (ev.t === 'text') {
          if (!reasonSent && reasonAll) { reasonSent = true; await signals.onResponse({ text: aiReasonBlock(reasonAll) }); }
          await signals.onResponse({ text: textAll.slice(before.length) ? textAll.slice(before.length) : ev.d });
        }
      }
    }
  } catch (e) {
    // aborted by user: partial answer was saved server-side; resync below
    aiState.streaming = false; aiSetStatus('');
    const d = await api('GET', '/api/ai/threads/' + aiState.cur + '/messages').catch(() => null);
    if (d) { aiState.messages = d.messages || []; aiRenderHistory(); }
    aiReloadThreads();
    return;
  }
  aiState.streaming = false; aiSetStatus('');
  if (!reasonSent && reasonAll) await signals.onResponse({ text: aiReasonBlock(reasonAll) });
  if (errMsg) await signals.onResponse({ text: `\n\n> 出错了：${errMsg}` });
  if (usage && (usage.in || usage.out)) {
    const line = aiUsageLine(usage);
    if (line) await signals.onResponse({ text: '\n\n' + line });
  }
  aiSyncAfterSend(op, payload, userMsgId, aiMsgId, text, textAll, reasonAll, usage, metas, errMsg, genFiles, genVideoPending);
  aiReloadThreads();
  setTimeout(() => { aiReloadThreads(); }, 5000); // pick up async auto-title
}
function aiSyncAfterSend(op, payload, userMsgId, aiMsgId, sentText, textAll, reasonAll, usage, metas, errMsg, genFiles, genVideoPending) {
  const uo = usage ? { ...usage } : {};
  if (errMsg) uo.error = errMsg;
  if (genVideoPending) uo.video = { status: 'pending' };
  const usageJson = Object.keys(uo).length ? JSON.stringify(uo) : '';
  const aiAtt = JSON.stringify(genFiles || []);
  if (op && op.kind === 'regen') {
    const old = aiState.messages.find(m => m.id === op.id);
    if (old && aiMsgId) {
      aiState.messages.push({ id: aiMsgId, threadId: aiState.cur, parentId: old.parentId, role: 'ai', content: textAll, reasoning: reasonAll, attachments: aiAtt, usageJson, createdAt: Math.floor(Date.now() / 1000) });
      aiState.branchPick[old.parentId] = aiMsgId;
    }
  } else if (op && op.kind === 'edit') {
    const old = aiState.messages.find(m => m.id === op.id);
    if (old && userMsgId) {
      aiState.messages.push({ id: userMsgId, threadId: aiState.cur, parentId: old.parentId, role: 'user', content: sentText, reasoning: '', attachments: JSON.stringify(metas || []), usageJson: '', createdAt: Math.floor(Date.now() / 1000) });
      aiState.branchPick[old.parentId] = userMsgId;
      if (aiMsgId) aiState.messages.push({ id: aiMsgId, threadId: aiState.cur, parentId: userMsgId, role: 'ai', content: textAll, reasoning: reasonAll, attachments: aiAtt, usageJson, createdAt: Math.floor(Date.now() / 1000) });
    }
  } else if (userMsgId) {
    aiState.messages.push({ id: userMsgId, threadId: aiState.cur, parentId: payload.parentId || 0, role: 'user', content: sentText, reasoning: '', attachments: JSON.stringify(metas || []), usageJson: '', createdAt: Math.floor(Date.now() / 1000) });
    if (aiMsgId) aiState.messages.push({ id: aiMsgId, threadId: aiState.cur, parentId: userMsgId, role: 'ai', content: textAll, reasoning: reasonAll, attachments: aiAtt, usageJson, createdAt: Math.floor(Date.now() / 1000) });
  }
  aiRenderHistory();
}

/* ---------- provider switcher ---------- */
function aiDictKindClient(model) {
  const m = (model || '').toLowerCase();
  if (['t2v', 'i2v', 'cogvideo', 'kling', 'hailuo', 'seedance', 'wan2', 'ltx-video', 'veo3', 'sora'].some(p => m.includes(p))) return 'video';
  if (['qwen-image', 'z-image', 'kolors', 'dall-e', 'flux', 'stable-diffusion', 'sdxl', 'cogview', 'seedream', 'imagen', 'gpt-image', 'glm-image', 'image-edit', '-image', '/image'].some(p => m.includes(p))) return 'image';
  return 'chat';
}
function aiKindOfModel(cfg, model) {
  return ((cfg && cfg.modelKinds) || {})[model] || aiDictKindClient(model);
}

function aiOpenSwitcher() {
  const wasOpen = !!$('.ai-popup.ai-switch-pop');
  aiClosePopup();
  if (wasOpen) return;
  const t = aiCurThread();
  if (!t) return;
  const r = $('#aiSwitch').getBoundingClientRect();
  const d = document.createElement('div');
  d.className = 'ai-popup ai-switch-pop';
  d.style.left = Math.max(8, r.left) + 'px';
  d.style.top = (r.bottom + 6) + 'px';
  let html = '';
  for (const c of aiState.configs) {
    const active = c.id === t.configId;
    html += `<div class="ai-sw-cfg ${active ? 'active' : ''}" data-cfg="${c.id}">
      ${aiCfgLogo(c, 22)}<span class="grow">${esc(c.name)}</span>
      ${c.lastStatus === 'ok' ? '<span class="ai-dot ok"></span>' : c.lastStatus === 'fail' ? '<span class="ai-dot fail"></span>' : ''}
    </div>`;
    if (active) {
      const models = c.models && c.models.length ? c.models : (c.defaultModel ? [c.defaultModel] : []);
      html += `<div class="ai-sw-models">` + models.map(m => {
        const k = aiKindOfModel(c, m);
        const badge = k === 'image' ? '<span class="ai-kind-badge">图</span>' : k === 'video' ? '<span class="ai-kind-badge">视频</span>' : '';
        return `<button data-model="${esc(m)}" class="${m === t.model ? 'active' : ''}">${esc(m)}${badge}</button>`;
      }).join('') + `</div>`;
    }
  }
  html += `<div class="ai-sw-foot"><button data-act="manage">管理 API…</button></div>`;
  d.innerHTML = html;
  document.body.appendChild(d);
  $$('.ai-sw-cfg', d).forEach(el => el.onclick = async () => {
    const cfg = aiCfg(+el.dataset.cfg);
    if (!cfg) return;
    await api('PUT', '/api/ai/threads/' + t.id, { configId: cfg.id, model: cfg.defaultModel });
    t.configId = cfg.id; t.model = cfg.defaultModel;
    aiClosePopup(); aiDrawSwitcher(); aiMountChat();
  });
  $$('[data-model]', d).forEach(b => b.onclick = async () => {
    await api('PUT', '/api/ai/threads/' + t.id, { model: b.dataset.model });
    t.model = b.dataset.model;
    aiClosePopup(); aiDrawSwitcher(); aiMountChat();
  });
  const mg = $('[data-act="manage"]', d);
  if (mg) mg.onclick = () => { aiClosePopup(); aiOpenManager(); };
  aiArmPopupClose();
}

/* ---------- prompts ---------- */
function aiOpenPrompts() {
  const prompts = (aiState.meta && aiState.meta.prompts) || [];
  openModal(`<h3>提示词</h3>
    <div id="aiPromptList">${prompts.length ? prompts.map((p, i) => `
      <div class="ai-prompt-row">
        <div class="grow"><b>${esc(p.name)}</b><div class="muted" style="font-size:12px">${esc((p.content || '').slice(0, 80))}…</div></div>
        <button class="btn sm" data-use="${i}">使用</button>
        <button class="icon-btn" data-editp="${i}" title="编辑">${ic('edit')}</button>
        <button class="icon-btn" data-delp="${i}" title="删除">${ic('trash')}</button>
      </div>`).join('') : '<p class="muted">还没有提示词。</p>'}
    </div>
    <div class="foot"><button class="btn ghost" data-close>关闭</button>
    <button class="btn" id="aiPromptAdd">${ic('plus')} 新建提示词</button></div>`);
  $$('#aiPromptList [data-use]').forEach(b => b.onclick = () => aiUsePrompt(prompts[+b.dataset.use]));
  $$('#aiPromptList [data-editp]').forEach(b => b.onclick = () => aiEditPrompt(+b.dataset.editp));
  $$('#aiPromptList [data-delp]').forEach(b => b.onclick = async () => {
    prompts.splice(+b.dataset.delp, 1);
    await aiSavePrompts(prompts);
    aiOpenPrompts();
  });
  $('#aiPromptAdd').onclick = () => aiEditPrompt(-1);
}
async function aiSavePrompts(prompts) {
  await api('PUT', '/api/ai/meta', { prompts });
  aiState.meta.prompts = prompts;
}
function aiEditPrompt(idx) {
  const prompts = (aiState.meta && aiState.meta.prompts) || [];
  const p = idx >= 0 ? prompts[idx] : { name: '', content: '' };
  openModal(`<h3>${idx >= 0 ? '编辑' : '新建'}提示词</h3>
    <div class="frow"><label>名称</label><input id="aiPrName" value="${esc(p.name)}" maxlength="40"></div>
    <div class="frow"><label>内容（可用 {{变量}} 占位）</label><textarea id="aiPrContent" rows="7">${esc(p.content)}</textarea></div>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="aiPrSave">保存</button></div>`);
  $('#aiPrSave').onclick = async () => {
    const name = $('#aiPrName').value.trim();
    const content = $('#aiPrContent').value;
    if (!name || !content.trim()) { toast('名称和内容都要填', true); return; }
    if (idx >= 0) prompts[idx] = { name, content }; else prompts.push({ id: 'p' + Date.now(), name, content });
    await aiSavePrompts(prompts);
    toast('已保存');
    aiOpenPrompts();
  };
}
function aiUsePrompt(p) {
  if (!p) return;
  const vars = [...new Set([...(p.content || '').matchAll(/\{\{([^}]+)\}\}/g)].map(m => m[1].trim()))];
  const doSend = text => {
    closeModal();
    if (!aiChatEl) { toast('先打开一个会话', true); return; }
    aiChatEl.submitUserMessage({ text });
  };
  if (!vars.length) { doSend(p.content); return; }
  openModal(`<h3>${esc(p.name)}</h3>
    ${vars.map((v, i) => `<div class="frow"><label>${esc(v)}</label><input data-var="${i}"></div>`).join('')}
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="aiPrSend">发送</button></div>`);
  $('#aiPrSend').onclick = () => {
    let text = p.content;
    vars.forEach((v, i) => { text = text.split('{{' + v + '}}').join($(`[data-var="${i}"]`).value); });
    doSend(text);
  };
}

/* ---------- API manager ---------- */
function aiClosePanel() { const p = $('#aiPanel'); if (p) p.remove(); }
function aiPanelShell(title, bodyHTML) {
  aiClosePanel();
  const main = $('.ai-main');
  const d = document.createElement('div');
  d.className = 'ai-panel';
  d.id = 'aiPanel';
  d.innerHTML = `<div class="ai-panel-head"><h3>${esc(title)}</h3><span style="flex:1"></span>
    <button class="icon-btn" id="aiPanelClose">${ic('x')}</button></div><div class="ai-panel-body">${bodyHTML}</div>`;
  main.appendChild(d);
  $('#aiPanelClose').onclick = aiClosePanel;
  return d;
}
async function aiOpenManager(addFirst, tab) {
  const body = `<div class="ai-mgr-sec">
      <div class="ai-mgr-sec-head"><b>我的 API</b><span style="flex:1"></span>
      <button class="btn sm" id="aiCfgAdd">${ic('plus')} 添加 API</button></div>
      <div id="aiCfgList"></div>
    </div>
    <div class="ai-mgr-sec" id="aiSearchSec">
      <div class="ai-mgr-sec-head"><b>联网搜索</b></div>
      <p class="muted" style="font-size:12px;margin:0 0 10px">有自带搜索的商家（Claude / Gemini / Perplexity 等）优先用自带；其余商家由 SecretDock 给模型递搜索工具，后端选一个：</p>
      <div class="frow"><label>搜索后端</label>
        <select id="aiSearchBackend">
          <option value="tavily">Tavily（API，免费额度 1000 次/月）</option>
          <option value="searxng">自建 SearXNG</option>
        </select></div>
      <div class="frow" id="aiTavilyRow"><label>Tavily API Key（每行一个，可填多个，轮流调用）</label>
        <textarea id="aiTavilyKey" rows="3" autocomplete="off" placeholder="tvly- 开头，每行一个 Key" style="width:100%"></textarea>
        <div style="display:flex;align-items:center;gap:8px;margin-top:4px">
          <span class="muted" id="aiTavilyState" style="font-size:12px"></span><span style="flex:1"></span>
          <button class="btn ghost sm" id="aiTavilyClear" type="button">清空 Key</button>
        </div></div>
      <div class="frow" id="aiSearxRow"><label>SearXNG 地址</label>
        <input id="aiSearxUrl" value="${esc((aiState.meta && aiState.meta.searxngUrl) || '')}" placeholder="https://search.example.com"></div>
      <button class="btn" id="aiSearchSave">保存搜索设置</button>
    </div>`;
  aiPanelShell('API 管理', body);
  aiDrawCfgList();
  $('#aiCfgAdd').onclick = () => aiCfgDialog(null);
  const backendSel = $('#aiSearchBackend');
  backendSel.value = (aiState.meta && aiState.meta.searchBackend) || 'tavily';
  const syncRows = () => {
    $('#aiTavilyRow').style.display = backendSel.value === 'tavily' ? '' : 'none';
    $('#aiSearxRow').style.display = backendSel.value === 'searxng' ? '' : 'none';
  };
  backendSel.onchange = syncRows; syncRows();
  const drawTavilyState = () => {
    const el = $('#aiTavilyState');
    if (!el) return;
    const n = (aiState.meta && aiState.meta.tavilyKeyCount) || 0;
    const hints = (aiState.meta && aiState.meta.tavilyKeyHints) || [];
    el.textContent = n
      ? `已保存 ${n} 个 Key：${hints.join('、') || '（已加密保存）'}，轮流调用。在上面重新填写会整体替换，留空保存则不改动。`
      : '还没有保存 Key。';
  };
  drawTavilyState();
  $('#aiTavilyClear').onclick = async () => {
    await api('PUT', '/api/ai/meta', { tavilyKeys: [] });
    aiState.meta = await api('GET', '/api/ai/meta');
    $('#aiTavilyKey').value = '';
    drawTavilyState();
    toast('Key 已清空');
  };
  $('#aiSearchSave').onclick = async () => {
    const body = { searchBackend: backendSel.value, searxngUrl: $('#aiSearxUrl').value.trim() };
    const lines = $('#aiTavilyKey').value.split('\n').map(s => s.trim()).filter(Boolean);
    if (lines.length) body.tavilyKeys = lines;
    await api('PUT', '/api/ai/meta', body);
    aiState.meta = await api('GET', '/api/ai/meta');
    $('#aiTavilyKey').value = '';
    drawTavilyState();
    toast('搜索设置已保存');
  };
  if (addFirst && !aiState.configs.length) aiCfgDialog(null);
  if (tab === 'search') { const s = $('#aiSearchSec'); if (s) s.scrollIntoView({ behavior: 'smooth' }); }
}
function aiStatusHTML(c) {
  if (c.lastStatus === 'ok') return `<span class="ai-dot ok"></span> 正常 · ${c.lastLatency}ms`;
  if (c.lastStatus === 'fail') return `<span class="ai-dot fail"></span> 异常`;
  return `<span class="ai-dot"></span> 未测`;
}
function aiDrawCfgList() {
  const box = $('#aiCfgList');
  if (!box) return;
  if (!aiState.configs.length) {
    box.innerHTML = '<p class="muted">还没有 API，点右上角添加。</p>';
    return;
  }
  const defId = aiState.meta && aiState.meta.defaultConfigId;
  box.innerHTML = aiState.configs.map(c => {
    const p = aiPreset(c.providerId);
    let bal = '';
    if (c.balanceJson) {
      try {
        const b = JSON.parse(c.balanceJson);
        bal = b.supported ? ` · 余额 ${esc(String(b.amount))}` : ' · 余额不支持';
      } catch (e) {}
    }
    return `<div class="ai-cfg-row">
      ${aiCfgLogo(c, 30)}
      <div class="grow">
        <div class="ai-cfg-name">${esc(c.name)} ${defId === c.id ? '<span class="tag accent">默认</span>' : ''} <span class="muted" style="font-size:12px">${p ? esc(p.name) : '自定义'} · ${esc(c.defaultModel || '未选模型')}</span></div>
        <div class="ai-cfg-meta">${aiStatusHTML(c)}${bal} · Key …${esc(c.keyLast4 || '')}</div>
      </div>
      <div class="ai-cfg-acts">
        <button class="btn ghost sm" data-test="${c.id}">测活</button>
        <button class="btn ghost sm" data-bal="${c.id}">余额</button>
        <button class="btn ghost sm" data-def="${c.id}">设默认</button>
        <button class="icon-btn" data-edit="${c.id}" title="编辑">${ic('edit')}</button>
        <button class="icon-btn" data-del="${c.id}" title="删除">${ic('trash')}</button>
      </div>
    </div>`;
  }).join('');
  $$('[data-test]', box).forEach(b => b.onclick = async () => {
    b.textContent = '测活中…'; b.disabled = true;
    try {
      const r = await api('POST', `/api/ai/configs/${b.dataset.test}/test`, {});
      toast(`测活成功：${r.latencyMs}ms`);
      const c = aiCfg(+b.dataset.test);
      if (c) { c.lastStatus = 'ok'; c.lastLatency = r.latencyMs; }
    } catch (e) {
      toast('测活失败：' + e.message, true);
      const c = aiCfg(+b.dataset.test);
      if (c) c.lastStatus = 'fail';
    }
    aiDrawCfgList();
  });
  $$('[data-bal]', box).forEach(b => b.onclick = async () => {
    try {
      const r = await api('POST', `/api/ai/configs/${b.dataset.bal}/balance`, {});
      if (!r.supported) { toast('这个商家没有余额接口'); return; }
      toast('余额：' + r.amount);
      const c = aiCfg(+b.dataset.bal);
      if (c) { c.balanceJson = JSON.stringify(r); c.balanceAt = r.at; }
      aiDrawCfgList();
    } catch (e) { toast('查余额失败：' + e.message, true); }
  });
  $$('[data-def]', box).forEach(b => b.onclick = async () => {
    await api('PUT', '/api/ai/meta', { defaultConfigId: +b.dataset.def });
    aiState.meta.defaultConfigId = +b.dataset.def;
    aiDrawCfgList();
    toast('已设为默认商家');
  });
  $$('[data-edit]', box).forEach(b => b.onclick = () => aiCfgDialog(aiCfg(+b.dataset.edit)));
  $$('[data-del]', box).forEach(b => b.onclick = () => {
    const c = aiCfg(+b.dataset.del);
    openModal(`<h3>删除「${esc(c.name)}」？</h3><p class="muted">使用这个商家的会话将无法继续对话（会话记录保留）。</p>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn danger" id="aiCfgDelYes">删除</button></div>`);
    $('#aiCfgDelYes').onclick = async () => {
      await api('DELETE', '/api/ai/configs/' + c.id);
      closeModal(); toast('已删除');
      aiState.configs = await api('GET', '/api/ai/configs');
      aiDrawCfgList(); aiDrawSwitcher();
    };
  });
}
function aiCfgDialog(c) {
  const isNew = !c;
  c = c || { name: '', providerId: '', format: 'openai', baseUrl: '', chatPath: '/v1/chat/completions', models: [], defaultModel: '', nativeSearch: false, priceIn: 0, priceOut: 0, keyLast4: '' };
  const tiles = aiState.presets.map(p => `
    <button class="ai-tile ${c.providerId === p.id ? 'active' : ''}" data-preset="${p.id}">
      ${p.logo ? `<img src="${AI_LOGO(p)}" alt="">` : `<span class="ai-logo-letter">${esc(p.name[0])}</span>`}
      <span>${esc(p.name)}</span><small>${esc(p.name_cn || '')}</small>
    </button>`).join('') + `
    <button class="ai-tile ${!c.providerId ? 'active' : ''}" data-preset="">
      <span class="ai-logo-letter">定</span><span>自定义</span><small>OpenAI 兼容</small>
    </button>`;
  openModal(`<h3>${isNew ? '添加' : '编辑'} API</h3>
    <div class="ai-tiles">${tiles}</div>
    <div class="frow"><label>显示名称</label><input id="cfName" value="${esc(c.name)}" maxlength="40" placeholder="如：我的 DeepSeek"></div>
    <div class="frow"><label>接口地址</label><input id="cfBase" value="${esc(c.baseUrl)}" placeholder="https://api.example.com/v1"></div>
    <div class="frow"><label>聊天路径</label><input id="cfPath" value="${esc(c.chatPath)}"></div>
    <div class="frow"><label>API Key</label><input id="cfKey" type="password" autocomplete="off" placeholder="${c.keyLast4 ? '已保存（…' + esc(c.keyLast4) + '），留空不改' : '粘贴你的 Key'}"></div>
    <div class="frow"><label>模型（每行一个）</label><textarea id="cfModels" rows="4">${esc((c.models || []).join('\n'))}</textarea>
    <div style="margin-top:6px;display:flex;gap:8px;align-items:center"><button class="btn ghost sm" id="cfFetchModels" ${isNew ? 'disabled' : ''}>从商家拉取模型列表</button><span class="muted" style="font-size:12px">${isNew ? '保存后可拉取' : '拉取会覆盖上面的列表，Pro 付费版会自动过滤'}</span></div></div>
    <div class="frow"><label>默认模型</label><input id="cfDefModel" value="${esc(c.defaultModel)}" placeholder="从上面的模型里选一个填这里"></div>
    <div class="frow" style="display:flex;gap:10px">
      <span style="flex:1"><label>输入价（/1M tokens）</label><input id="cfPriceIn" type="number" step="0.01" value="${c.priceIn || 0}"></span>
      <span style="flex:1"><label>输出价（/1M tokens）</label><input id="cfPriceOut" type="number" step="0.01" value="${c.priceOut || 0}"></span>
    </div>
    <p class="muted" style="font-size:12px">价格填 0 则只统计 token 数，不算花费。Key 加密保存在你的服务器上。</p>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="cfSave">${isNew ? '添加' : '保存'}</button></div>`, 'wide');
  let selPreset = c.providerId || '';
  $$('.ai-tile').forEach(t => t.onclick = () => {
    $$('.ai-tile').forEach(x => x.classList.remove('active'));
    t.classList.add('active');
    selPreset = t.dataset.preset;
    const p = aiPreset(selPreset);
    if (p) {
      if (!$('#cfName').value.trim()) $('#cfName').value = p.name;
      $('#cfBase').value = p.base_url || '';
      $('#cfPath').value = p.chat_path || '/v1/chat/completions';
      $('#cfModels').value = (p.models || []).join('\n');
      if (p.models && p.models.length) $('#cfDefModel').value = p.models[0];
    }
  });
  const fb = $('#cfFetchModels');
  if (fb && !isNew) fb.onclick = async () => {
    fb.disabled = true; fb.textContent = '拉取中…';
    try {
      const r = await api('POST', '/api/ai/configs/' + c.id + '/models', {});
      $('#cfModels').value = (r.models || []).join('\n');
      if (r.models && r.models.length && !$('#cfDefModel').value.trim()) $('#cfDefModel').value = r.models[0];
      const lc = aiCfg(c.id);
      if (lc) { lc.models = r.models || []; lc.modelKinds = r.modelKinds || {}; }
      toast(`拉取了 ${(r.models || []).length} 个模型`);
    } catch (e) { toast('拉取失败：' + e.message, true); }
    fb.disabled = false; fb.textContent = '从商家拉取模型列表';
  };
  $('#cfSave').onclick = async () => {
    const models = $('#cfModels').value.split('\n').map(s => s.trim()).filter(Boolean);
    const body = {
      name: $('#cfName').value.trim(),
      providerId: selPreset,
      format: (aiPreset(selPreset) || {}).format || c.format || 'openai',
      baseUrl: $('#cfBase').value.trim(),
      chatPath: $('#cfPath').value.trim(),
      key: $('#cfKey').value,
      models,
      defaultModel: $('#cfDefModel').value.trim() || models[0] || '',
      // Native web search is decided by the provider preset, not a user
      // checkbox: if the provider ships search it gets used automatically.
      nativeSearch: !!((aiPreset(selPreset) || {}).native_search),
      priceIn: parseFloat($('#cfPriceIn').value) || 0,
      priceOut: parseFloat($('#cfPriceOut').value) || 0,
    };
    if (!body.name) { toast('给这个 API 起个名字', true); return; }
    if (!body.baseUrl) { toast('接口地址不能为空', true); return; }
    if (isNew && !body.key) { toast('请填写 API Key', true); return; }
    try {
      if (isNew) await api('POST', '/api/ai/configs', body);
      else await api('PUT', '/api/ai/configs/' + c.id, body);
      closeModal(); toast('已保存');
      aiState.configs = await api('GET', '/api/ai/configs');
      aiDrawCfgList(); aiDrawSwitcher(); aiDrawThreads();
    } catch (e) { toast('保存失败：' + e.message, true); }
  };
}

/* ---------- data manager ---------- */
async function aiOpenData() {
  const st = await api('GET', '/api/ai/stats');
  const kinds = [
    ['image', '图片'], ['video', '视频'], ['audio', '语音'], ['text', '文本附件'], ['pdf', 'PDF'], ['file', '其他文件'],
  ];
  const body = `
    <div class="ai-mgr-sec">
      <div class="ai-mgr-sec-head"><b>个人档案（长记忆）</b></div>
      <p class="muted" style="font-size:12px;margin:0 0 8px">写下希望 AI 长期知道的事（身份、偏好、常用信息），每次对话都会带上。你亲手写、白盒可控。</p>
      <textarea id="aiProfile" rows="4" style="width:100%">${esc((aiState.meta && aiState.meta.profile) || '')}</textarea>
      <button class="btn" id="aiProfileSave" style="margin-top:8px">保存档案</button>
    </div>
    <div class="ai-mgr-sec">
      <div class="ai-mgr-sec-head"><b>数据占用</b></div>
      <div class="ai-stat-grid">
        <div><b>${st.threads}</b><span>会话</span></div>
        <div><b>${st.messages}</b><span>消息</span></div>
        <div><b>${aiFmtBytes(st.textBytes)}</b><span>对话文本</span></div>
        <div><b>${aiFmtBytes(st.reasonBytes)}</b><span>思考过程</span></div>
        <div><b>${aiFmtNum(st.usage.in + st.usage.out)}</b><span>累计 tokens</span></div>
        <div><b>${st.usage.cacheHit ? aiFmtNum(st.usage.cacheHit) : '0'}</b><span>缓存命中 tokens</span></div>
      </div>
      <table class="ai-kind-table">
        ${kinds.map(([k, label]) => {
          const ks = st.byKind[k] || { count: 0, bytes: 0 };
          return `<tr><td>${label}</td><td>${ks.count} 个 · ${aiFmtBytes(ks.bytes)}</td>
          <td><button class="btn ghost sm" data-clean="${k}" ${ks.count ? '' : 'disabled'}>清空</button></td></tr>`;
        }).join('')}
        <tr><td>思考过程</td><td>${aiFmtBytes(st.reasonBytes)}</td>
        <td><button class="btn ghost sm" data-clean="reasoning" ${st.reasonBytes ? '' : 'disabled'}>清空</button></td></tr>
      </table>
    </div>
    <div class="ai-mgr-sec">
      <div class="ai-mgr-sec-head"><b class="danger-t">危险区</b></div>
      <button class="btn danger ghost" id="aiCleanAll">清空全部 AI 数据（会话+消息+附件）</button>
    </div>`;
  aiPanelShell('数据管理', body);
  $('#aiProfileSave').onclick = async () => {
    await api('PUT', '/api/ai/meta', { profile: $('#aiProfile').value });
    aiState.meta.profile = $('#aiProfile').value;
    toast('档案已保存');
  };
  $$('#aiPanel [data-clean]').forEach(b => b.onclick = () => {
    const kind = b.dataset.clean;
    const label = { image: '所有图片', video: '所有视频', audio: '所有语音', text: '所有文本附件', pdf: '所有 PDF', file: '所有其他文件', reasoning: '所有思考过程' }[kind];
    openModal(`<h3>清空${label}？</h3><p class="muted">对话文字会保留，只删这类内容，不可恢复。</p>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn danger" id="aiCleanYes">清空</button></div>`);
    $('#aiCleanYes').onclick = async () => {
      await api('POST', '/api/ai/cleanup', { type: kind });
      closeModal(); toast('已清空');
      aiOpenData();
      if (aiState.cur) aiOpenThread(aiState.cur);
    };
  });
  $('#aiCleanAll').onclick = () => {
    openModal(`<h3>清空全部 AI 数据？</h3><p class="muted">所有会话、消息、附件都会被删除，不可恢复。输入 YES 确认。</p>
      <div class="frow"><input id="aiCleanYesInput" placeholder="YES"></div>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn danger" id="aiCleanAllYes">全部清空</button></div>`);
    $('#aiCleanAllYes').onclick = async () => {
      if ($('#aiCleanYesInput').value !== 'YES') { toast('请输入 YES', true); return; }
      await api('POST', '/api/ai/cleanup', { type: 'all' });
      closeModal(); toast('已清空');
      aiClosePanel();
      aiState.cur = 0; aiState.messages = [];
      renderAI();
    };
  };
}
