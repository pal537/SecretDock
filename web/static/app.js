/* SecretDock frontend — single-page app, no dependencies. */
'use strict';

/* ---------- icons (linear SVG, no emoji) ---------- */
const ICONS = {
  lock: '<rect x="4" y="10" width="16" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>',
  vault: '<rect x="3" y="8" width="18" height="12" rx="2"/><path d="M12 12v3M8 8V6a4 4 0 0 1 8 0v2"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  eye: '<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/>',
  eyeOff: '<path d="M17.94 17.94A10.6 10.6 0 0 1 12 19c-6.5 0-10-7-10-7a17.6 17.6 0 0 1 4.06-4.94M9.9 4.24A10.6 10.6 0 0 1 12 5c6.5 0 10 7 10 7a17.7 17.7 0 0 1-2.16 3.19M9.88 9.88a3 3 0 1 0 4.24 4.24"/><path d="M2 2l20 20"/>',
  copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
  edit: '<path d="M17 3a2.8 2.8 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5z"/>',
  trash: '<path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/>',
  gear: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>',
  bot: '<rect x="4" y="8" width="16" height="12" rx="3"/><path d="M12 8V4M8 4h8"/><circle cx="9" cy="14" r="1"/><circle cx="15" cy="14" r="1"/><path d="M9 17h6"/>',
  shield: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>',
  download: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>',
  upload: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12"/>',
  x: '<path d="M18 6L6 18M6 6l12 12"/>',
  back: '<path d="M19 12H5M12 19l-7-7 7-7"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 3"/>',
  key: '<circle cx="8" cy="15" r="4"/><path d="M10.85 12.15L21 2M18 5l3 3M15 8l3 3"/>',
  user: '<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
  terminal: '<path d="M4 17l6-6-6-6M12 19h8"/>',
  minus: '<path d="M5 12h14"/>',
  refresh: '<path d="M23 4v6h-6M1 20v-6h6"/><path d="M3.5 9a9 9 0 0 1 14.9-3.4L23 10M1 14l4.6 4.4A9 9 0 0 0 20.5 15"/>',
  alert: '<path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0zM12 9v4M12 17h.01"/>',
  check: '<path d="M20 6L9 17l-5-5"/>',
  moon: '<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"/>',
  logout: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9"/>',
  tag: '<path d="M20.59 13.41L11 3.83A2 2 0 0 0 9.59 3.24H4a1 1 0 0 0-1 1v5.59a2 2 0 0 0 .59 1.41l9.58 9.59a2 2 0 0 0 2.83 0l4.59-4.59a2 2 0 0 0 0-2.83z"/><circle cx="7.5" cy="7.5" r="1"/>',
  code: '<path d="M16 18l6-6-6-6M8 6l-6 6 6 6"/>',
  link: '<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>',
  share: '<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.6" y1="13.5" x2="15.4" y2="17.5"/><line x1="15.4" y1="6.5" x2="8.6" y2="10.5"/>',
  note: '<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.12 2.12 0 0 1 3 3L12 15l-4 1 1-4z"/>',
  image: '<rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/>',
  list: '<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>',
  file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/>',
  globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a15 15 0 0 1 0 18M12 3a15 15 0 0 0 0 18"/>',
  monitor: '<rect x="2" y="4" width="20" height="13" rx="2"/><path d="M8 21h8M12 17v4"/>',
  chevL: '<path d="M11 17l-5-5 5-5M18 17l-5-5 5-5"/>',
  chevD: '<path d="M6 9l6 6 6-6"/>',
  chevR: '<path d="M13 17l5-5-5-5M6 17l5-5-5-5"/>',
  arrowUp: '<path d="M12 19V5M5 12l7-7 7 7"/>',
  arrowDown: '<path d="M12 5v14M19 12l-7 7-7-7"/>',
  zap: '<path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>',
  archive: '<path d="M21 8v13H3V8M1 3h22v5H1zM10 12h4"/>',
  pdf: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6M9 15h6"/>',
  folder: '<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>',
  help: '<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3M12 17h.01"/>',
  bell: '<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>',
  pause: '<rect x="6" y="4" width="4" height="16" rx="1"/><rect x="14" y="4" width="4" height="16" rx="1"/>',
  play: '<polygon points="6 3 20 12 6 21 6 3"/>',
  pin: '<path d="M12 17v5M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16h14v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h1a2 2 0 0 0 0-4H8a2 2 0 0 0 0 4h1z"/>',
};
function ic(n) {
  return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${ICONS[n] || ''}</svg>`;
}

/* ---------- utils ---------- */
const $ = (s, r) => (r || document).querySelector(s);
const $$ = (s, r) => Array.from((r || document).querySelectorAll(s));
function esc(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}
function toast(msg, err) {
  const t = document.createElement('div');
  t.className = 'toast' + (err ? ' err' : '');
  t.textContent = msg;
  $('#toasts').appendChild(t);
  setTimeout(() => t.remove(), 2600);
}
async function api(method, path, body, opts) {
  const opt = { method, headers: {} };
  if (body !== undefined) { opt.headers['Content-Type'] = 'application/json'; opt.body = JSON.stringify(body); }
  const r = await fetch(path, opt);
  if (r.status === 401) {
    if (state.authed && !(opts && opts.noReload)) location.reload();
    throw new Error('unauthorized');
  }
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.error || ('HTTP ' + r.status));
  return data;
}
function copyText(s) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    return navigator.clipboard.writeText(s).then(() => toast('已复制'), () => toast('复制失败', true));
  }
  const ta = document.createElement('textarea');
  ta.value = s; document.body.appendChild(ta); ta.select();
  try { document.execCommand('copy'); toast('已复制'); } catch (e) { toast('复制失败', true); }
  ta.remove();
}
function fmtDate(ts) {
  if (!ts) return '';
  const d = new Date(ts * 1000);
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}/${p(d.getMonth()+1)}/${p(d.getDate())} ${d.getHours()}:${p(d.getMinutes())}`;
}
function expInfo(sec) {
  if (!sec.expiresAt) return null;
  const days = Math.ceil((sec.expiresAt - Date.now() / 1000) / 86400);
  if (days < 0) return { cls: 'exp', text: '已过期' };
  if (days === 0) return { cls: 'exp', text: '今天到期' };
  if (days <= 7) return { cls: 'soon', text: '还有' + days + '天到期' };
  return { cls: '', text: '还有' + days + '天到期' };
}
function avatarColor(name) {
  let h = 0;
  for (const c of name) h = (h * 31 + c.codePointAt(0)) % 360;
  return `hsl(${h}, 55%, 88%)`;
}
function avatarFg(name) {
  let h = 0;
  for (const c of name) h = (h * 31 + c.codePointAt(0)) % 360;
  return `hsl(${h}, 60%, 32%)`;
}
function logoHTML(m, size) {
  const s = size || 44;
  if (m.logoUrl) {
    return `<span class="mlogo" style="width:${s}px;height:${s}px;flex-basis:${s}px"><img src="${esc(m.logoUrl)}" alt="" loading="lazy" onerror="this.parentNode.innerHTML='${esc((m.name || '?')[0])}'"></span>`;
  }
  const ch = (m.name || '?')[0];
  return `<span class="mlogo" style="width:${s}px;height:${s}px;flex-basis:${s}px;background:${avatarColor(m.name)};color:${avatarFg(m.name)}">${esc(ch)}</span>`;
}

/* ---------- state ---------- */
const state = { view: 'merchants', merchants: [], cur: null, secrets: [], q: '', labels: [], authed: false };
const browserCfg = { enabled: false, url: '', pwd: '' };
/* ---------- sidebar menu: order / visibility / custom links ---------- */
const MENU_ITEMS = [
  { id: 'browser', label: '远程浏览器', icon: 'monitor' },
  { id: 'merchants', label: '机密库', icon: 'vault' },
  { id: 'scripts', label: '脚本', icon: 'code' },
  { id: 'links', label: '网址', icon: 'link' },
  { id: 'notes', label: '笔记', icon: 'note' },
  { id: 'images', label: '图片', icon: 'image' },
  { id: 'files', label: '文件', icon: 'file' },
  { id: 'nodes', label: '订阅节点', icon: 'globe' },
  { id: 'reminders', label: '提醒', icon: 'bell' },
  { id: 'ssh', label: 'SSH', icon: 'terminal' },
  { id: 'tghelp', label: 'TG操作说明', icon: 'help', foot: true },
  { id: 'settings', label: '设置', icon: 'gear', foot: true },
];
const MENU_BUILTIN_IDS = MENU_ITEMS.map(m => m.id);
const MENU_FOOT_IDS = MENU_ITEMS.filter(m => m.foot).map(m => m.id);
const MENU_DEFAULT_ORDER = MENU_BUILTIN_IDS.filter(id => !MENU_FOOT_IDS.includes(id));
const menuCfg = { order: [...MENU_DEFAULT_ORDER], hidden: [], links: [], folders: [], customOrder: [] };
let openFolders = new Set();
try { openFolders = new Set(JSON.parse(localStorage.getItem('sd-open-folders') || '[]')); } catch (e) {}
function sanitizeLinks(rawLinks) {
  let arr = [];
  try { arr = JSON.parse(rawLinks || '[]'); } catch (e) { arr = []; }
  if (!Array.isArray(arr)) arr = [];
  return arr.filter(l => l && typeof l === 'object')
    .filter(l => typeof l.id === 'string' && l.id.startsWith('lnk-'))
    .map(l => ({
      id: l.id,
      label: String(l.label || '').trim().slice(0, 40),
      url: String(l.url || '').trim().slice(0, 500),
      mode: l.mode === 'embed' ? 'embed' : 'new',
      folder: typeof l.folder === 'string' ? l.folder : '',
      icon: typeof l.icon === 'number' && l.icon > 0 ? Math.floor(l.icon) : 0,
    }))
    .filter(l => l.label && /^https?:\/\//i.test(l.url));
}
function sanitizeFolders(rawFolders) {
  let arr = [];
  try { arr = JSON.parse(rawFolders || '[]'); } catch (e) { arr = []; }
  if (!Array.isArray(arr)) arr = [];
  return arr.filter(f => f && typeof f === 'object')
    .filter(f => typeof f.id === 'string' && f.id.startsWith('fld-') && typeof f.name === 'string')
    .slice(0, 30)
    .map(f => ({ id: f.id.slice(0, 40), name: f.name.trim().slice(0, 40) }))
    .filter(f => f.name);
}
function linkItem(l) {
  return { id: l.id, label: l.label, icon: 'link', custom: true, url: l.url, mode: l.mode, folder: l.folder || '', iconTs: l.icon || 0 };
}
function folderItem(f) {
  return { id: f.id, label: f.name, icon: 'folder', folderObj: true, kids: [] };
}
function sanitizeMenuCfg(rawOrder, rawHidden, rawLinks, rawFolders, rawCustomOrder) {
  const folders = sanitizeFolders(rawFolders);
  const folderIds = new Set(folders.map(f => f.id));
  const links = sanitizeLinks(rawLinks).map(l => ({ ...l, folder: folderIds.has(l.folder) ? l.folder : '' }));
  let ord = [];
  try { ord = JSON.parse(rawOrder || '[]'); } catch (e) { ord = []; }
  if (!Array.isArray(ord)) ord = [];
  ord = ord.filter(id => MENU_BUILTIN_IDS.includes(id) && !MENU_FOOT_IDS.includes(id));
  for (const id of MENU_DEFAULT_ORDER) if (!ord.includes(id)) ord.push(id);
  // custom section order: folders + root-level links; migrate ids out of the old mixed menu_order
  let cord = [];
  try { cord = JSON.parse(rawCustomOrder || '[]'); } catch (e) { cord = []; }
  if (!Array.isArray(cord)) cord = [];
  const inCustom = id => folders.some(f => f.id === id) || links.some(l => l.id === id && !l.folder);
  cord = cord.filter(inCustom);
  let oldOrd = [];
  try { oldOrd = JSON.parse(rawOrder || '[]'); } catch (e) { oldOrd = []; }
  for (const id of oldOrd) if (inCustom(id) && !cord.includes(id)) cord.push(id);
  for (const f of folders) if (!cord.includes(f.id)) cord.push(f.id);
  for (const l of links) if (!l.folder && !cord.includes(l.id)) cord.push(l.id);
  let hid = [];
  try { hid = JSON.parse(rawHidden || '[]'); } catch (e) { hid = []; }
  if (!Array.isArray(hid)) hid = [];
  const validIds = new Set([...MENU_BUILTIN_IDS, ...folders.map(f => f.id), ...links.map(l => l.id)]);
  // settings must stay visible so the menu config is always reachable
  hid = hid.filter(id => id !== 'settings' && validIds.has(id));
  return { order: ord, hidden: hid, links, folders, customOrder: cord };
}
function visibleNavItems() {
  return MENU_ITEMS
    .filter(m => !m.foot)
    .filter(m => menuCfg.order.includes(m.id))
    .sort((a, b) => menuCfg.order.indexOf(a.id) - menuCfg.order.indexOf(b.id))
    .filter(m => !menuCfg.hidden.includes(m.id))
    .filter(m => m.id !== 'browser' || browserCfg.enabled);
}
function customSectionItems(editing) {
  // top-level custom entries in customOrder; folders carry their (visible) child links
  const out = [];
  const folderById = {};
  for (const f of menuCfg.folders) folderById[f.id] = f;
  const linkById = {};
  for (const l of menuCfg.links) linkById[l.id] = l;
  for (const id of menuCfg.customOrder) {
    const f = folderById[id];
    if (f) {
      const item = folderItem(f);
      item.kids = menuCfg.links.filter(l => l.folder === id)
        .filter(l => editing || !menuCfg.hidden.includes(l.id))
        .map(linkItem);
      if (editing || !menuCfg.hidden.includes(id)) out.push(item);
      continue;
    }
    const l = linkById[id];
    if (l && !l.folder && (editing || !menuCfg.hidden.includes(id))) out.push(linkItem(l));
  }
  return out;
}
function visibleCustomItems() { return customSectionItems(false); }
function visibleFootItems() {
  const out = [];
  const tg = MENU_ITEMS.find(m => m.id === 'tghelp');
  if (tg && !menuCfg.hidden.includes('tghelp')) out.push(tg);
  out.push(MENU_ITEMS.find(m => m.id === 'settings'));
  return out.filter(Boolean);
}
function findLinkItem(id) {
  const l = menuCfg.links.find(x => x.id === id);
  return l ? linkItem(l) : null;
}
async function loadBrowserCfg() {
  try {
    const s = await api('GET', '/api/settings');
    browserCfg.enabled = s.browser_enabled === '1' && !!s.browser_url;
    browserCfg.url = s.browser_url || '';
    browserCfg.pwd = s.browserPwd || '';
    const m = sanitizeMenuCfg(s.menu_order, s.menu_hidden, s.menu_links, s.menu_folders, s.menu_custom_order);
    menuCfg.order = m.order; menuCfg.hidden = m.hidden; menuCfg.links = m.links;
    menuCfg.folders = m.folders; menuCfg.customOrder = m.customOrder;
  } catch (e) {
    browserCfg.enabled = false; browserCfg.url = ''; browserCfg.pwd = '';
    menuCfg.order = [...MENU_DEFAULT_ORDER]; menuCfg.hidden = []; menuCfg.links = [];
    menuCfg.folders = []; menuCfg.customOrder = [];
  }
}

/* ---------- login ---------- */
function renderSetup() {
  $('#app').innerHTML = `
  <div class="login-wrap">
    <div class="login-card" style="text-align:center">
      <div style="display:flex;justify-content:center;margin-bottom:14px"><img src="/static/logo.svg?v=__APPV__" alt="SecretDock" style="width:72px;height:72px"></div>
      <h2 style="margin:0 0 6px">SecretDock</h2>
      <p class="muted" style="margin:0 0 18px">初始设置 · 创建管理员账号</p>
      <div style="text-align:left">
      <div class="field"><label>账号</label><input id="suUser" autocomplete="username" placeholder="设置登录账号"></div>
      <div class="field"><label>密码</label><input id="suPass" type="password" autocomplete="new-password" placeholder="大写+小写+数字，11位以上"></div>
      <div class="field"><label>重复密码</label><input id="suPass2" type="password" autocomplete="new-password" placeholder="再次输入密码"></div>
      <div id="suErr" class="danger-t" style="font-size:13px;min-height:20px;margin-bottom:6px"></div>
      <button class="btn" id="suGo" style="width:100%;justify-content:center">创建账号</button>
      <p class="muted" style="font-size:12px;margin-top:12px">只能创建一个账号，创建后此页面永久消失</p>
      </div>
    </div>
  </div>`;
  const go = async () => {
    $('#suErr').textContent = '';
    const u = $('#suUser').value.trim();
    const p1 = $('#suPass').value;
    const p2 = $('#suPass2').value;
    if (!u) { $('#suErr').textContent = '账号不能为空'; return; }
    if (p1 !== p2) { $('#suErr').textContent = '两次密码不一致'; return; }
    try {
      await api('POST', '/api/setup', { username: u, password: p1 }, { noReload: true });
      location.reload();
    } catch (e) { $('#suErr').textContent = e.message; }
  };
  $('#suGo').onclick = go;
  $('#suPass2').onkeydown = e => { if (e.key === 'Enter') go(); };
  $('#suUser').focus();
}

function renderLogin() {
  const insecure = location.protocol === 'http:' && !/^(localhost|127\.0\.0\.1)$/.test(location.hostname);
  $('#app').innerHTML = `
  <div class="login-wrap">
    <div class="login-card" style="text-align:center">
      <div style="display:flex;justify-content:center;margin-bottom:14px"><img src="/static/logo.svg?v=__APPV__" alt="SecretDock" style="width:64px;height:64px"></div>
      <h2 style="margin:0 0 6px">SecretDock</h2>
      <p class="muted" style="margin:0 0 18px">机密库 · 请登录</p>
      <div style="text-align:left">
      <div class="field"><label>用户名</label><input id="liUser" autocomplete="username"></div>
      <div class="field"><label>密码</label><input id="liPass" type="password" autocomplete="current-password"></div>
      <label class="check"><input type="checkbox" id="liRemember" checked> 记住我（30 天）</label>
      <div id="liErr" class="danger-t" style="font-size:13px;min-height:20px;margin-bottom:6px"></div>
      <button class="btn" id="liGo" style="width:100%;justify-content:center">${ic('lock')} 登录</button>
      </div>
    </div>
  </div>
  ${insecure ? '<div id="httpWarn" style="position:fixed;bottom:18px;left:50%;transform:translateX(-50%);background:#fef3c7;color:#92400e;padding:10px 18px;border-radius:10px;font-size:13px;box-shadow:0 4px 12px rgba(0,0,0,.15);z-index:9999">当前为 HTTP 访问，账号密码可能被窃听，建议使用 HTTPS</div>' : ''}`;
  const go = async () => {
    $('#liErr').textContent = '';
    try {
      const r = await api('POST', '/api/login', {
        username: $('#liUser').value.trim(),
        password: $('#liPass').value,
        remember: $('#liRemember').checked,
      }, { noReload: true });
      if (r.need2fa) { render2FAStep(r.tmp); return; }
      state.authed = true;
      boot();
    } catch (e) { $('#liErr').textContent = e.message === 'unauthorized' ? '用户名或密码错误' : e.message; }
  };
  $('#liGo').onclick = go;
  $('#liPass').onkeydown = e => { if (e.key === 'Enter') go(); };
  $('#liUser').focus();
  // HTTP 警告：每会话只显示一次，10秒消失
  const warnEl = $('#httpWarn');
  if (warnEl) {
    if (sessionStorage.getItem('httpWarnShown')) {
      warnEl.remove();
    } else {
      sessionStorage.setItem('httpWarnShown', '1');
      setTimeout(() => warnEl.remove(), 10000);
    }
  }
}

/* ---------- 2FA 登录步骤 ---------- */
function render2FAStep(tmp) {
  let curTmp = tmp;
  let useRecovery = false;
  $('#app').innerHTML = `
  <div class="login-wrap">
    <div class="login-card" style="text-align:center">
      <div style="display:flex;justify-content:center;margin-bottom:14px"><img src="/static/logo.svg?v=__APPV__" alt="SecretDock" style="width:64px;height:64px"></div>
      <h2 style="margin:0 0 6px">SecretDock</h2>
      <p class="muted" style="margin:0 0 18px" id="tfaTitle">两步验证 · 输入验证器 App 上的 6 位动态码</p>
      <div style="text-align:left">
      <div class="field"><label id="tfaLabel">动态码</label>
        <input id="liCode" inputmode="numeric" autocomplete="one-time-code" placeholder="6 位数字" style="font-size:20px;letter-spacing:8px;text-align:center"></div>
      <div style="text-align:center;margin-bottom:10px"><a href="javascript:void 0" id="tfaSwap" style="font-size:13px">改用恢复码</a></div>
      <div id="liErr" class="danger-t" style="font-size:13px;min-height:20px;margin-bottom:6px"></div>
      <button class="btn" id="liGo2" style="width:100%;justify-content:center">${ic('lock')} 验证登录</button>
      <div style="text-align:center;margin-top:10px"><a href="javascript:void 0" id="tfaBack" style="font-size:13px" class="muted">← 返回账号密码登录</a></div>
      </div>
    </div>
  </div>`;
  const go = async () => {
    $('#liErr').textContent = '';
    const code = $('#liCode').value.trim();
    if (!code) return;
    try {
      // 直接用 fetch：401 时也要读回后端重发的 tmp（可免输密码重试动态码）
      const r = await fetch('/api/login/2fa', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tmp: curTmp, code }),
      });
      const data = await r.json().catch(() => ({}));
      if (!r.ok) {
        if (data.tmp) curTmp = data.tmp;
        throw new Error(data.error || ('HTTP ' + r.status));
      }
      if (data.recoveryUsed) toast('恢复码已使用，还剩 ' + data.recoveryLeft + ' 个，请尽快重新生成');
      state.authed = true;
      boot();
    } catch (e) { $('#liErr').textContent = e.message; }
  };
  $('#liGo2').onclick = go;
  $('#liCode').onkeydown = e => { if (e.key === 'Enter') go(); };
  $('#liCode').focus();
  $('#tfaSwap').onclick = () => {
    useRecovery = !useRecovery;
    $('#tfaTitle').textContent = useRecovery ? '两步验证 · 输入恢复码（XXXX-XXXX）' : '两步验证 · 输入验证器 App 上的 6 位动态码';
    $('#tfaLabel').textContent = useRecovery ? '恢复码' : '动态码';
    $('#liCode').placeholder = useRecovery ? 'XXXX-XXXX' : '6 位数字';
    $('#liCode').value = '';
    $('#tfaSwap').textContent = useRecovery ? '改用动态码' : '改用恢复码';
    $('#liCode').focus();
  };
  $('#tfaBack').onclick = () => renderLogin();
}

/* ---------- shell ---------- */
function shell(content) {
  const insecure = location.protocol === 'http:' && !/^(localhost|127\.0\.0\.1)$/.test(location.hostname);
  const collapsed = state.sbCollapsed === true;
  function menuIcon(m) {
    if (!(m.custom && m.iconTs)) return ic(m.icon);
    return `<span class="nav-icowrap">${ic('link')}<img class="nav-ico" src="/menu-icons/${m.id}?v=${m.iconTs}" alt="" loading="lazy" onerror="this.remove()"></span>`;
  }
  function navBtn(m, kid) {
    const lbl = `<span class="nav-label">${esc(m.label)}</span>`;
    const kidCls = kid ? ' kid' : '';
    const lead = menuIcon(m);
    if (state.editMode && m.id !== 'settings') {
      const hid = menuCfg.hidden.includes(m.id);
      const eye = `<button class="icon-btn mini" data-act="eye" data-id="${m.id}" title="${hid ? '显示' : '隐藏'}">${ic(hid ? 'eyeOff' : 'eye')}</button>`;
      const btns = m.id === 'tghelp' ? eye
        : `<button class="icon-btn mini" data-act="up" data-id="${m.id}" title="上移">${ic('arrowUp')}</button>`
        + `<button class="icon-btn mini" data-act="down" data-id="${m.id}" title="下移">${ic('arrowDown')}</button>`
        + eye
        + (m.custom ? `<button class="icon-btn mini" data-act="edit" data-id="${m.id}" title="编辑链接">${ic('edit')}</button>` : '');
      return `<div class="nav-item editing${kidCls}${hid ? ' is-hidden' : ''}" title="${esc(m.label)}">${lead}${lbl}<span class="edit-btns">${btns}</span></div>`;
    }
    const act = state.view === m.id ? ' active' : '';
    if (m.custom && m.mode === 'new')
      return `<button class="nav-item${kidCls}${act}" data-ext="new" data-url="${esc(m.url)}" title="${esc(m.label)}">${lead}${lbl}</button>`;
    return `<button class="nav-item${kidCls}${act}" data-nav="${m.id}" title="${esc(m.label)}">${lead}${lbl}</button>`;
  }
  function folderBtn(f) {
    const lbl = `<span class="nav-label">${esc(f.label)}</span>`;
    if (state.editMode) {
      const hid = menuCfg.hidden.includes(f.id);
      return `<div class="nav-item editing${hid ? ' is-hidden' : ''}" title="${esc(f.label)}">${ic(f.icon)}${lbl}<span class="edit-btns">`
        + `<button class="icon-btn mini" data-act="up" data-id="${f.id}" title="上移">${ic('arrowUp')}</button>`
        + `<button class="icon-btn mini" data-act="down" data-id="${f.id}" title="下移">${ic('arrowDown')}</button>`
        + `<button class="icon-btn mini" data-act="eye" data-id="${f.id}" title="${hid ? '显示' : '隐藏'}">${ic(hid ? 'eyeOff' : 'eye')}</button>`
        + `<button class="icon-btn mini" data-act="rename" data-id="${f.id}" title="重命名">${ic('edit')}</button>`
        + `<button class="icon-btn mini" data-act="delfolder" data-id="${f.id}" title="删除文件夹">${ic('trash')}</button>`
        + `</span></div>`
        + f.kids.map(k => navBtn(k, true)).join('');
    }
    const open = openFolders.has(f.id);
    return `<div class="nav-item folder${open ? ' open' : ''}" data-folder="${f.id}" title="${esc(f.label)}">${ic(f.icon)}${lbl}<span class="chev">${ic('chevD')}</span></div>`
      + (open ? `<div class="folder-kids">` + f.kids.map(k => navBtn(k, true)).join('') + `</div>` : '');
  }
  function customBtn(item) {
    if (item.folderObj) return folderBtn(item);
    return navBtn(item);
  }
  const navHtml = (state.editMode ? editModeItems() : visibleNavItems()).map(m => navBtn(m)).join('');
  const customItems = customSectionItems(state.editMode);
  const customHtml = (customItems.length || state.editMode)
    ? `<div class="nav-divider"></div>` + customItems.map(customBtn).join('') : '';
  const addRows = state.editMode
    ? `<button class="nav-item" id="menuAddLink">${ic('plus')}<span class="nav-label">添加链接</span></button>`
    + `<button class="nav-item" id="menuAddFolder">${ic('folder')}<span class="nav-label">新建文件夹</span></button>`
    : '';
  const footHtml = (state.editMode
    ? ['tghelp', 'settings'].map(id => MENU_ITEMS.find(m => m.id === id))
    : visibleFootItems()).filter(Boolean).map(m => navBtn(m)).join('');
  return `
  <div class="sidebar ${collapsed ? 'collapsed' : ''}${state.editMode ? ' editing' : ''}">
    <div class="brand"><span class="logo"><img src="/static/logo.svg?v=__APPV__" alt="SecretDock"></span><span class="brand-text"><b>SecretDock</b><small>机密库</small></span><button class="icon-btn sb-toggle" id="sbToggle" title="${collapsed ? '展开菜单' : '折叠菜单'}">${ic(collapsed ? 'chevR' : 'chevL')}</button></div>
    <nav class="nav">
      ${navHtml}${customHtml}${addRows}
    </nav>
    <div class="side-foot">
      ${footHtml}
      <div class="foot-row">
        <button class="icon-btn" id="menuEditBtn" title="${state.editMode ? '编辑完成' : '编辑菜单'}">${ic(state.editMode ? 'check' : 'edit')}</button>
        <button class="icon-btn" id="themeBtn" title="深色/浅色">${ic(document.documentElement.dataset.theme === 'dark' ? 'sun' : 'moon')}</button>
        <button class="icon-btn" id="logoutBtn" title="退出登录">${ic('logout')}</button>
      </div>
    </div>
  </div>
  <div class="main">
    ${insecure ? `<div class="banner http">${ic('alert')}<span><b>危险：</b>当前使用 HTTP 访问，账号密码与机密数据可能被窃听。请配置 HTTPS 后使用。</span></div>` : ''}
    <div id="view"></div>
  </div>`;
}
function bindShell() {
  if (state.editMode) {
    $$('.nav-item.editing [data-act]').forEach(b => b.onclick = e => { e.stopPropagation(); menuEditAct(b.dataset.act, b.dataset.id); });
    const add = $('#menuAddLink');
    if (add) add.onclick = () => openLinkDialog(null);
    const addF = $('#menuAddFolder');
    if (addF) addF.onclick = () => openFolderDialog(null);
  } else {
    $$('.nav-item').forEach(b => b.onclick = () => {
      if (b.dataset.folder) { toggleFolder(b.dataset.folder); return; }
      if (b.dataset.ext === 'new') { window.open(b.dataset.url, '_blank', 'noopener'); return; }
      nav(b.dataset.nav);
    });
  }
  $('#sbToggle').onclick = () => {
    state.sbCollapsed = !state.sbCollapsed;
    try { localStorage.setItem('sd-sb-collapsed', state.sbCollapsed ? '1' : '0'); } catch (e) {}
    const sb = $('.sidebar');
    sb.classList.toggle('collapsed', state.sbCollapsed);
    $('#sbToggle').innerHTML = ic(state.sbCollapsed ? 'chevR' : 'chevL');
    $('#sbToggle').title = state.sbCollapsed ? '展开菜单' : '折叠菜单';
  };
  $('#menuEditBtn').onclick = toggleEditMode;
  $('#themeBtn').onclick = () => {
    const cur = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
    document.documentElement.dataset.theme = cur;
    try { localStorage.setItem('sd-theme', cur); } catch (e) {}
    $('#themeBtn').innerHTML = ic(cur === 'dark' ? 'sun' : 'moon');
  };
  $('#logoutBtn').onclick = async () => { await api('POST', '/api/logout'); location.reload(); };
}
function nav(v) {
  if (typeof v === 'string' && v.startsWith('lnk-')) {
    const li = findLinkItem(v);
    if (!li) return;
    if (li.mode === 'new') { window.open(li.url, '_blank', 'noopener'); return; }
  }
  state.view = v; state.cur = null;
  try { localStorage.setItem('sd-view', v); } catch (e) {}
  $('#app').innerHTML = shell('');
  bindShell();
  renderView();
}

/* ---------- merchants view ---------- */
async function renderMerchants() {
  await refreshMerchants();
  const v = $('#view');
  v.innerHTML = `
    <div class="topbar">
      <h2>服务商</h2>
      <div class="search">${ic('search')}<input id="q" placeholder="搜索服务商、标签、备注…" value="${esc(state.q)}"></div>
      <span style="flex:1"></span>
      <button class="btn sm" id="addM">${ic('plus')} 添加服务商</button>
    </div>
    <div id="expBanner"></div>
    <div id="mlist" class="grid"><div class="empty"><div>加载中…</div></div></div>`;
  $('#addM').onclick = () => merchantModal(null);
  let t;
  $('#q').oninput = e => { clearTimeout(t); t = setTimeout(() => { state.q = e.target.value.trim(); drawMerchants(); }, 250); };
  // expiring banner
  try {
    const exp = await api('GET', '/api/expiring?days=7');
    if (exp.length) {
      $('#expBanner').innerHTML = `<div class="banner warn" id="expGo">${ic('clock')}<span>有 ${exp.length} 条机密 7 天内到期（或已过期），点击查看</span></div>`;
      $('#expGo').onclick = () => showExpiring(exp);
    }
  } catch (e) {}
  await drawMerchants();
}
async function drawMerchants() {
  const box = $('#mlist');
  let list = state.merchants;
  if (state.q) {
    const q = state.q.toLowerCase();
    list = list.filter(m => m.name.toLowerCase().includes(q) ||
      (m.aliases || []).some(a => a.toLowerCase().includes(q)));
  }
  if (!list.length) {
    box.innerHTML = `<div class="empty">${ic('vault')}<div>还没有服务商，点右上角添加第一个吧</div></div>`;
    return;
  }
  // expiring merchant ids for red dots
  let expIds = new Set();
  try {
    const exp = await api('GET', '/api/expiring?days=7');
    exp.forEach(s => expIds.add(s.merchantId));
  } catch (e) {}
  box.innerHTML = list.map(m => `
    <div class="mcard-wrap" data-id="${m.id}">
      <button class="mcard" data-id="${m.id}">
        ${logoHTML(m)}
        <span style="min-width:0"><span class="nm">${m.isPinned ? '<span class="pinmark">📌 </span>' : ''}${esc(m.name)}${expIds.has(m.id) ? '<span class="expdot" title="有即将到期的机密"></span>' : ''}</span><br>
        <span class="ct">${m.count} 条机密</span></span>
      </button>
      ${pinBtnHTML('merchants', m)}
    </div>`).join('');
  $$('.mcard', box).forEach(b => b.onclick = () => openMerchant(+b.dataset.id));
  bindPinBtns(box, 'merchants', renderMerchants);
}
async function showExpiring(list) {
  const names = {};
  state.merchants.forEach(m => names[m.id] = m.name);
  openModal(`
    <h3>${ic('clock')} 即将到期的机密（7 天内）</h3>
    ${list.map(s => {
      const ei = expInfo(s);
      return `<div class="kv"><span>${esc(names[s.merchantId] || '')} · ${esc(s.label || '（未命名）')}</span><span class="${ei.cls === 'exp' ? 'danger-t' : 'warn-t'}">${ei.text}</span></div>`;
    }).join('')}
    <div class="foot"><button class="btn ghost" data-close>关闭</button></div>`);
}

/* ---------- merchant detail ---------- */
async function openMerchant(id) {
  const m = state.merchants.find(x => x.id === id);
  if (!m) return;
  state.cur = m;
  state.view = 'detail';
  $('#app').innerHTML = shell('');
  bindShell();
  const v = $('#view');
  v.innerHTML = `
    <div class="crumbs"><button id="backBtn">${ic('back')} 全部服务商</button></div>
    <div class="topbar">
      ${logoHTML(m, 52)}
      <div><h2 style="margin:0">${esc(m.name)}</h2>
      <div class="muted" style="font-size:12px">${(m.aliases || []).map(esc).join(' · ')}</div></div>
      <span style="flex:1"></span>
      <button class="btn ghost sm" id="editM">${ic('edit')} 编辑服务商</button>
      <button class="btn sm" id="addS">${ic('plus')} 添加密钥</button>
    </div>
    <div id="slist"></div>`;
  $('#backBtn').onclick = () => nav('merchants');
  $('#editM').onclick = () => merchantModal(m);
  $('#addS').onclick = () => secretModal(null, 'secret');
  await drawSecrets();
}
async function drawSecrets() {
  const box = $('#slist');
  let list;
  try { list = await api('GET', '/api/secrets?merchantId=' + state.cur.id); }
  catch (e) { box.innerHTML = `<div class="empty">加载失败</div>`; return; }
  if (!list.length) { box.innerHTML = `<div class="empty">${ic('key')}<div>还没有机密，添加第一条吧</div></div>`; return; }
  // 同名标签合并成一组，组内按从新到旧
  const groups = [];
  const gmap = new Map();
  list.forEach(s => {
    const key = s.kind + '\0' + (s.label || '');
    if (!gmap.has(key)) {
      const g = { kind: s.kind, label: s.label, items: [] };
      gmap.set(key, g);
      groups.push(g);
    }
    gmap.get(key).items.push(s);
  });
  const secretMap = {};
  const entryHTML = s => {
    secretMap[s.id] = s.value || '';
    const uname = s.kind === 'account' && s.username
      ? `<div class="smeta">账号：<code>${esc(s.username)}</code></div>` : '';
    const ei = expInfo(s);
    return `
    <div class="srow sub" data-id="${s.id}">
      <div class="grow">
        ${uname}
        <div class="sval">••••••••</div>
        <div class="smeta">添加于 ${fmtDate(s.createdAt)}${s.note ? ' · ' + esc(s.note) : ''}${ei ? ' · <span class="' + ei.cls + '">' + ei.text + '</span>' : ''}</div>
      </div>
      <div class="acts">
        <button class="icon-btn" data-act="reveal" title="显示/隐藏">${ic('eye')}</button>
        <button class="icon-btn" data-act="copy" title="复制">${ic('copy')}</button>
        <button class="icon-btn" data-act="edit" title="编辑">${ic('edit')}</button>
        <button class="icon-btn" data-act="del" title="删除">${ic('trash')}</button>
      </div>
    </div>`;
  };
  box.innerHTML = `<div class="card" style="padding:6px 18px">` + groups.map(g => {
    const tags = [`<span class="tag">${g.kind === 'account' ? '账号密码' : '密钥'}</span>`];
    if (g.items.length > 1) tags.push(`<span class="tag accent">${g.items.length} 条</span>`);
    return `
    <div class="sgroup">
      <div class="lab" style="padding:10px 0 2px">${esc(g.label || '（未命名）')} ${tags.join('')}</div>
      ${g.items.map(entryHTML).join('')}
    </div>`;
  }).join('') + `</div>`;
  $$('.srow', box).forEach(row => {
    const id = +row.dataset.id;
    const valEl = $('.sval', row);
    const real = secretMap[id] || '';
    let shown = false;
    $('[data-act="reveal"]', row).onclick = e => {
      shown = !shown;
      valEl.textContent = shown ? real : '••••••••';
      e.currentTarget.innerHTML = ic(shown ? 'eyeOff' : 'eye');
    };
    $('[data-act="copy"]', row).onclick = () => copyText(real);
    $('[data-act="edit"]', row).onclick = async () => {
      const s = (await api('GET', '/api/secrets?merchantId=' + state.cur.id)).find(x => x.id === id);
      secretModal(s, s.kind);
    };
    $('[data-act="del"]', row).onclick = () => {
      openModal(`<h3>删除这条机密？</h3><p class="muted">删除后无法恢复。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/secrets/' + id);
        closeModal(); toast('已删除'); drawSecrets(); refreshMerchants();
      };
    };
  });
}

/* ---------- merchant modal ---------- */
function merchantModal(m) {
  const isNew = !m;
  openModal(`
    <h3>${isNew ? '添加服务商' : '编辑服务商'}</h3>
    <div class="field"><label>服务商名称</label>
      <input id="mmName" value="${esc(m ? m.name : '')}" placeholder="如 Cloudflare" autocomplete="off"></div>
    <div class="field"><label>别名（逗号分隔，如 cf, CF）</label>
      <input id="mmAliases" value="${esc(m ? (m.aliases || []).join(', ') : '')}" placeholder="cf, CF"></div>
    <div class="field"><label>Logo</label>
      <div class="logosel" id="mmLogos"></div>
      <div class="hint">输入名称自动获取；也可上传、选择，或留空用首字母。</div>
      <input type="file" id="mmUpload" accept="image/*" style="display:none">
      <div style="margin-top:8px;display:flex;gap:8px;flex-wrap:wrap">
        <button class="btn ghost sm" id="mmFetch">${ic('search')} 获取 Logo</button>
        <button class="btn ghost sm" id="mmUpBtn">${ic('upload')} 上传 Logo</button>
        <button class="btn ghost sm" id="mmNoLogo">不要 Logo</button>
      </div>
    </div>
    ${!isNew ? `<button class="btn danger sm" id="mmDel" style="margin-top:6px">${ic('trash')} 删除该服务商（含全部机密）</button>` : ''}
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="mmSave">${ic('check')} 保存</button></div>`);
  let logoUrl = m ? m.logoUrl : '';
  let timer;
  const renderLogos = cands => {
    const box = $('#mmLogos');
    const opts = [{ url: '', label: '首字母' }].concat(cands.map(c => ({ url: c.url, label: c.kind })));
    box.innerHTML = opts.map(o => `
      <button class="logopt ${logoUrl === o.url ? 'sel' : ''}" data-url="${esc(o.url)}" title="${esc(o.label)}">
        ${o.url ? `<img src="${esc(o.url)}" loading="lazy" onerror="this.parentNode.style.display='none'">` : '<span>A</span>'}
      </button>`).join('');
    $$('.logopt', box).forEach(b => b.onclick = () => { logoUrl = b.dataset.url; renderLogos(cands); });
  };
  renderLogos([]);
  const fetchCands = async () => {
    const name = $('#mmName').value.trim();
    if (name.length < 2) { renderLogos([]); return; }
    $('#mmLogos').innerHTML = '<span class="muted" style="font-size:12px">获取 Logo 中…</span>';
    try {
      const cands = await api('GET', '/api/logo/candidates?name=' + encodeURIComponent(name));
      if (!logoUrl && cands.length) logoUrl = cands[0].url;
      renderLogos(cands);
    } catch (e) {}
  };
  $('#mmName').oninput = () => { clearTimeout(timer); timer = setTimeout(fetchCands, 500); };
  if (isNew) fetchCands();
  $('#mmUpBtn').onclick = () => $('#mmUpload').click();
  $('#mmUpload').onchange = async e => {
    const f = e.target.files[0];
    if (!f) return;
    toast('上传中…');
    // need merchant id first: create/update then upload
    window._pendingLogoFile = f;
    toast('保存后将上传此 Logo');
  };
  $('#mmNoLogo').onclick = () => { logoUrl = ''; renderLogos([]); };
  $('#mmFetch').onclick = () => { logoUrl = ''; fetchCands(); };
  if (!isNew) $('#mmDel').onclick = () => {
    openModal(`<h3>删除服务商“${esc(m.name)}”？</h3><p class="muted">其下全部机密一并删除，无法恢复。</p>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn danger" id="cfDelM">删除</button></div>`);
    $('#cfDelM').onclick = async () => {
      await api('DELETE', '/api/merchants/' + m.id);
      closeModal(); toast('已删除');
      await refreshMerchants(); nav('merchants');
    };
  };
  $('#mmSave').onclick = async () => {
    const name = $('#mmName').value.trim();
    if (!name) { toast('名称不能为空', true); return; }
    const aliases = $('#mmAliases').value.split(/[,，]/).map(s => s.trim()).filter(Boolean);
    try {
      let saved;
      if (isNew) {
        saved = await api('POST', '/api/merchants', { name, aliases, logoUrl });
      } else {
        saved = await api('PUT', '/api/merchants/' + m.id, { name, aliases, logoUrl });
      }
      // upload logo file if chosen
      const f = window._pendingLogoFile;
      window._pendingLogoFile = null;
      if (f) {
        const fd = new FormData();
        fd.append('logo', f);
        const r = await fetch('/api/merchants/' + saved.id + '/logo', { method: 'POST', body: fd });
        if (!r.ok) toast('Logo 上传失败', true);
      }
      closeModal(); toast('已保存');
      await refreshMerchants();
      if (isNew) openMerchant(saved.id);
      else { state.cur = state.merchants.find(x => x.id === m.id); openMerchant(m.id); }
    } catch (e) { toast(e.message, true); }
  };
  setTimeout(() => $('#mmName').focus(), 50);
}

/* ---------- secret modal ---------- */
async function secretModal(s, kind) {
  const isNew = !s;
  kind = s ? s.kind : (kind || 'secret');
  let labels = [];
  try { labels = await api('GET', '/api/labels'); } catch (e) {}
  openModal(`
    <h3>${isNew ? '添加' : '编辑'}${kind === 'account' ? '账号密码' : '密钥'}</h3>
    <div class="tabs" style="margin-bottom:14px">
      <button class="tab ${kind === 'secret' ? 'active' : ''}" data-k="secret">${ic('key')} 密钥</button>
      <button class="tab ${kind === 'account' ? 'active' : ''}" data-k="account">${ic('user')} 账号密码</button>
    </div>
    <div class="field"><label>标签</label>
      <input id="smLabel" list="labelList" value="${esc(s ? s.label : '')}" placeholder="如 访问密钥" autocomplete="off">
      <datalist id="labelList">${labels.map(l => `<option value="${esc(l)}">`).join('')}</datalist></div>
    <div class="field" id="smUserWrap" style="display:${kind === 'account' ? '' : 'none'}"><label>用户名 / 账号</label>
      <input id="smUser" value="${esc(s ? s.username : '')}" autocomplete="off"></div>
    <div class="field"><label>${kind === 'account' ? '密码' : '值'}</label>
      <textarea id="smValue" rows="3" placeholder="粘贴密钥值…">${esc(s ? s.value : '')}</textarea></div>
    <div class="row2">
      <div class="field"><label>备注（可选）</label><input id="smNote" value="${esc(s ? s.note : '')}"></div>
      <div class="field"><label>有效期（天，0 = 不填）</label><input id="smExp" type="number" min="0" value="${s && s.expiresAt ? Math.max(0, Math.ceil((s.expiresAt - Date.now() / 1000) / 86400)) : ''}" placeholder="如 60"></div>
    </div>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="smSave">${ic('check')} 保存</button></div>`, true);
  $$('.tab', $('#modal-root')).forEach(t => t.onclick = () => {
    secretModal(s, t.dataset.k);
  });
  $('#smSave').onclick = async () => {
    const value = $('#smValue').value;
    if (!value.trim()) { toast('值不能为空', true); return; }
    const expDays = parseInt($('#smExp').value, 10);
    const body = {
      merchantId: state.cur.id,
      kind,
      label: $('#smLabel').value.trim(),
      username: $('#smUser').value.trim(),
      value,
      note: $('#smNote').value.trim(),
      expiresAt: expDays > 0 ? Math.floor(Date.now() / 1000) + expDays * 86400 : 0,
    };
    try {
      if (isNew) await api('POST', '/api/secrets', body);
      else await api('PUT', '/api/secrets/' + s.id, body);
      closeModal(); toast('已保存'); drawSecrets();
    } catch (e) { toast(e.message, true); }
  };
  setTimeout(() => $('#smLabel').focus(), 50);
}

/* ---------- settings ---------- */
let settingsTab = 'tg';
async function renderSettings() {
  const v = $('#view');
  v.innerHTML = `
    <div class="topbar"><h2>设置</h2></div>
    <div class="tabs">
      <button class="tab ${settingsTab === 'tg' ? 'active' : ''}" data-t="tg">${ic('bot')} TG 机器人</button>
      <button class="tab ${settingsTab === 'backup' ? 'active' : ''}" data-t="backup">${ic('download')} 备份</button>
      <button class="tab ${settingsTab === 'browser' ? 'active' : ''}" data-t="browser">${ic('globe')} 远程浏览器</button>
      <button class="tab ${settingsTab === 'audit' ? 'active' : ''}" data-t="audit">${ic('shield')} 安全日志</button>
      <button class="tab ${settingsTab === 'clear' ? 'active' : ''}" data-t="clear">${ic('trash')} 清理数据</button>
      <button class="tab ${settingsTab === 'pwd' ? 'active' : ''}" data-t="pwd">${ic('lock')} 修改密码</button>
      <button class="tab ${settingsTab === 'sec' ? 'active' : ''}" data-t="sec">${ic('shield')} 登录安全</button>
    </div>
    <div id="setBody"></div>`;
  $$('.tab', v).forEach(t => t.onclick = () => { settingsTab = t.dataset.t; renderSettings(); });
  if (settingsTab === 'tg') await renderTgSettings();
  else if (settingsTab === 'backup') await renderBackupSettings();
  else if (settingsTab === 'browser') await renderBrowserSettings();
  else if (settingsTab === 'clear') await renderClearData();
  else if (settingsTab === 'pwd') await renderChangePwd();
  else if (settingsTab === 'sec') await renderSecSettings();
  else await renderAudit();
}
async function renderTgSettings() {
  const s = await api('GET', '/api/settings');
  const tokenSet = !!s.botTokenSet;
  $('#setBody').innerHTML = `
  <div class="card"><h3>${ic('bot')} Telegram 机器人</h3>
    <div class="field"><label>Bot Token ${tokenSet ? '<span class="tag">已设置</span>' : ''}</label>
      <div id="stTokenWrap">
        ${tokenSet
          ? `<div style="display:flex;gap:8px;align-items:center"><input value="••••••••••••" disabled style="flex:1"><button class="btn ghost sm" id="stTokenChg" type="button">更换</button></div>`
          : `<input id="stToken" type="password" placeholder="从 @BotFather 获取" autocomplete="off">`}
      </div>
      <div class="hint">Token 加密保存；更换时才会发送新值</div></div>
    <div class="field"><label>管理员 Chat ID（只有这个 ID 能用机器人）</label>
      <input id="stChat" value="${esc(s.admin_chat_id || '')}" placeholder="如 123456789" autocomplete="off">
      <div class="hint">给 @userinfobot 发消息可查到自己的 ID</div></div>
    <div class="field"><label>查询密码 ${s.queryPwdSet ? '<span class="tag">已设置</span>' : ''}</label>
      <div class="pwdwrap"><input id="stQpwd" type="password" placeholder="${s.queryPwdSet ? '已设置，留空不修改' : '每次 TG 查询都要输入，几位都行'}" autocomplete="new-password">
      <button class="btn ghost sm iconbtn" id="stQpwdEye" title="显示/隐藏">${ic("eye")}</button></div></div>
    <div class="row2">
      <div class="field"><label>查询结果自动删除（分钟）</label>
        <input id="stDel" type="number" min="1" value="${esc(s.auto_delete_minutes || '3')}"></div>
      <div class="field"><label>查询密码重试次数</label>
        <input id="stTries" type="number" min="1" max="10" value="${esc(s.query_pwd_max_tries || '3')}"></div>
    </div>
    <div class="row2">
      <div class="field"><label>无对话自动清空（分钟）</label>
        <input id="stIdle" type="number" min="1" value="${esc(s.tg_idle_wipe_minutes || '5')}">
        <div class="hint">超过此时长无对话，自动删除全部聊天记录</div></div>
      <div class="field"><label>提醒默认连发条数</label>
        <input id="stRemTimes" type="number" min="1" max="10" value="${esc(s.reminder_default_times || '3')}">
        <div class="hint">到时间后连发几条提醒消息</div></div>
    </div>
    ${s.tgBanUntil ? `<div class="banner warn" style="margin:0 0 12px">${ic('shield')}<span> TG 查询已被封禁 ${s.tgBanCount ? '(第' + esc(s.tgBanCount) + '次)' : ''}，<button class="btn ghost sm" id="stUnban" style="margin-left:8px">解除封禁</button></span></div>` : ''}
    <div style="display:flex;gap:10px;flex-wrap:wrap">
      <button class="btn" id="stSave">${ic('check')} 保存 TG 设置</button>
      <button class="btn ghost" id="stTest">${ic('bot')} 测试机器人</button>
    </div>
    <div class="hint" style="margin-top:10px">用法：多行文本=添加（首行服务商名）；单行服务商名=查询（需输查询密码）。入库后留一条回执，其余聊天记录自动删除。</div>
  </div>`;
  const chg = $('#stTokenChg');
  if (chg) chg.onclick = () => {
    $('#stTokenWrap').innerHTML = `<input id="stToken" type="password" placeholder="粘贴新的 Bot Token" autocomplete="off">`;
    $('#stToken').focus();
  };
  let testCd = 0;
  const unb = $('#stUnban');
  if (unb) unb.onclick = async () => {
    try { await api('POST', '/api/settings/tg-unban', {}, { noReload: true }); toast('已解除封禁'); renderTgSettings(); }
    catch (e) { toast(e.message, true); }
  };
  $('#stTest').onclick = async () => {
    if (testCd > 0) { toast('测试冷却中（' + testCd + 's）', true); return; }
    const btn = $('#stTest'); btn.disabled = true; btn.textContent = '测试中…';
    try {
      const t = $('#stToken') ? $('#stToken').value : '';
      const r = await api('POST', '/api/settings/tg-test', { botToken: t, adminChatId: $('#stChat').value.trim() }, { noReload: true });
      btn.classList.add('ok'); btn.innerHTML = '✓ 测试成功 @' + esc(r.bot);
      toast('测试成功 @' + r.bot);
      setTimeout(() => { btn.classList.remove('ok'); btn.innerHTML = '测试机器人'; }, 5000);
    } catch (e) { toast(e.message, true); }
    // 10 秒冷却，防止重复发送
    testCd = 10;
    const tick = setInterval(() => {
      testCd--;
      if (testCd <= 0) { clearInterval(tick); btn.disabled = false; btn.textContent = '测试机器人'; }
      else btn.textContent = '冷却 ' + testCd + 's';
    }, 1000);
  };
  const eye = $('#stQpwdEye');
  if (eye) eye.onclick = (e) => {
    e.preventDefault();
    const inp = $('#stQpwd');
    inp.type = inp.type === 'password' ? 'text' : 'password';
  };
  $('#stSave').onclick = async () => {
    const body = {
      admin_chat_id: $('#stChat').value.trim(),
      auto_delete_minutes: $('#stDel').value.trim() || '3',
      query_pwd_max_tries: $('#stTries').value.trim() || '3',
      tg_idle_wipe_minutes: $('#stIdle').value.trim() || '5',
      reminder_default_times: $('#stRemTimes').value.trim() || '3',
    };
    const tokEl = $('#stToken');
    if (tokEl && tokEl.value) body.botToken = tokEl.value;
    try {
      await api('PUT', '/api/settings', body);
      const qp = $('#stQpwd').value;
      if (qp) await api('POST', '/api/settings/query-password', { password: qp });
      toast('已保存'); renderTgSettings();
    } catch (e) { toast(e.message, true); }
  };
}
/* ---------- 登录安全 ---------- */
const SEC_PINNED = ['CN', 'HK', 'US', 'SG', 'JP', 'KR', 'DE'];
const SEC_COUNTRIES = [
['CN','中国'],['HK','香港'],['TW','台湾'],['MO','澳门'],
['US','美国'],['CA','加拿大'],['MX','墨西哥'],['BR','巴西'],['AR','阿根廷'],['CL','智利'],['CO','哥伦比亚'],['PE','秘鲁'],
['GB','英国'],['DE','德国'],['FR','法国'],['IT','意大利'],['ES','西班牙'],['NL','荷兰'],['BE','比利时'],['CH','瑞士'],['AT','奥地利'],['SE','瑞典'],['NO','挪威'],['DK','丹麦'],['FI','芬兰'],['IE','爱尔兰'],['PT','葡萄牙'],['GR','希腊'],['PL','波兰'],['CZ','捷克'],['BG','保加利亚'],['HU','匈牙利'],['RO','罗马尼亚'],['UA','乌克兰'],['RU','俄罗斯'],['BY','白俄罗斯'],
['JP','日本'],['KR','韩国'],['KP','朝鲜'],['SG','新加坡'],['MY','马来西亚'],['TH','泰国'],['VN','越南'],['PH','菲律宾'],['ID','印度尼西亚'],['IN','印度'],['PK','巴基斯坦'],['BD','孟加拉国'],['LK','斯里兰卡'],['NP','尼泊尔'],['MM','缅甸'],['KH','柬埔寨'],['LA','老挝'],['MN','蒙古'],['KZ','哈萨克斯坦'],['UZ','乌兹别克斯坦'],['KG','吉尔吉斯斯坦'],['TJ','塔吉克斯坦'],['TM','土库曼斯坦'],['AZ','阿塞拜疆'],['GE','格鲁吉亚'],['AM','亚美尼亚'],
['AU','澳大利亚'],['NZ','新西兰'],['FJ','斐济'],['PG','巴布亚新几内亚'],
['AE','阿联酋'],['SA','沙特阿拉伯'],['QA','卡塔尔'],['KW','科威特'],['BH','巴林'],['OM','阿曼'],['IL','以色列'],['TR','土耳其'],['IR','伊朗'],['IQ','伊拉克'],['SY','叙利亚'],['JO','约旦'],['LB','黎巴嫩'],['YE','也门'],
['ZA','南非'],['EG','埃及'],['NG','尼日利亚'],['KE','肯尼亚'],['GH','加纳'],['ET','埃塞俄比亚'],['MA','摩洛哥'],['DZ','阿尔及利亚'],['TN','突尼斯'],
['IS','冰岛'],['LU','卢森堡'],['MT','马耳他'],['CY','塞浦路斯'],['EE','爱沙尼亚'],['LV','拉脱维亚'],['LT','立陶宛'],['SK','斯洛伐克'],['SI','斯洛文尼亚'],['HR','克罗地亚'],['BA','波黑'],['RS','塞尔维亚'],['ME','黑山'],['MK','北马其顿'],['AL','阿尔巴尼亚'],['MD','摩尔多瓦'],['MC','摩纳哥'],['LI','列支敦士登'],['SM','圣马力诺'],['VA','梵蒂冈'],['AD','安道尔'],['GI','直布罗陀'],['FO','法罗群岛'],['GL','格陵兰'],['IM','马恩岛'],['JE','泽西岛'],['GG','根西岛'],['AX','奥兰群岛'],
['BO','玻利维亚'],['PY','巴拉圭'],['UY','乌拉圭'],['EC','厄瓜多尔'],['VE','委内瑞拉'],['GY','圭亚那'],['SR','苏里南'],['GF','法属圭亚那'],['PA','巴拿马'],['CR','哥斯达黎加'],['NI','尼加拉瓜'],['HN','洪都拉斯'],['SV','萨尔瓦多'],['GT','危地马拉'],['BZ','伯利兹'],['CU','古巴'],['DO','多米尼加'],['HT','海地'],['JM','牙买加'],['TT','特立尼达和多巴哥'],['BB','巴巴多斯'],['BS','巴哈马'],['PR','波多黎各'],['VI','美属维尔京群岛'],['VG','英属维尔京群岛'],['KY','开曼群岛'],['BM','百慕大'],['AG','安提瓜和巴布达'],['DM','多米尼克'],['GD','格林纳达'],['KN','圣基茨和尼维斯'],['LC','圣卢西亚'],['VC','圣文森特和格林纳丁斯'],['AW','阿鲁巴'],['CW','库拉索'],['BQ','荷属加勒比区'],['SX','荷属圣马丁'],['MF','法属圣马丁'],['BL','圣巴泰勒米'],['GP','瓜德罗普'],['MQ','马提尼克'],['TC','特克斯和凯科斯群岛'],['AI','安圭拉'],['MS','蒙特塞拉特'],
['MV','马尔代夫'],['BT','不丹'],['BN','文莱'],['TL','东帝汶'],['LK','斯里兰卡'],
['KZ','哈萨克斯坦'],
['DZ','阿尔及利亚'],['AO','安哥拉'],['BJ','贝宁'],['BW','博茨瓦纳'],['BF','布基纳法索'],['BI','布隆迪'],['CM','喀麦隆'],['CV','佛得角'],['CF','中非'],['TD','乍得'],['KM','科摩罗'],['CG','刚果（布）'],['CD','刚果（金）'],['CI','科特迪瓦'],['DJ','吉布提'],['GQ','赤道几内亚'],['ER','厄立特里亚'],['SZ','斯威士兰'],['GA','加蓬'],['GM','冈比亚'],['GN','几内亚'],['GW','几内亚比绍'],['LR','利比里亚'],['LY','利比亚'],['MG','马达加斯加'],['MW','马拉维'],['ML','马里'],['MR','毛里塔尼亚'],['MU','毛里求斯'],['YT','马约特'],['MZ','莫桑比克'],['NA','纳米比亚'],['NE','尼日尔'],['RW','卢旺达'],['SH','圣赫勒拿'],['ST','圣多美和普林西比'],['SN','塞内加尔'],['SC','塞舌尔'],['SL','塞拉利昂'],['LS','莱索托'],['SO','索马里'],['SS','南苏丹'],['SD','苏丹'],['TZ','坦桑尼亚'],['TG','多哥'],['UG','乌干达'],['ZM','赞比亚'],['ZW','津巴布韦'],['RE','留尼汪'],
['AF','阿富汗'],['PS','巴勒斯坦'],['EH','西撒哈拉'],
['AS','美属萨摩亚'],['CK','库克群岛'],['GU','关岛'],['KI','基里巴斯'],['MH','马绍尔群岛'],['FM','密克罗尼西亚'],['NR','瑙鲁'],['NC','新喀里多尼亚'],['NU','纽埃'],['NF','诺福克岛'],['MP','北马里亚纳群岛'],['PW','帕劳'],['PF','法属波利尼西亚'],['PN','皮特凯恩群岛'],['WS','萨摩亚'],['SB','所罗门群岛'],['TK','托克劳'],['TO','汤加'],['TV','图瓦卢'],['VU','瓦努阿图'],['WF','瓦利斯和富图纳'],
['AQ','南极洲'],['BV','布维岛'],['CC','科科斯群岛'],['CX','圣诞岛'],['GS','南乔治亚和南桑威奇群岛'],['HM','赫德岛和麦克唐纳群岛'],['IO','英属印度洋领地'],['FK','福克兰群岛'],['TF','法属南部和南极领地'],['SJ','斯瓦尔巴和扬马延'],['UM','美国本土外小岛屿'],['PM','圣皮埃尔和密克隆']
];
function secCountryName(code) {
  const f = SEC_COUNTRIES.find(c => c[0] === code);
  return f ? f[1] : code;
}
// 去重（上面手写难免有重复）
const _seenCC = new Set();
const SEC_COUNTRIES_U = SEC_COUNTRIES.filter(c => !_seenCC.has(c[0]) && (_seenCC.add(c[0]), true));

let secSelCountries = [];

function secRenderCountryList(filter) {
  const q = (filter || '').trim().toLowerCase();
  const sel = new Set(secSelCountries);
  const match = c => !q || c[1].includes(filter.trim()) || c[0].toLowerCase().includes(q);
  const row = c => {
    const name = c[1], code = c[0];
    return `<label class="ccheck"><input type="checkbox" data-cc="${code}" ${sel.has(code) ? 'checked' : ''}><span>${esc(name)} <em>${code}</em></span></label>`;
  };
  const pinned = SEC_COUNTRIES_U.filter(c => SEC_PINNED.includes(c[0]) && match(c));
  const rest = SEC_COUNTRIES_U.filter(c => !SEC_PINNED.includes(c[0]) && match(c));
  $('#cSelList').innerHTML =
    (pinned.length ? `<div class="csec">常用</div>` + pinned.map(row).join('') : '') +
    (rest.length ? `<div class="csec">全部国家/地区</div>` + rest.map(row).join('') : '') +
    ((!pinned.length && !rest.length) ? `<div class="muted" style="padding:10px;font-size:13px">没有匹配的国家/地区</div>` : '');
  $$('#cSelList input[data-cc]').forEach(cb => cb.onchange = () => {
    const code = cb.dataset.cc;
    if (cb.checked && !secSelCountries.includes(code)) secSelCountries.push(code);
    if (!cb.checked) secSelCountries = secSelCountries.filter(x => x !== code);
    secUpdateSelLabel();
  });
}
function secUpdateSelLabel() {
  const n = secSelCountries.length;
  const names = secSelCountries.slice(0, 3).map(secCountryName).join('、');
  $('#cSelBtnText').textContent = n ? `已选 ${n} 个${names ? '（' + names + (n > 3 ? '…' : '') + '）' : ''}` : '请选择允许访问的国家/地区';
}

async function renderSecSettings() {
  const s = await api('GET', '/api/settings');
  let info = null;
  try { info = await api('GET', '/api/settings/security/ipinfo'); } catch (e) {}
  const enabled = s.sec_enabled === '1';
  secSelCountries = (s.sec_countries || '').split(',').map(x => x.trim().toUpperCase()).filter(x => x.length === 2 && SEC_COUNTRIES_U.some(c => c[0] === x));
  $('#setBody').innerHTML = `
  <div class="card"><h3>${ic('shield')} 登录安全 <span class="tag ${enabled ? '' : 'soon'}">${enabled ? '已启用' : '未启用'}</span></h3>
    <p class="muted" style="font-size:13px">默认关闭。开启后：IP 访问策略 + 可选的 2FA 两步验证。内网 IP（127.0.0.1 / 192.168.x.x 等）始终放行，不会被锁在外面。</p>
    ${info ? `<p style="font-size:13px">你当前的 IP：<code>${esc(info.ip)}</code>${info.country ? '（' + esc(secCountryName(info.country)) + '）' : ''} · 判定：<b style="color:${info.allowed ? 'var(--ok)' : 'var(--danger)'}">${info.allowed ? '允许' : '拒绝：' + esc(info.reason)}</b></p>` : ''}
    <label class="check" style="font-size:14px;color:var(--text)"><input type="checkbox" id="secEnabled" ${enabled ? 'checked' : ''}> <b>启用登录安全</b></label>
    <div class="field"><label>允许访问的国家/地区</label>
      <div class="cselect">
        <button type="button" class="cselect-btn" id="cSelBtn"><span id="cSelBtnText"></span><span>▾</span></button>
        <div class="cselect-panel" id="cSelPanel" style="display:none">
          <input id="cSelSearch" placeholder="搜索国家/地区名称或代码…" autocomplete="off">
          <div class="cselect-list" id="cSelList"></div>
        </div>
      </div>
      <div class="hint">只有这些国家/地区的 IP 能打开登录页和调用接口。建议只选你常在的 1～2 个。</div>
    </div>
    <div class="field"><label>IP 白名单 <span class="muted" style="font-weight:normal">（每行一个，支持 IP 段如 1.2.3.0/24，IPv4/IPv6 均可）</span></label>
      <textarea id="secWl" class="notearea" style="min-height:70px" placeholder="1.2.3.4&#10;2001:db8::/32" spellcheck="false">${esc(s.sec_ip_whitelist || '')}</textarea>
      <div class="hint">白名单 IP 无视国家限制，直接放行。黑名单优先级最高。</div>
    </div>
    <div class="field"><label>IP 黑名单 <span class="muted" style="font-weight:normal">（每行一个，支持 IP 段，IPv4/IPv6 均可）</span></label>
      <textarea id="secBl" class="notearea" style="min-height:70px" placeholder="9.9.9.9&#10;10.0.0.0/8" spellcheck="false">${esc(s.sec_ip_blacklist || '')}</textarea>
      <div class="hint">黑名单 IP 直接拒绝，连登录页都打不开。</div>
    </div>
    <div class="field"><label>IP 国家库</label>
      <div style="display:flex;gap:10px;align-items:center;flex-wrap:wrap">
        <span class="tag ${s.geoipLoaded === '1' ? '' : 'soon'}">${s.geoipLoaded === '1' ? '已加载' : '未加载'}</span>
        <button class="btn ghost sm" id="geoUpBtn">更新 IP 库</button>
        <span class="hint" style="margin:0">免费库（DB-IP），每月更新一次即可</span>
      </div>
    </div>
    <div class="field">
      <div style="display:flex;align-items:center;gap:8px;margin-bottom:5px"><input type="checkbox" id="secTgAlert" style="width:15px;height:15px" ${s.sec_tg_alert === '1' ? 'checked' : ''}><b style="font-size:14px;color:var(--text)">TG 安全提醒</b></div>
      <div class="hint">登录成功 / 新 IP 登录 / 密码错误 / 错误过多被锁定时，机器人给你发提醒（带 IP 和归属地）。每条提醒都带「一键封禁」按钮，点的 IP 立刻无法访问网站；消息 3 分钟后自动删除。需要先在「TG 机器人」里配好机器人。<br>提醒开关独立于上面的 IP 策略，不开 IP 策略也能收提醒。</div>
    </div>
    <div style="display:flex;gap:10px"><button class="btn" id="secSave">${ic('check')} 保存安全设置</button></div>
    <div class="hint" style="margin-top:8px">保存时会自动检查：至少要选一个国家或填一条白名单；且当前 IP 必须能通过新规则，防止把自己锁在外面。</div>
  </div>
  <div class="card" id="sec2faCard"><h3>${ic('lock')} 两步验证（2FA）</h3><div id="sec2faBody"><p class="muted">加载中…</p></div></div>
  <div class="card"><h3>${ic('shield')} 已封禁 IP <span class="tag" id="banCount"></span></h3>
    <div id="banList"><p class="muted">加载中…</p></div>
    <div class="field" style="margin-top:10px"><label>手动封禁 IP</label>
      <div style="display:flex;gap:8px"><input id="banIpInput" placeholder="1.2.3.4" style="flex:1"><button class="btn ghost sm" id="banAddBtn">封禁</button></div>
      <div class="hint">被封禁的 IP 连登录页都打不开。TG 提醒里点的「一键封禁」也会出现在这里。万一把自己封了：服务器上执行 <code>docker exec secretdock /secretdock unban 你的IP</code> 解封。</div>
    </div>
  </div>`;
  secUpdateSelLabel();
  secRenderCountryList('');
  $('#cSelBtn').onclick = e => { e.stopPropagation(); const p = $('#cSelPanel'); p.style.display = p.style.display === 'none' ? 'block' : 'none'; };
  document.addEventListener('click', function cSelOut(e) {
    const p = $('#cSelPanel');
    if (p && p.style.display !== 'none' && !e.target.closest('.cselect')) { p.style.display = 'none'; }
  });
  $('#cSelSearch').oninput = e => secRenderCountryList(e.target.value);
  $('#secSave').onclick = async () => {
    try {
      await api('PUT', '/api/settings', {
        sec_enabled: $('#secEnabled').checked ? '1' : '0',
        sec_countries: secSelCountries.join(','),
        sec_ip_whitelist: $('#secWl').value,
        sec_ip_blacklist: $('#secBl').value,
        sec_tg_alert: $('#secTgAlert').checked ? '1' : '0',
      });
      toast('已保存'); renderSecSettings();
    } catch (e) { toast(e.message, true); }
  };
  $('#geoUpBtn').onclick = async () => {
    const btn = $('#geoUpBtn'); btn.disabled = true; btn.textContent = '下载中…';
    try {
      const r = await api('POST', '/api/settings/security/geoip-update', {}, { noReload: true });
      toast('IP 库已更新（' + r.month + '）'); renderSecSettings();
    } catch (e) { toast(e.message, true); btn.disabled = false; btn.textContent = '更新 IP 库'; }
  };
  render2FACard();
  renderBans();
}

async function renderBans() {
  const el = $('#banList');
  if (!el) return;
  let data;
  try { data = await api('GET', '/api/settings/security/bans'); }
  catch (e) { el.innerHTML = `<p class="danger-t">加载失败：${esc(e.message)}</p>`; return; }
  const rows = data.bans || [];
  $('#banCount').textContent = rows.length ? rows.length + ' 个' : '无';
  el.innerHTML = rows.length ? rows.map(b =>
    `<div class="trow"><code>${esc(b.ip)}</code><span class="muted">${esc(b.reason || '')} · ${fmtDate(b.time)}</span><button class="btn ghost sm" data-unban="${esc(b.ip)}">解封</button></div>`).join('')
    : `<p class="muted" style="font-size:13px">暂无封禁 IP</p>`;
  $$('#banList [data-unban]').forEach(btn => btn.onclick = async () => {
    if (!confirm('解封 ' + btn.dataset.unban + '？')) return;
    try {
      await api('DELETE', '/api/settings/security/bans', { ip: btn.dataset.unban }, { noReload: true });
      toast('已解封'); renderBans();
    } catch (e) { toast(e.message, true); }
  });
  $('#banAddBtn').onclick = async () => {
    const ip = $('#banIpInput').value.trim();
    if (!ip) return;
    try {
      await api('POST', '/api/settings/security/bans', { ip }, { noReload: true });
      $('#banIpInput').value = '';
      toast('已封禁 ' + ip); renderBans();
    } catch (e) { toast(e.message, true); }
  };
}

async function render2FACard() {
  const el = $('#sec2faBody');
  let st;
  try { st = await api('GET', '/api/settings/2fa/status'); }
  catch (e) { el.innerHTML = `<p class="danger-t">加载失败：${esc(e.message)}</p>`; return; }
  if (!st.enabled) {
    el.innerHTML = `
      <p class="muted" style="font-size:13px">开启后：<b>新 IP</b> 首次登录除了账号密码，还要输入验证器 App 上的 6 位动态码；同一个 IP 只验证一次。换 IP 登录会先掉线并要求重新验证。</p>
      ${st.hasPending ? `<p class="warn-t" style="font-size:13px">有一次未完成的开通流程，继续会重新生成密钥。</p>` : ''}
      <button class="btn" id="tfaOn">${ic('lock')} 开启两步验证</button>`;
    $('#tfaOn').onclick = totpSetupFlow;
    return;
  }
  const rows = (st.trusted || []).map(t =>
    `<div class="trow"><code>${esc(t.ip)}</code><span class="muted">${t.country ? esc(secCountryName(t.country)) + ' · ' : ''}${fmtDate(t.since)} 验证</span></div>`).join('')
    || `<p class="muted" style="font-size:13px">暂无</p>`;
  el.innerHTML = `
    <p><span class="tag">已启用</span> <span class="muted" style="font-size:13px">新 IP 登录需要动态码；已验证过的 IP 不再打扰。</span></p>
    <div class="field"><label>已信任的 IP（首次验证通过后记住）</label><div class="tlist">${rows}</div></div>
    <div style="display:flex;gap:10px;flex-wrap:wrap">
      <button class="btn ghost sm" id="tfaClearIp">清除已信任 IP</button>
      <button class="btn ghost sm" id="tfaOff" style="color:var(--danger)">关闭两步验证</button>
    </div>`;
  $('#tfaClearIp').onclick = async () => {
    if (!confirm('清除后，所有 IP 下次登录都要重新验证动态码。确定？')) return;
    try { await api('DELETE', '/api/settings/security/trusted', null, { noReload: true }); toast('已清除'); render2FACard(); }
    catch (e) { toast(e.message, true); }
  };
  $('#tfaOff').onclick = () => {
    openModal(`<h3>关闭两步验证</h3>
      <p class="muted" style="font-size:13px">关闭后登录只靠账号密码。请三思。</p>
      <div class="field"><label>输入登录密码确认</label><input id="tfaOffPw" type="password" autocomplete="current-password"></div>
      <div id="tfaOffErr" class="danger-t"></div>
      <div class="foot"><button class="btn ghost" data-close>取消</button><button class="btn" id="tfaOffGo" style="background:var(--danger)">确认关闭</button></div>`);
    $('#tfaOffGo').onclick = async () => {
      try {
        await api('POST', '/api/settings/2fa/disable', { password: $('#tfaOffPw').value }, { noReload: true });
        closeModal(); toast('两步验证已关闭'); render2FACard();
      } catch (e) { $('#tfaOffErr').textContent = e.message; }
    };
  };
}

async function totpSetupFlow() {
  let r;
  try { r = await api('POST', '/api/settings/2fa/setup', {}, { noReload: true }); }
  catch (e) { toast(e.message, true); return; }
  openModal(`<h3>开启两步验证</h3>
    <p class="muted" style="font-size:13px">用验证器 App（Google Authenticator / 微软 Authenticator / 1Password 等）扫描二维码，或手动输入密钥。</p>
    <div style="text-align:center;margin:8px 0"><img src="${r.qr}" alt="2FA QR" style="width:200px;height:200px;border:1px solid var(--line);border-radius:8px"></div>
    <div class="field"><label>密钥（手动输入用）</label>
      <div style="display:flex;gap:8px"><input id="totpSecret" value="${esc(r.secret)}" readonly style="flex:1;font-family:monospace"><button class="btn ghost sm" id="totpCopy" type="button">复制</button></div></div>
    <div class="field"><label>恢复码（<b style="color:var(--danger)">只显示这一次</b>，请抄到安全的地方；手机丢了靠它登录）</label>
      <div class="recovery-codes">${r.recoveryCodes.map(c => `<code>${esc(c)}</code>`).join('')}</div></div>
    <div class="field"><label>输入 App 上的 6 位动态码，确认没错才算开通成功</label>
      <input id="totpCode" inputmode="numeric" autocomplete="one-time-code" maxlength="6" placeholder="6 位数字" style="font-size:20px;letter-spacing:8px;text-align:center"></div>
    <div id="totpErr" class="danger-t" style="min-height:20px"></div>
    <div class="foot"><button class="btn ghost" data-close>取消</button><button class="btn" id="totpConfirm">验证并开启</button></div>`, true, true);
  $('#totpCopy').onclick = () => copyText($('#totpSecret').value);
  $('#totpCode').focus();
  $('#totpConfirm').onclick = async () => {
    $('#totpErr').textContent = '';
    try {
      await api('POST', '/api/settings/2fa/confirm', { code: $('#totpCode').value.trim() }, { noReload: true });
      closeModal(); toast('两步验证已开启'); render2FACard();
    } catch (e) { $('#totpErr').textContent = e.message; }
  };
  $('#totpCode').onkeydown = e => { if (e.key === 'Enter') $('#totpConfirm').click(); };
}

async function renderBrowserSettings() {
  const s = await api('GET', '/api/settings');
  $('#setBody').innerHTML = `
  <div class="card"><h3>${ic('globe')} 远程浏览器</h3>
    <p class="muted" style="font-size:13px">在别的服务器上部署 neko（网页版浏览器），填入地址和密码并启用后，侧边栏顶部会出现「远程浏览器」菜单。浏览器跑在远程服务器上，用的是那边的网络出口。点开菜单自动登录，无需再次输入密码。</p>
    <label class="check"><input type="checkbox" id="brEn" ${s.browser_enabled === '1' ? 'checked' : ''}> 启用远程浏览器</label>
    <div class="field"><label>浏览器地址（https:// 开头）</label>
      <input id="brUrl" value="${esc(s.browser_url || '')}" placeholder="https://neko.example.com"></div>
    <div class="field"><label>浏览器密码（neko 登录密码，加密存储）</label>
      <input id="brPwd" type="password" autocomplete="off" value="${esc(s.browserPwd || '')}" placeholder="neko 的用户密码"></div>
    <button class="btn" id="brSave">${ic('check')} 保存</button>
  </div>`;
  $('#brSave').onclick = async () => {
    const url = $('#brUrl').value.trim();
    const pwd = $('#brPwd').value;
    if ($('#brEn').checked && !url) { toast('请先填写浏览器地址', true); return; }
    if (url && !/^https?:\/\//i.test(url)) { toast('地址必须以 http(s):// 开头', true); return; }
    try {
      await api('PUT', '/api/settings', {
        browser_enabled: $('#brEn').checked ? '1' : '0',
        browser_url: url,
        browserPwd: pwd,
      });
      toast('已保存');
      await loadBrowserCfg();
      $('#app').innerHTML = shell('');
      bindShell();
      await renderSettings();
    } catch (e) { toast('保存失败：' + e.message, true); }
  };
}

/* ---------- sidebar inline menu editing ---------- */
function editModeItems() {
  const byId = {};
  for (const m of MENU_ITEMS) byId[m.id] = m;
  return menuCfg.order.map(id => byId[id]).filter(m => m && !m.foot);
}
function toggleFolder(id) {
  if (openFolders.has(id)) openFolders.delete(id); else openFolders.add(id);
  try { localStorage.setItem('sd-open-folders', JSON.stringify([...openFolders])); } catch (e) {}
  renderSidebar();
}
async function fetchMenuIcon(id, url) {
  try {
    await api('POST', '/api/menu-icon', { id, url });
    await loadBrowserCfg();
    renderSidebar();
  } catch (e) { /* keep the generic icon */ }
}
let iconBackfillDone = false;
async function backfillMenuIcons() {
  if (iconBackfillDone) return;
  iconBackfillDone = true;
  const missing = menuCfg.links.filter(l => !l.icon);
  for (const l of missing.slice(0, 30)) {
    try { await api('POST', '/api/menu-icon', { id: l.id, url: l.url }); } catch (e) {}
  }
  if (missing.length) { await loadBrowserCfg(); renderSidebar(); }
}
function toggleEditMode() {
  state.editMode = !state.editMode;
  if (state.editMode) {
    state.sbCollapsed = false; // editing needs room; don't persist this
  } else {
    try { state.sbCollapsed = localStorage.getItem('sd-sb-collapsed') === '1'; } catch (e) { state.sbCollapsed = false; }
  }
  renderSidebar();
}
function renderSidebar() {
  const tmp = document.createElement('div');
  tmp.innerHTML = shell('');
  $('.sidebar').replaceWith(tmp.firstElementChild);
  bindShell();
}
async function menuEditSave() {
  try {
    await api('PUT', '/api/settings', {
      menu_order: JSON.stringify(menuCfg.order.filter(id => !MENU_FOOT_IDS.includes(id))),
      menu_hidden: JSON.stringify(menuCfg.hidden),
      menu_links: JSON.stringify(menuCfg.links),
      menu_folders: JSON.stringify(menuCfg.folders),
      menu_custom_order: JSON.stringify(menuCfg.customOrder),
    });
    await loadBrowserCfg();
    renderSidebar();
  } catch (e) { toast('保存失败：' + e.message, true); }
}
function swapIn(arr, id, dir) {
  const i = arr.indexOf(id);
  const j = i + dir;
  if (i < 0 || j < 0 || j >= arr.length) return false;
  [arr[i], arr[j]] = [arr[j], arr[i]];
  return true;
}
function menuEditAct(act, id) {
  const link = menuCfg.links.find(x => x.id === id);
  const folder = menuCfg.folders.find(x => x.id === id);
  if (act === 'edit' && link) { openLinkDialog(link); return; }
  if (act === 'rename' && folder) { openFolderDialog(folder); return; }
  if (act === 'delfolder' && folder) { confirmDeleteFolder(folder); return; }
  if (act === 'eye') {
    const h = menuCfg.hidden.indexOf(id);
    if (h >= 0) menuCfg.hidden.splice(h, 1); else menuCfg.hidden.push(id);
    menuEditSave();
    return;
  }
  if (act !== 'up' && act !== 'down') return;
  const dir = act === 'up' ? -1 : 1;
  if (folder || (link && !link.folder)) {
    if (!swapIn(menuCfg.customOrder, id, dir)) return;
  } else if (link && link.folder) {
    const sibs = menuCfg.links.filter(l => l.folder === link.folder);
    const i = sibs.findIndex(l => l.id === link.id);
    const j = i + dir;
    if (j < 0 || j >= sibs.length) return;
    const a = menuCfg.links.indexOf(sibs[i]);
    const b = menuCfg.links.indexOf(sibs[j]);
    [menuCfg.links[a], menuCfg.links[b]] = [menuCfg.links[b], menuCfg.links[a]];
  } else {
    if (!swapIn(menuCfg.order, id, dir)) return;
  }
  menuEditSave();
}
function openFolderDialog(folder) {
  openModal(`<h3>${folder ? '重命名文件夹' : '新建文件夹'}</h3>
    <div class="field"><label>名称</label><input id="fldName" value="${esc(folder ? folder.name : '')}" maxlength="40" placeholder="文件夹名字"></div>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="fldOk">${ic('check')} 确定</button></div>`);
  $('#fldOk').onclick = () => {
    const name = $('#fldName').value.trim();
    if (!name) { toast('请填写名称', true); return; }
    if (folder) { folder.name = name.slice(0, 40); }
    else {
      const f = { id: 'fld-' + Date.now(), name: name.slice(0, 40) };
      menuCfg.folders.push(f);
      menuCfg.customOrder.push(f.id);
    }
    closeModal();
    menuEditSave();
  };
}
function confirmDeleteFolder(folder) {
  const n = menuCfg.links.filter(l => l.folder === folder.id).length;
  openModal(`<h3>删除文件夹「${esc(folder.name)}」？</h3>
    <p class="muted">${n ? `里面的 ${n} 个链接会移到根目录，不会被删除。` : '文件夹是空的。'}</p>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="fldDel" style="color:var(--danger)">${ic('trash')} 删除</button></div>`);
  $('#fldDel').onclick = () => {
    menuCfg.links.filter(l => l.folder === folder.id).forEach(l => {
      l.folder = '';
      if (!menuCfg.customOrder.includes(l.id)) menuCfg.customOrder.push(l.id);
    });
    menuCfg.folders = menuCfg.folders.filter(x => x.id !== folder.id);
    menuCfg.customOrder = menuCfg.customOrder.filter(x => x !== folder.id);
    menuCfg.hidden = menuCfg.hidden.filter(x => x !== folder.id);
    closeModal();
    menuEditSave();
  };
}
function openLinkDialog(link) {
  const folderOpts = `<option value="">根目录</option>` + menuCfg.folders.map(f =>
    `<option value="${f.id}"${link && link.folder === f.id ? ' selected' : ''}>${esc(f.name)}</option>`).join('');
  openModal(`<h3>${link ? '编辑链接' : '添加链接'}</h3>
    <div class="field"><label>从我的网址里选（可选）</label><select id="lkdPick" style="width:100%"><option value="">加载中…</option></select></div>
    <div class="field"><label>名称</label><input id="lkdName" value="${esc(link ? link.label : '')}" placeholder="菜单显示的名字"></div>
    <div class="field"><label>网址（http(s):// 开头）</label><input id="lkdUrl" value="${esc(link ? link.url : '')}" placeholder="https://example.com"></div>
    <div class="row2">
      <div class="field"><label>打开方式</label><select id="lkdMode" style="width:100%">
        <option value="embed"${!link || link.mode !== 'new' ? ' selected' : ''}>站内右侧打开</option>
        <option value="new"${link && link.mode === 'new' ? ' selected' : ''}>新窗口打开</option>
      </select></div>
      <div class="field"><label>所在文件夹</label><select id="lkdFolder" style="width:100%">${folderOpts}</select></div>
    </div>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    ${link ? '<button class="btn ghost" id="lkdDel" style="color:var(--danger)">删除</button>' : ''}
    <button class="btn" id="lkdOk">${ic('check')} 确定</button></div>`);
  api('GET', '/api/links').then(list => {
    const sel = $('#lkdPick');
    if (!sel) return;
    const items = Array.isArray(list) ? list : [];
    if (!items.length) { sel.innerHTML = '<option value="">网址里还没有保存的网址</option>'; return; }
    sel.innerHTML = '<option value="">— 选择一个已保存的网址 —</option>' + items.map((l, i) => `<option value="${i}">${esc(l.name || l.url)}</option>`).join('');
    sel.onchange = () => {
      if (sel.value === '') return;
      const it = items[Number(sel.value)];
      if (!it) return;
      $('#lkdName').value = String(it.name || '').slice(0, 40);
      let u = String(it.url || '').trim();
      if (u && !/^https?:\/\//i.test(u)) u = 'https://' + u;
      $('#lkdUrl').value = u;
    };
  }).catch(() => { const sel = $('#lkdPick'); if (sel) sel.innerHTML = '<option value="">加载失败</option>'; });
  $('#lkdOk').onclick = () => {
    const label = $('#lkdName').value.trim();
    const url = $('#lkdUrl').value.trim();
    const mode = $('#lkdMode').value === 'new' ? 'new' : 'embed';
    const folderVal = $('#lkdFolder').value;
    const folder = menuCfg.folders.some(f => f.id === folderVal) ? folderVal : '';
    if (!label) { toast('请填写名称', true); return; }
    if (!/^https?:\/\//i.test(url)) { toast('网址必须以 http(s):// 开头', true); return; }
    if (link) {
      const wasRoot = !link.folder;
      link.label = label; link.url = url; link.mode = mode; link.folder = folder;
      if (wasRoot && folder) menuCfg.customOrder = menuCfg.customOrder.filter(x => x !== link.id);
      if (!wasRoot && !folder && !menuCfg.customOrder.includes(link.id)) menuCfg.customOrder.push(link.id);
    } else {
      const nl = { id: 'lnk-' + Date.now(), label, url, mode, folder };
      menuCfg.links.push(nl);
      if (!folder) menuCfg.customOrder.push(nl.id);
    }
    closeModal();
    const _id = link ? link.id : menuCfg.links[menuCfg.links.length - 1].id;
    const _url = url;
    menuEditSave().then(() => fetchMenuIcon(_id, _url));
  };
  const del = $('#lkdDel');
  if (del) del.onclick = () => {
    menuCfg.links = menuCfg.links.filter(x => x.id !== link.id);
    menuCfg.customOrder = menuCfg.customOrder.filter(x => x !== link.id);
    menuCfg.hidden = menuCfg.hidden.filter(x => x !== link.id);
    closeModal();
    menuEditSave();
  };
}





async function renderBackupSettings() {
  const s = await api('GET', '/api/settings');
  $('#setBody').innerHTML = `
  <div class="card"><h3>${ic('download')} 手动备份 / 恢复</h3>
    <p class="muted" style="font-size:13px">备份文件用你设的密码加密，包含机密、脚本、网址、笔记、订阅节点、提醒、SSH、TG配置。密码只有你知道，丢了打不开。</p>
    <div class="row2">
      <div class="field"><label>备份密码</label><input id="bkPass" type="password" autocomplete="off"></div>
      <div class="field"><label>恢复：选择备份文件</label><input id="bkFile" type="file" accept=".json"></div>
    </div>
    <div style="display:flex;gap:16px;margin-bottom:12px">
      <label class="check"><input type="checkbox" id="bkImg"> 包含图片</label>
      <label class="check"><input type="checkbox" id="bkFileChk"> 包含文件</label>
    </div>
    <div style="display:flex;gap:10px;flex-wrap:wrap">
      <button class="btn" id="bkExport">${ic('download')} 导出备份</button>
      <button class="btn ghost" id="bkImport">${ic('upload')} 导入恢复</button>
    </div>
  </div>
  <div class="card"><h3>${ic('clock')} 定时本地备份</h3>
    <label class="check"><input type="checkbox" id="bkEn" ${s.backup_enabled === '1' ? 'checked' : ''}> 启用定时备份</label>
    <div class="row2">
      <div class="field"><label>每天备份时间</label><input id="bkTime" value="${esc(s.backup_time || '03:00')}" placeholder="03:00"></div>
      <div class="field"><label>保留最近几份</label><input id="bkKeep" type="number" min="1" value="${esc(s.backup_keep || '7')}"></div>
    </div>
    <div class="field"><label>备份目录（挂载的数据卷路径，如 /backup）</label>
      <input id="bkDir" value="${esc(s.backup_dir || '')}" placeholder="/backup"></div>
    <div style="display:flex;gap:16px;margin:12px 0">
      <label class="check"><input type="checkbox" id="bkSchedImg" ${s.backup_include_images === '1' ? 'checked' : ''}> 包含图片</label>
      <label class="check"><input type="checkbox" id="bkSchedFile" ${s.backup_include_files === '1' ? 'checked' : ''}> 包含文件</label>
    </div>
    <button class="btn" id="bkSave">${ic('check')} 保存备份设置</button>
    <div class="hint" style="margin-top:8px">定时备份用主密钥加密，恢复需要主密钥文件（<code>.master_key</code>），请一并保管好。</div>
  </div>`;
  $('#bkExport').onclick = async () => {
    const pw = $('#bkPass').value;
    if (!pw) { toast('请先输入备份密码', true); return; }
    try {
      const r = await fetch('/api/backup/export', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          password: pw,
          includeImages: $('#bkImg').checked,
          includeFiles: $('#bkFileChk').checked,
        }),
      });
      if (!r.ok) { const d = await r.json().catch(() => ({})); throw new Error(d.error || '导出失败'); }
      const blob = await r.blob();
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = 'secretdock-backup-' + new Date().toISOString().slice(0, 10) + '.enc.json';
      a.click();
      toast('备份已下载');
    } catch (e) { toast(e.message, true); }
  };
  $('#bkImport').onclick = async () => {
    const pw = $('#bkPass').value;
    const f = $('#bkFile').files[0];
    if (!pw || !f) { toast('请选择文件并输入备份密码', true); return; }
    const fd = new FormData();
    fd.append('file', f); fd.append('password', pw);
    try {
      const r = await fetch('/api/backup/import', { method: 'POST', body: fd });
      const d = await r.json();
      if (!r.ok) throw new Error(d.error || '导入失败');
      toast('已导入 ' + d.imported + ' 条机密');
      await refreshMerchants(); renderView();
    } catch (e) { toast(e.message, true); }
  };
  $('#bkSave').onclick = async () => {
    try {
      await api('PUT', '/api/settings', {
        backup_enabled: $('#bkEn').checked ? '1' : '',
        backup_time: $('#bkTime').value.trim(),
        backup_keep: $('#bkKeep').value.trim(),
        backup_dir: $('#bkDir').value.trim(),
        backup_include_images: $('#bkSchedImg').checked ? '1' : '',
        backup_include_files: $('#bkSchedFile').checked ? '1' : '',
      });
      toast('已保存');
    } catch (e) { toast(e.message, true); }
  };
}
async function renderChangePwd() {
  $('#setBody').innerHTML = `
  <div class="card"><h3>${ic('lock')} 修改密码</h3>
    <div class="field"><label>当前密码</label><input id="cpOld" type="password" autocomplete="current-password"></div>
    <div class="field"><label>新密码</label><input id="cpNew" type="password" autocomplete="new-password" placeholder="大写+小写+数字，11位以上"></div>
    <div class="field"><label>重复新密码</label><input id="cpNew2" type="password" autocomplete="new-password" placeholder="再次输入新密码"></div>
    <div id="cpErr" class="danger-t" style="font-size:13px;min-height:20px;margin-bottom:6px"></div>
    <button class="btn" id="cpGo">确认修改</button>
    <p class="muted" style="font-size:12px;margin-top:12px">修改后将退出登录，需用新密码重新登录</p>
  </div>`;
  $('#cpGo').onclick = async () => {
    $('#cpErr').textContent = '';
    const oldP = $('#cpOld').value;
    const p1 = $('#cpNew').value;
    const p2 = $('#cpNew2').value;
    if (p1 !== p2) { $('#cpErr').textContent = '两次新密码不一致'; return; }
    try {
      await api('POST', '/api/change-password', { oldPassword: oldP, newPassword: p1 });
      toast('密码已修改，请重新登录');
      setTimeout(() => location.reload(), 800);
    } catch (e) { $('#cpErr').textContent = e.message; }
  };
}

/* ---------- 清理数据 ---------- */
async function renderClearData() {
  const v = $('#setBody');
  const cats = [
    { key: 'merchants', name: '机密库' },
    { key: 'scripts', name: '脚本' },
    { key: 'links', name: '网址' },
    { key: 'notes', name: '笔记' },
    { key: 'images', name: '图片' },
    { key: 'files', name: '文件' },
    { key: 'nodes', name: '订阅节点' },
    { key: 'reminders', name: '提醒' },
    { key: 'ssh', name: 'SSH' },
    { key: 'tg', name: 'TG 配置' },
    { key: 'audit', name: '安全日志' },
  ];
  v.innerHTML = `<div class="card" style="padding:16px">
    <div class="muted" style="font-size:13px;margin-bottom:12px">清空后无法恢复，请先备份。</div>
    ${cats.map(c => `
      <div class="lrow">
        <div class="grow"><div class="lab">${c.name}</div></div>
        <button class="btn ghost sm danger" data-clear="${c.key}">清空</button>
      </div>`).join('')}
    <div class="lrow" style="margin-top:8px;border-top:1px solid var(--border);padding-top:12px">
      <div class="grow"><div class="lab" style="color:var(--danger);font-weight:600">全部数据</div>
      <div class="smeta">清空所有菜单的数据（含设置）</div></div>
      <button class="btn danger sm" data-clear="all">全部清空</button>
    </div>
  </div>`;
  $$('[data-clear]', v).forEach(b => b.onclick = () => {
    const key = b.dataset.clear;
    const name = key === 'all' ? '全部数据' : cats.find(c => c.key === key).name;
    openModal(`<h3>确认清空？</h3>
      <p>将清空「${name}」的所有数据，<b style="color:var(--danger)">无法恢复</b>。</p>
      <div class="frow"><input id="cfConfirm" placeholder="输入 YES 确认"></div>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn danger" id="cfDoClear">确认清空</button></div>`);
    $('#cfDoClear').onclick = async () => {
      if ($('#cfConfirm').value.trim() !== 'YES') { toast('请输入 YES 确认'); return; }
      await api('POST', '/api/data/clear', { key });
      closeModal(); toast('已清空');
    };
  });
}

async function renderAudit() {
  let list = [];
  try { list = await api('GET', '/api/audit?limit=100'); } catch (e) {}
  const kindName = { login: '登录成功', login_fail: '登录失败', login_locked: '登录锁定', tg_query: 'TG 查询', tg_add: 'TG 添加', settings: '设置变更', backup_export: '备份导出', backup_import: '备份导入' };
  $('#setBody').innerHTML = `<div class="card"><h3>${ic('shield')} 安全日志（最近 100 条）</h3>
    ${list.length ? list.map(a => `
      <div class="audit-row"><span class="t">${new Date(a.createdAt * 1000).toLocaleString('zh-CN')}</span>
      <span><b>${esc(kindName[a.kind] || a.kind)}</b> <span class="muted">· ${esc(a.detail || "-")} · ${esc(a.ip)}</span></span></div>`).join('')
      : '<div class="empty">暂无记录</div>'}
  </div>`;
}

/* ---------- modal ---------- */
function openModal(html, wide, noBgClose, modalCls) {
  $('#modal-root').innerHTML = `<div class="modal-bg"><div class="modal${wide ? ' wide' : ''}${modalCls ? ' ' + modalCls : ''}">${html}</div></div>`;
  const bg = $('.modal-bg');
  if (!noBgClose) bg.onclick = e => { if (e.target === bg) closeModal(); };
  $$('[data-close]').forEach(b => b.onclick = closeModal);
  if (!noBgClose) document.onkeydown = e => { if (e.key === 'Escape') closeModal(); };
}
function closeModal() {
  $('#modal-root').innerHTML = '';
  document.onkeydown = null;
}

function openScriptModal(sc) {
  openModal(`<h3>${esc(sc.name || '脚本')}</h3>
    <div id="scView">
      <pre class="codeview">${esc(sc.content || '')}</pre>
      <div class="foot"><button class="btn ghost" data-close>关闭</button>
      <button class="btn ghost" id="scCopy">复制</button>
      <button class="btn" id="scEdit">编辑</button></div>
    </div>
    <div id="scEditBox" style="display:none">
      <div class="frow"><label>名称</label><input id="scName" value="${esc(sc.name || '')}" maxlength="60"></div>
      <textarea id="scContent" class="notearea codeview" style="min-height:260px">${esc(sc.content || '')}</textarea>
      <div class="foot"><button class="btn ghost" id="scCancel">取消</button>
      <button class="btn" id="scSave">保存</button></div>
    </div>`);
  $('#scCopy').onclick = () => copyText(sc.content || '');
  $('#scEdit').onclick = () => { $('#scView').style.display = 'none'; $('#scEditBox').style.display = ''; };
  $('#scCancel').onclick = () => { $('#scEditBox').style.display = 'none'; $('#scView').style.display = ''; };
  $('#scSave').onclick = async () => {
    await api('PUT', '/api/scripts/' + sc.id, {
      name: $('#scName').value.trim(),
      content: $('#scContent').value,
    });
    closeModal(); toast('已保存'); renderScripts();
  };
}

/* ---------- links view ---------- */
let linkState = { cats: [], cur: 0, sel: new Set(), selectMode: false, sort: 'new' };
let nodeState = { cats: [], cur: 0, list: [], testing: false };
async function renderLinks() {
  const v = $('#view');
  v.innerHTML = `
    <div class="topbar">
      <h2>网址</h2>
      <span style="flex:1"></span>
      <span class="muted" style="font-size:12px">TG 里第一行发"网址"粘贴入库</span>
      ${sortSelHTML('new')}
      <button class="btn ghost sm" id="selMode">${ic('check')} 选择</button>
      <button class="btn sm" id="addL">${ic('plus')} 添加网址</button>
    </div>
    <div class="catbar" id="catBar"></div>
    <div id="batchBar" style="display:none"></div>
    <div id="linkList"><div class="empty"><div>加载中…</div></div></div>`;
  $('#addL').onclick = () => linkModal(null);
  $('.sortsel').onchange = e => { linkState.sort = e.target.value; drawLinks(); };
  $('#selMode').onclick = () => {
    linkState.selectMode = !linkState.selectMode;
    linkState.sel = new Set();
    $('#selMode').innerHTML = linkState.selectMode ? `${ic('x')} 取消选择` : `${ic('check')} 选择`;
    drawLinks();
  };
  try { linkState.cats = await api('GET', '/api/link-categories'); } catch (e) { linkState.cats = []; }
  if (!linkState.cats.length) linkState.cats = [{ id: 0, name: '未分类' }];
  if (linkState.cur !== 0 && !linkState.cats.find(c => c.id === linkState.cur)) linkState.cur = linkState.cats[0].id;
  linkState.sel = new Set();
  drawCatBar();
  await drawLinks();
}
function drawCatBar() {
  const bar = $('#catBar');
  const allCats = [{ id: 0, name: '全部' }, ...linkState.cats];
  bar.innerHTML = allCats.map(c =>
    `<button class="pill ${c.id === linkState.cur ? 'active' : ''}" data-id="${c.id}">${esc(c.name)}</button>`
  ).join('') + `<button class="pill ghost" id="addCat">${ic('plus')} 分类</button>`;
  $$('.pill[data-id]', bar).forEach(b => b.onclick = () => {
    linkState.cur = +b.dataset.id; linkState.sel = new Set(); drawCatBar(); drawLinks();
  });
  $('#addCat').onclick = () => {
    openModal(`<h3>新建分类</h3>
      <div class="frow"><input id="catName" placeholder="分类名称" maxlength="20"></div>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn" id="cfAdd">创建</button></div>`);
    $('#cfAdd').onclick = async () => {
      const name = $('#catName').value.trim();
      if (!name) return;
      const r = await api('POST', '/api/link-categories', { name });
      closeModal(); toast('已创建');
      linkState.cats = await api('GET', '/api/link-categories');
      linkState.cur = r.id; linkState.sel = new Set();
      drawCatBar(); drawLinks();
    };
  };
  // 分类管理（重命名/删除当前分类，未分类除外）
  const cur = linkState.cats.find(c => c.id === linkState.cur);
  if (cur && cur.name !== '未分类') {
    const mg = document.createElement('span');
    mg.style.marginLeft = '8px';
    mg.innerHTML = `<button class="icon-btn" id="renCat" title="重命名分类">${ic('edit')}</button>
      <button class="icon-btn" id="delCat" title="删除分类（网址移到未分类）">${ic('trash')}</button>`;
    bar.appendChild(mg);
    $('#renCat').onclick = () => {
      openModal(`<h3>重命名分类</h3>
        <div class="frow"><input id="catName" value="${esc(cur.name)}" maxlength="20"></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfRen">保存</button></div>`);
      $('#cfRen').onclick = async () => {
        const name = $('#catName').value.trim();
        if (!name) return;
        await api('PUT', '/api/link-categories/' + cur.id, { name });
        closeModal(); toast('已重命名');
        linkState.cats = await api('GET', '/api/link-categories');
        drawCatBar(); drawLinks();
      };
    };
    $('#delCat').onclick = () => {
      openModal(`<h3>删除分类「${esc(cur.name)}」？</h3><p class="muted">里面的网址会移到"未分类"。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/link-categories/' + cur.id);
        closeModal(); toast('已删除');
        linkState.cats = await api('GET', '/api/link-categories');
        linkState.cur = linkState.cats[0].id; linkState.sel = new Set();
        drawCatBar(); drawLinks();
      };
    };
  }
}
async function drawLinks() {
  const box = $('#linkList');
  let list = [];
  try { list = await api('GET', '/api/links?categoryId=' + linkState.cur); } catch (e) {}
  list = sortItems(list, linkState.sort);
  if (!list.length) {
    box.innerHTML = `<div class="empty">${ic('link')}<div>这个分类还没有网址</div></div>`;
    drawBatchBar();
    return;
  }
  box.innerHTML = `<div class="card" style="padding:4px 14px">` + list.map(l => `
    <div class="lrow" data-id="${l.id}">
      ${linkState.selectMode ? `<input type="checkbox" class="lsel" data-id="${l.id}" ${linkState.sel.has(l.id) ? 'checked' : ''}>` : ''}
      <div class="grow" style="min-width:0">
        <div class="lab">${l.isPinned ? '<span class="pinmark">📌 </span>' : ''}${esc(l.name || '（未命名）')}${l.autoName ? ' <span class="tag">自检名称</span>' : ''}</div>
        <div class="lurl" data-url="${esc(l.url)}" title="点击复制">${esc(l.url)}</div>
      </div>
      <div class="acts">
        <button class="icon-btn" data-act="preview" title="预览">${ic('eye')}</button>
        ${pinBtnHTML('links', l)}
        <button class="icon-btn" data-act="edit" title="编辑">${ic('edit')}</button>
        <button class="icon-btn" data-act="del" title="删除">${ic('trash')}</button>
      </div>
    </div>`).join('') + `</div>`;
  $$('.lsel', box).forEach(cb => cb.onchange = () => {
    const id = +cb.dataset.id;
    cb.checked ? linkState.sel.add(id) : linkState.sel.delete(id);
    drawBatchBar();
  });
  $$('.lurl', box).forEach(el => el.onclick = () => copyText(el.dataset.url));
  bindPinBtns(box, 'links', drawLinks);
  $$('.lrow', box).forEach(row => {
    const id = +row.dataset.id;
    const l = list.find(x => x.id === id);
    $('[data-act="preview"]', row).onclick = () => openLinkPreview(l);
    $('[data-act="edit"]', row).onclick = () => startInlineEdit(row, l);
    $('[data-act="del"]', row).onclick = () => {
      openModal(`<h3>删除这个网址？</h3><p class="muted">删除后无法恢复。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/links/' + id);
        linkState.sel.delete(id);
        closeModal(); toast('已删除'); drawLinks();
      };
    };
  });
  drawBatchBar();
}
function drawBatchBar() {
  const bar = $('#batchBar');
  if (!linkState.selectMode) { bar.style.display = 'none'; bar.innerHTML = ''; return; }
  const n = linkState.sel.size;
  bar.style.display = '';
  const others = linkState.cats.filter(c => c.id !== linkState.cur);
  bar.innerHTML = `<div class="batchbar">
    <label style="display:flex;align-items:center;gap:6px;cursor:pointer">
      <input type="checkbox" id="selAll" class="lsel"> 全选</label>
    <span>已选 ${n} 项</span>
    <select id="moveTo">${others.map(c => `<option value="${c.id}">${esc(c.name)}</option>`).join('')}</select>
    <button class="btn sm" id="doMove">移动到该分类</button>
    <button class="btn ghost sm danger-t" id="doDelSel">删除所选</button>
  </div>`;
  $('#selAll').onclick = e => {
    const boxes = $$('.lsel', $('#linkList'));
    if (e.target.checked) boxes.forEach(cb => linkState.sel.add(+cb.dataset.id));
    else linkState.sel.clear();
    drawLinks();
  };
  $('#doMove').onclick = async () => {
    if (!linkState.sel.size) { toast('请先勾选网址'); return; }
    const cid = +$('#moveTo').value;
    await api('POST', '/api/links-move', { ids: [...linkState.sel], categoryId: cid });
    toast('已移动'); linkState.sel = new Set(); drawLinks();
  };
  $('#doDelSel').onclick = () => {
    if (!linkState.sel.size) { toast('请先勾选网址'); return; }
    openModal(`<h3>删除选中的 ${n} 个网址？</h3><p class="muted">删除后无法恢复。</p>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn danger" id="cfDel">删除</button></div>`);
    $('#cfDel').onclick = async () => {
      for (const id of linkState.sel) await api('DELETE', '/api/links/' + id);
      linkState.sel = new Set();
      closeModal(); toast('已删除'); drawLinks();
    };
  };
}
// 行内编辑网址：点击编辑直接在行内改，确认保存
function startInlineEdit(row, l) {
  if (!l || row.dataset.editing) return;
  row.dataset.editing = '1';
  const grow = $('.grow', row);
  const orig = grow.innerHTML;
  grow.innerHTML = `
    <div class="frow" style="margin:2px 0"><input id="ieName" value="${esc(l.name || '')}" placeholder="名称" maxlength="60"></div>
    <div class="frow" style="margin:2px 0"><input id="ieUrl" value="${esc(l.url || '')}" placeholder="https://…" inputmode="url"></div>`;
  const acts = $('.acts', row);
  const origActs = acts.innerHTML;
  acts.innerHTML = `
    <button class="btn sm" id="ieOk">${ic('check')} 确认</button>
    <button class="btn ghost sm" id="ieNo">取消</button>`;
  $('#ieNo', acts).onclick = () => { grow.innerHTML = orig; acts.innerHTML = origActs; delete row.dataset.editing; bindLinkRow(row, l); };
  $('#ieOk', acts).onclick = async () => {
    const name = $('#ieName', grow).value.trim(), url = $('#ieUrl', grow).value.trim();
    if (!url) { toast('网址不能为空'); return; }
    await api('PUT', '/api/links/' + l.id, { name, url, categoryId: l.categoryId });
    toast('已保存'); drawLinks();
  };
  $('#ieName', grow).focus();
}
// 重新绑定行按钮（行内编辑取消后用）
function bindLinkRow(row, l) {
  const id = l.id;
  $('[data-act="edit"]', row).onclick = () => startInlineEdit(row, l);
  $('[data-act="del"]', row).onclick = () => {
    openModal(`<h3>删除这个网址？</h3><p class="muted">删除后无法恢复。</p>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn danger" id="cfDel">删除</button></div>`);
    $('#cfDel').onclick = async () => {
      await api('DELETE', '/api/links/' + id);
      linkState.sel.delete(id);
      closeModal(); toast('已删除'); drawLinks();
    };
  };
  const urlEl = $('.lurl', row);
  if (urlEl) urlEl.onclick = () => copyText(urlEl.dataset.url);
}
function linkModal(l) {
  const isNew = !l;
  l = l || { name: '', url: '', categoryId: linkState.cur };
  const opts = linkState.cats.map(c =>
    `<option value="${c.id}" ${c.id === (l.categoryId || linkState.cur) ? 'selected' : ''}>${esc(c.name)}</option>`).join('');
  openModal(`<h3>${isNew ? '添加网址' : '编辑网址'}</h3>
    <div class="frow"><label>名称</label><input id="lName" value="${esc(l.name || '')}" placeholder="爱锋助手源" maxlength="60"></div>
    <div class="frow"><label>网址</label><input id="lUrl" value="${esc(l.url || '')}" placeholder="https://…" inputmode="url"></div>
    <div class="frow"><label>分类</label><select id="lCat">${opts}</select></div>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="cfSave">保存</button></div>`);
  $('#cfSave').onclick = async () => {
    const name = $('#lName').value.trim(), url = $('#lUrl').value.trim(), cid = +$('#lCat').value;
    if (!url) { toast('网址不能为空'); return; }
    if (isNew) await api('POST', '/api/links', { name, url, categoryId: cid });
    else await api('PUT', '/api/links/' + l.id, { name, url, categoryId: cid });
    closeModal(); toast('已保存');
    if (cid !== linkState.cur) { linkState.cur = cid; drawCatBar(); }
    drawLinks();
  };
}

/* ---------- notes view ---------- */
let noteState = { cats: [], cur: 0 };
async function renderNotes() {
  const v = $('#view');
  v.innerHTML = `
    <div class="topbar">
      <h2>笔记</h2>
      <span style="flex:1"></span>
      ${sortSelHTML('new')}
      <span class="muted" style="font-size:12px">TG 里第一行发"笔记"、第二行标题入库</span>
      <button class="btn sm" id="addN">${ic('plus')} 写笔记</button>
    </div>
    <div class="catbar" id="noteCatBar"></div>
    <div id="noteList"><div class="empty"><div>加载中…</div></div></div>`;
  $('#addN').onclick = () => openNoteModal(null);
  try { noteState.cats = await api('GET', '/api/note-categories'); } catch (e) { noteState.cats = []; }
  drawNoteCatBar();
  await loadNotes();
}
function drawNoteCatBar() {
  const bar = $('#noteCatBar');
  const cats = [{ id: 0, name: '全部' }, ...noteState.cats];
  bar.innerHTML = cats.map(c =>
    `<button class="pill ${c.id === noteState.cur ? 'active' : ''}" data-id="${c.id}">${esc(c.name)}</button>`
  ).join('') + `<button class="pill ghost" id="addNoteCat">${ic('plus')} 分类</button>`;
  $$('.pill[data-id]', bar).forEach(b => b.onclick = () => { noteState.cur = +b.dataset.id; drawNoteCatBar(); loadNotes(); });
  $('#addNoteCat').onclick = () => {
    openModal(`<h3>新建分类</h3>
      <div class="frow"><input id="catName" placeholder="分类名称" maxlength="20"></div>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn" id="cfAdd">创建</button></div>`);
    $('#cfAdd').onclick = async () => {
      const name = $('#catName').value.trim();
      if (!name) return;
      const r = await api('POST', '/api/note-categories', { name });
      closeModal(); toast('已创建');
      noteState.cats = await api('GET', '/api/note-categories');
      noteState.cur = r.id;
      drawNoteCatBar(); loadNotes();
    };
  };
  const cur = noteState.cats.find(c => c.id === noteState.cur);
  if (cur && cur.name !== '未分类') {
    const mg = document.createElement('span');
    mg.style.marginLeft = '8px';
    mg.innerHTML = `<button class="icon-btn" id="renNoteCat" title="重命名分类">${ic('edit')}</button>
      <button class="icon-btn" id="delNoteCat" title="删除分类（笔记移到未分类）">${ic('trash')}</button>`;
    bar.appendChild(mg);
    $('#renNoteCat').onclick = () => {
      openModal(`<h3>重命名分类</h3>
        <div class="frow"><input id="catName" value="${esc(cur.name)}" maxlength="20"></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfRen">保存</button></div>`);
      $('#cfRen').onclick = async () => {
        const name = $('#catName').value.trim();
        if (!name) return;
        await api('PUT', '/api/note-categories/' + cur.id, { name });
        closeModal(); toast('已重命名');
        noteState.cats = await api('GET', '/api/note-categories');
        drawNoteCatBar(); loadNotes();
      };
    };
    $('#delNoteCat').onclick = () => {
      openModal(`<h3>删除分类「${esc(cur.name)}」？</h3><p class="muted">里面的笔记会移到"未分类"。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/note-categories/' + cur.id);
        closeModal(); toast('已删除');
        noteState.cats = await api('GET', '/api/note-categories');
        noteState.cur = 0;
        drawNoteCatBar(); loadNotes();
      };
    };
  }
}
async function loadNotes() {
  let list = [];
  try { list = await api('GET', '/api/notes?categoryId=' + noteState.cur); } catch (e) {}
  const sortSel = $('.sortsel');
  const mode = sortSel ? sortSel.value : 'new';
  paintNotes(sortItems(list, mode));
  if (sortSel) sortSel.onchange = e => paintNotes(sortItems(list, e.target.value));
}
function paintNotes(list) {
  const box = $('#noteList');
  if (!list.length) {
    box.innerHTML = `<div class="empty">${ic('note')}<div>还没有笔记，去 TG 发一篇吧</div></div>`;
    return;
  }
  box.innerHTML = `<div class="card" style="padding:4px 14px">` + list.map(n => `
    <div class="lrow" data-id="${n.id}">
      <span class="licon">${ic('note')}</span>
      <div class="grow">
        <div class="lab">${n.isPinned ? '<span class="pinmark">📌 </span>' : ''}${esc(n.title || '（无标题）')}</div>
        <div class="smeta">${fmtDate(n.updatedAt || n.createdAt)}</div>
      </div>
      <div class="acts">
        <button class="btn ghost sm" data-act="open">${ic('eye')} 查看/编辑</button>
        <button class="icon-btn" data-act="move" title="移动分类">${ic('folder')}</button>
        ${pinBtnHTML('notes', n)}
        <button class="icon-btn" data-act="del" title="删除">${ic('trash')}</button>
      </div>
    </div>`).join('') + `</div>`;
  bindPinBtns(box, 'notes', loadNotes);
  $$('.lrow', box).forEach(row => {
    const id = +row.dataset.id;
    $('[data-act="open"]', row).onclick = async () => {
      const n = await api('GET', '/api/notes/' + id);
      openNoteModal(n);
    };
    $('[data-act="move"]', row).onclick = async () => {
      const nn = await api('GET', '/api/notes/' + id);
      openModal(`<h3>移动到分类</h3>
        <div class="frow"><select id="nCatMv">${noteState.cats.map(c => `<option value="${c.id}"${c.id === nn.categoryId ? ' selected' : ''}>${esc(c.name)}</option>`).join('')}</select></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfMv">移动</button></div>`);
      $('#cfMv').onclick = async () => {
        await api('PUT', '/api/notes/' + id, { categoryId: +$('#nCatMv').value });
        closeModal(); toast('已移动'); loadNotes();
      };
    };
    $('[data-act="del"]', row).onclick = () => {
      openModal(`<h3>删除这篇笔记？</h3><p class="muted">删除后无法恢复。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/notes/' + id);
        closeModal(); toast('已删除'); renderNotes();
      };
    };
  });
}
// marked.js 动态加载
let _markedLoaded = false;
function loadMarked() {
  return new Promise((resolve) => {
    if (window.marked) { _markedLoaded = true; return resolve(); }
    if (_markedLoaded) return resolve();
    const sc = document.createElement('script');
    sc.src = 'https://cdn.jsdelivr.net/npm/marked@12.0.0/marked.min.js';
    sc.onload = () => { _markedLoaded = true; resolve(); };
    sc.onerror = () => resolve();
    document.head.appendChild(sc);
  });
}
function openNoteModal(n) {
  const isNew = !n;
  n = n || { title: '', content: '', categoryId: noteState.cur || undefined };
  const catOpts = noteState.cats.map(c => `<option value="${c.id}"${c.id === (n.categoryId || noteState.cur) ? ' selected' : ''}>${esc(c.name)}</option>`).join('');
  openModal(`<h3>${isNew ? '写笔记' : '编辑笔记'}</h3>
    <div class="frow"><label>标题</label><input id="nTitle" value="${esc(n.title || '')}" maxlength="80"></div>
    <div class="frow"><label>分类</label><select id="nCat">${catOpts}</select></div>
    <div style="display:flex;gap:6px;margin-bottom:8px;align-items:center;flex-wrap:wrap">
      <button class="btn ghost sm" id="nEditTab">编辑</button>
      <button class="btn ghost sm" id="nPrevTab">预览</button>
      <span id="mdTools" style="display:flex;gap:6px;align-items:center;flex-wrap:wrap">
      <span style="width:1px;height:20px;background:var(--border);margin:0 4px"></span>
      <button class="btn ghost sm" data-md="**" title="粗体">B</button>
      <button class="btn ghost sm" data-md="*" title="斜体"><i>I</i></button>
      <button class="btn ghost sm" data-md="\`" title="行内代码">&lt;&gt;</button>
      <button class="btn ghost sm" data-md="codeblock" title="代码块">{ }</button>
      <button class="btn ghost sm" data-md="#" title="标题">#</button>
      <button class="btn ghost sm" data-md="- " title="列表">•</button>
      <button class="btn ghost sm" data-md="> " title="引用">”</button>
      <button class="btn ghost sm" data-md="u" title="下划线"><u>U</u></button>
      <button class="btn ghost sm" data-md="link" title="超链接">🔗</button>
      <button class="btn ghost sm" data-md="left" title="靠左">⇤</button>
      <button class="btn ghost sm" data-md="center" title="居中">⇔</button>
      <button class="btn ghost sm" data-md="right" title="靠右">⇥</button>
      <select id="mdFontSize" class="btn ghost sm" title="字号" style="padding:4px 6px">
        <option value="">字号</option>
        <option value="12">12</option>
        <option value="14">14</option>
        <option value="16">16</option>
        <option value="18">18</option>
        <option value="20">20</option>
        <option value="24">24</option>
        <option value="28">28</option>
      </select>
    </span></div>
    <textarea id="nContent" class="notearea" placeholder="内容…" style="display:none">${esc(n.content || '')}</textarea>
    <div id="nPreview" class="md-preview" style="min-height:200px;max-height:420px;overflow-y:auto;border:1px solid var(--border);border-radius:8px;padding:12px"></div>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="cfSave">保存</button></div>`, true, true);
  const showEdit = () => {
    $('#nContent').style.display = '';
    $('#nPreview').style.display = 'none';
    $('#mdTools').style.display = '';
    $('#nEditTab').classList.remove('ghost');
    $('#nPrevTab').classList.add('ghost');
  };
  const showPrev = async () => {
    await loadMarked();
    let html = window.marked ? marked.parse($('#nContent').value) : esc($('#nContent').value);
    // 代码块加复制按钮
    html = html.replace(/<pre><code(.*?)>/g, '<div style="position:relative"><button class="btn ghost sm md-copy" style="position:absolute;right:8px;top:8px">复制</button><pre><code$1>');
    html = html.replace(/<\/pre>/g, '</pre></div>');
    $('#nPreview').innerHTML = html;
    $$('.md-copy', $('#nPreview')).forEach(b => b.onclick = () => {
      const code = b.parentElement.querySelector('code').innerText;
      navigator.clipboard.writeText(code).then(() => toast('已复制'));
    });
    $('#nContent').style.display = 'none';
    $('#nPreview').style.display = '';
    $('#mdTools').style.display = 'none';
    $('#nPrevTab').classList.remove('ghost');
    $('#nEditTab').classList.add('ghost');
  };
  $('#nEditTab').onclick = showEdit;
  $('#nPrevTab').onclick = showPrev;
  // Markdown 工具栏
  $$('[data-md]').forEach(b => b.onclick = () => {
    const ta = $('#nContent');
    const md = b.dataset.md;
    const s = ta.selectionStart, e = ta.selectionEnd, v = ta.value;
    const sel = v.slice(s, e);
    let ins = '';
    if (md === 'codeblock') ins = '\n```\n' + sel + '\n```\n';
    else if (md === '**' || md === '*') ins = md + sel + md;
    else if (md === '`') ins = '`' + sel + '`';
    else if (md === 'u') ins = '<u>' + sel + '</u>';
    else if (md === 'link') ins = '[' + (sel || '文本') + '](https://)';
    else if (md === 'center') ins = '\n<div style="text-align:center">\n' + sel + '\n</div>\n';
    else if (md === 'left') ins = '\n<div style="text-align:left">\n' + sel + '\n</div>\n';
    else if (md === 'right') ins = '\n<div style="text-align:right">\n' + sel + '\n</div>\n';
    else ins = md + sel;
    ta.value = v.slice(0, s) + ins + v.slice(e);
    ta.focus();
  });
  $('#mdFontSize').onchange = (e) => {
    const sz = e.target.value;
    if (!sz) return;
    const ta = $('#nContent');
    const s = ta.selectionStart, en = ta.selectionEnd, v = ta.value;
    const sel = v.slice(s, en) || '文字';
    ta.value = v.slice(0, s) + `<span style="font-size:${sz}px">` + sel + '</span>' + v.slice(en);
    ta.focus();
    e.target.value = '';
  };
  // 新建默认编辑，已有默认预览
  if (isNew) showEdit(); else showPrev();
  $('#cfSave').onclick = async () => {
    const title = $('#nTitle').value.trim(), content = $('#nContent').value;
    const categoryId = +$('#nCat').value;
    if (isNew) await api('POST', '/api/notes', { title, content, categoryId });
    else await api('PUT', '/api/notes/' + n.id, { title, content, categoryId });
    closeModal(); toast('已保存'); loadNotes();
  };
}

// 网址预览：弹窗内嵌网页
function normUrl(u) {
  u = (u || '').trim();
  return /^https?:\/\//i.test(u) ? u : 'https://' + u;
}
function openLinkPreview(l) {
  if (!l) return;
  const url = normUrl(l.url);
  openModal(`<h3>${esc(l.name || l.url)}</h3>
    <div class="preview-wrap"><iframe src="${esc(url)}" title="预览"></iframe></div>
    <div class="foot"><button class="btn ghost" data-close>关闭</button>
    <a class="btn" href="${esc(url)}" target="_blank" rel="noopener">新标签页打开</a></div>`, 'wide');
}

// 置顶切换
async function togglePin(type, id, pinned, refresh) {
  await api('POST', '/api/pin', { type, id, pinned: !pinned });
  toast(pinned ? '已取消置顶' : '已置顶');
  refresh();
}
// 排序：置顶始终在前，其余按模式排
function sortItems(list, mode) {
  const getT = x => x.updatedAt || x.createdAt || 0;
  const pinned = list.filter(x => x.isPinned);
  const rest = list.filter(x => !x.isPinned);
  if (mode === 'old') rest.sort((a, b) => getT(a) - getT(b));
  else if (mode === 'name') rest.sort((a, b) => (a.name || a.title || a.label || '').localeCompare(b.name || b.title || b.label || '', 'zh'));
  return [...pinned, ...rest];
}
function sortSelHTML(cur) {
  return `<select class="sel sortsel">
    <option value="new"${cur === 'new' ? ' selected' : ''}>从新到旧</option>
    <option value="old"${cur === 'old' ? ' selected' : ''}>从旧到新</option>
    <option value="name"${cur === 'name' ? ' selected' : ''}>名称排序</option>
  </select>`;
}
function pinBtnHTML(type, item) {
  return `<button class="icon-btn${item.isPinned ? ' pinned' : ''}" data-pin="${type}:${item.id}:${item.isPinned ? 1 : 0}" title="${item.isPinned ? '取消置顶' : '置顶'}">${ic('pin')}</button>`;
}
function bindPinBtns(box, type, refresh) {
  $$('[data-pin]', box).forEach(b => b.onclick = e => {
    e.stopPropagation();
    const [t, id, p] = b.dataset.pin.split(':');
    togglePin(t, +id, p === '1', refresh);
  });
}
function fmtSize(b) {
  if (!b) return '';
  if (b < 1024) return b + 'B';
  if (b < 1048576) return (b / 1024).toFixed(1) + 'KB';
  return (b / 1048576).toFixed(1) + 'MB';
}
/* ---------- images view ---------- */
let imgState = { cats: [], cur: 0, mode: 'm', sort: 'new' };
async function renderImages() {
  const v = $('#view');
  v.innerHTML = `
    <div class="topbar">
      <h2>图片</h2>
      <span style="flex:1"></span>
      <span class="muted" style="font-size:12px">TG 直接发图片入库（配文=分类）</span>
      <div class="seg" id="imgMode">
        <button data-m="list" title="列表">${ic('list')}</button>
        <button data-m="s" title="小缩略图">S</button>
        <button data-m="m" title="中缩略图" class="on">M</button>
        <button data-m="l" title="大缩略图">L</button>
      </div>
      <select id="imgSort" class="sel">
        <option value="new">从新到旧</option>
        <option value="old">从旧到新</option>
      </select>
      <button class="btn sm" id="upImg">${ic('upload')} 上传图片</button>
      <input type="file" id="imgFile" accept="image/*" multiple style="display:none">
    </div>
    <div class="catbar" id="imgCatBar"></div>
    <div id="imgGrid"><div class="empty"><div>加载中…</div></div></div>`;
  $('#upImg').onclick = () => $('#imgFile').click();
  $$('#imgMode button').forEach(b => b.onclick = () => {
    imgState.mode = b.dataset.m;
    $$('#imgMode button').forEach(x => x.classList.toggle('on', x === b));
    drawImages();
  });
  $('#imgSort').onchange = e => { imgState.sort = e.target.value; drawImages(); };
  $('#imgFile').onchange = async e => {
    const files = [...e.target.files];
    for (const f of files) {
      const fd = new FormData();
      fd.append('file', f);
      let catId = imgState.cur;
      if (catId === 0) {
        const unc = (imgState.cats || []).find(c => c.name === '未分类');
        catId = unc ? unc.id : 0;
      }
      fd.append('categoryId', catId);
      await fetch('/api/images', { method: 'POST', body: fd, credentials: 'same-origin' });
    }
    e.target.value = '';
    toast(`已上传 ${files.length} 张`);
    drawImages();
  };
  try { imgState.cats = await api('GET', '/api/image-categories'); } catch (e) { imgState.cats = []; }
  if (!imgState.cats.length) imgState.cats = [{ id: 0, name: '未分类' }];
  if (imgState.cur !== 0 && !imgState.cats.find(c => c.id === imgState.cur)) imgState.cur = imgState.cats[0].id;
  drawImgCatBar();
  await drawImages();
}
function drawImgCatBar() {
  const bar = $('#imgCatBar');
  const allCats = [{ id: 0, name: '全部' }, ...imgState.cats];
  bar.innerHTML = allCats.map(c =>
    `<button class="pill ${c.id === imgState.cur ? 'active' : ''}" data-id="${c.id}">${esc(c.name)}</button>`
  ).join('') + `<button class="pill ghost" id="addImgCat">${ic('plus')} 分类</button>`;
  $$('.pill[data-id]', bar).forEach(b => b.onclick = () => { imgState.cur = +b.dataset.id; drawImgCatBar(); drawImages(); });
  $('#addImgCat').onclick = () => {
    openModal(`<h3>新建分类</h3>
      <div class="frow"><input id="catName" placeholder="分类名称" maxlength="20"></div>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn" id="cfAdd">创建</button></div>`);
    $('#cfAdd').onclick = async () => {
      const name = $('#catName').value.trim();
      if (!name) return;
      const r = await api('POST', '/api/image-categories', { name });
      closeModal(); toast('已创建');
      imgState.cats = await api('GET', '/api/image-categories');
      imgState.cur = r.id;
      drawImgCatBar(); drawImages();
    };
  };
  const cur = imgState.cats.find(c => c.id === imgState.cur);
  if (cur && cur.name !== '未分类') {
    const mg = document.createElement('span');
    mg.style.marginLeft = '8px';
    mg.innerHTML = `<button class="icon-btn" id="renImgCat" title="重命名分类">${ic('edit')}</button>
      <button class="icon-btn" id="delImgCat" title="删除分类（图片移到未分类）">${ic('trash')}</button>`;
    bar.appendChild(mg);
    $('#renImgCat').onclick = () => {
      openModal(`<h3>重命名分类</h3>
        <div class="frow"><input id="catName" value="${esc(cur.name)}" maxlength="20"></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfRen">保存</button></div>`);
      $('#cfRen').onclick = async () => {
        const name = $('#catName').value.trim();
        if (!name) return;
        await api('PUT', '/api/image-categories/' + cur.id, { name });
        closeModal(); toast('已重命名');
        imgState.cats = await api('GET', '/api/image-categories');
        drawImgCatBar(); drawImages();
      };
    };
    $('#delImgCat').onclick = () => {
      openModal(`<h3>删除分类「${esc(cur.name)}」？</h3><p class="muted">里面的图片会移到"未分类"。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/image-categories/' + cur.id);
        closeModal(); toast('已删除');
        imgState.cats = await api('GET', '/api/image-categories');
        imgState.cur = imgState.cats[0].id;
        drawImgCatBar(); drawImages();
      };
    };
  }
}
async function drawImages() {
  const box = $('#imgGrid');
  let list = [];
  try { list = await api('GET', '/api/images?categoryId=' + imgState.cur); } catch (e) {}
  if (imgState.sort === 'old') list = [...list].reverse();
  if (!list.length) {
    box.innerHTML = `<div class="empty">${ic('image')}<div>这个分类还没有图片</div></div>`;
    return;
  }
  if (imgState.mode === 'list') {
    box.innerHTML = `<div class="card" style="padding:4px 14px">` + list.map(im => `
      <div class="lrow" data-id="${im.id}">
        <img class="lthumb" loading="lazy" src="/api/images/${im.id}?size=s">
        <div class="grow">
          <div class="lab">${esc(im.origName || im.caption || '（无名称）')}</div>
          <div class="smeta">${fmtDate(im.createdAt)}${im.fileSize ? ' · ' + fmtSize(im.fileSize) : ''}</div>
        </div>
        <div class="acts">
          <button class="btn ghost sm" data-act="view">${ic('eye')} 查看</button>
          <button class="btn ghost sm" data-act="share">${ic('share')} 分享</button>
          <button class="icon-btn" data-act="del" title="删除">${ic('trash')}</button>
        </div>
      </div>`).join('') + `</div>`;
  } else {
    const sz = imgState.mode;
    box.innerHTML = `<div class="imggrid ${sz}">` + list.map(im => `
      <div class="imgcell" data-id="${im.id}" style="position:relative">
        <img loading="lazy" src="/api/images/${im.id}?size=${sz}">
        <button class="icon-btn img-share" data-share="${im.id}" title="分享" style="position:absolute;top:6px;right:6px;background:rgba(0,0,0,.5);color:#fff">${ic('share')}</button>
        <div class="imgmeta"><div class="t">${esc(im.origName || im.caption || '')}</div><div class="d">${fmtDate(im.createdAt)}</div></div>
      </div>`).join('') + `</div>`;
  }
  $$('[data-id]', box).forEach(el => {
    const id = +el.dataset.id;
    const doShare = async (e) => {
      e.stopPropagation();
      const r = await api('POST', '/api/share', { kind: 'image', id });
      const url = location.origin + r.url;
      await navigator.clipboard.writeText(url);
      toast('分享链接已复制');
    };
    const shBtn = $('[data-act="share"]', el);
    if (shBtn) shBtn.onclick = doShare;
    const gridSh = $('[data-share]', el);
    if (gridSh) gridSh.onclick = doShare;
    el.onclick = e => {
      if (e.target.closest('[data-act="del"]')) return;
      if (e.target.closest('[data-act="share"]')) return;
      if (e.target.closest('[data-share]')) return;
      openImageViewer(id, list);
    };
  });
  $$('[data-act="del"]', box).forEach(b => b.onclick = e => {
    e.stopPropagation();
    const id = +b.closest('[data-id]').dataset.id;
    openModal(`<h3>删除这张图片？</h3><p class="muted">删除后无法恢复。</p>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn danger" id="cfDel">删除</button></div>`);
    $('#cfDel').onclick = async () => {
      await api('DELETE', '/api/images/' + id);
      closeModal(); toast('已删除'); drawImages();
    };
  });
}
function openImageViewer(id, list) {
  const im = list.find(x => x.id === id);
  if (!im) return;
  const opts = imgState.cats.map(c =>
    `<option value="${c.id}" ${c.id === im.categoryId ? 'selected' : ''}>${esc(c.name)}</option>`).join('');
  openModal(`
    <div class="imgview"><img src="/api/images/${im.id}?size=orig"></div>
    <div class="frow" style="margin-top:12px"><label>文件名</label><input value="${esc(im.origName || '')}" readonly style="background:var(--bg)"></div>
    <div class="frow"><label>分类</label><select id="ivCat">${opts}</select></div>
    <div class="foot"><button class="btn ghost" data-close>关闭</button>
    <a class="btn ghost" href="/api/images/${im.id}" download>下载</a>
    <button class="btn ghost" id="ivShare">${ic('share')} 分享</button>
    <button class="btn danger" id="ivDel">删除</button>
    <button class="btn" id="ivSave">保存</button></div>`, 'wide');
  $('#ivShare').onclick = async () => {
    const r = await api('POST', '/api/share', { kind: 'image', id: im.id });
    await navigator.clipboard.writeText(location.origin + r.url);
    toast('分享链接已复制');
  };
  $('#ivSave').onclick = async () => {
    await api('PUT', '/api/images/' + im.id, {
      categoryId: +$('#ivCat').value });
    closeModal(); toast('已保存');
    if (+$('#ivCat').value !== imgState.cur) { /* 移动后刷新 */ }
    drawImages();
  };
  $('#ivDel').onclick = () => {
    openModal(`<h3>删除这张图片？</h3><p class="muted">删除后无法恢复。</p>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn danger" id="cfDel">删除</button></div>`);
    $('#cfDel').onclick = async () => {
      await api('DELETE', '/api/images/' + im.id);
      closeModal(); closeModal(); toast('已删除'); drawImages();
    };
  };
}

/* ---------- view router ---------- */
async function refreshMerchants() {
  try { state.merchants = await api('GET', '/api/merchants'); }
  catch (e) { state.merchants = []; }
}
/* ---------- remote browser view ---------- */
function renderBrowser() {
  const v = $('#view');
  if (!browserCfg.enabled || !browserCfg.url) {
    v.innerHTML = `<div class="empty"><div>远程浏览器未启用</div><div class="muted" style="font-size:12px;margin-top:6px">请在设置 → 远程浏览器中配置并启用</div></div>`;
    return;
  }
  const sep = browserCfg.url.includes('?') ? '&' : '?';
  let src = browserCfg.url;
  if (browserCfg.pwd) src += `${sep}pwd=${encodeURIComponent(browserCfg.pwd)}&usr=${encodeURIComponent('SecretDock')}`;
  v.innerHTML = `<iframe src="${esc(src)}" style="width:100%;height:100vh;border:0;display:block;" allow="clipboard-read; clipboard-write" title="远程浏览器"></iframe>`;
  v.style.cssText = 'position:absolute;inset:0;padding:0;margin:0;max-width:none;';
  const main = v.closest('.main');
  if (main) main.style.cssText = 'padding:0;max-width:none;position:relative;';
}
function renderLinkView() {
  const v = $('#view');
  const li = findLinkItem(state.view);
  if (!li) { state.view = 'merchants'; renderView(); return; }
  v.innerHTML = `<iframe src="${esc(li.url)}" style="width:100%;height:100vh;border:0;display:block;" title="${esc(li.label)}"></iframe>`;
  v.style.cssText = 'position:absolute;inset:0;padding:0;margin:0;max-width:none;';
  const main = v.closest('.main');
  if (main) main.style.cssText = 'padding:0;max-width:none;position:relative;';
}
async function renderView() {
  const v = $('#view');
  if (v) v.style.cssText = '';
  const main = v ? v.closest('.main') : null;
  if (main) main.style.cssText = '';
  if (state.view === 'settings') await renderSettings();
  else if (state.view === 'browser') renderBrowser();
  else if (typeof state.view === 'string' && state.view.startsWith('lnk-')) renderLinkView();
  else if (state.view === 'scripts') await renderScripts();
  else if (state.view === 'links') await renderLinks();
  else if (state.view === 'notes') await renderNotes();
  else if (state.view === 'images') await renderImages();
  else if (state.view === 'files') await renderFiles();
  else if (state.view === 'nodes') await renderNodes();
  else if (state.view === 'ssh') await renderSSH();
  else if (state.view === 'reminders') await renderReminders();
  else if (state.view === 'tghelp') renderTgHelp();
  else if (state.view === 'detail' && state.cur) await openMerchant(state.cur.id);
  else { state.view = 'merchants'; await renderMerchants(); }
}

/* ---------- scripts view (top-level) ---------- */
let scriptState = { cats: [], cur: 0 };
async function renderScripts() {
  const v = $('#view');
  v.innerHTML = `
    <div class="topbar">
      <h2>脚本</h2>
      <span style="flex:1"></span>
      ${sortSelHTML('new')}
      <span class="muted" style="font-size:12px">TG 里第一行发"脚本"粘贴入库</span>
    </div>
    <div class="catbar" id="scriptCatBar"></div>
    <div id="scriptList"><div class="empty"><div>加载中…</div></div></div>`;
  try { scriptState.cats = await api('GET', '/api/script-categories'); } catch (e) { scriptState.cats = []; }
  drawScriptCatBar();
  await loadScripts();
}
function drawScriptCatBar() {
  const bar = $('#scriptCatBar');
  const cats = [{ id: 0, name: '全部' }, ...scriptState.cats];
  bar.innerHTML = cats.map(c =>
    `<button class="pill ${c.id === scriptState.cur ? 'active' : ''}" data-id="${c.id}">${esc(c.name)}</button>`
  ).join('') + `<button class="pill ghost" id="addScriptCat">${ic('plus')} 分类</button>`;
  $$('.pill[data-id]', bar).forEach(b => b.onclick = () => { scriptState.cur = +b.dataset.id; drawScriptCatBar(); loadScripts(); });
  $('#addScriptCat').onclick = () => {
    openModal(`<h3>新建分类</h3>
      <div class="frow"><input id="catName" placeholder="分类名称" maxlength="20"></div>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn" id="cfAdd">创建</button></div>`);
    $('#cfAdd').onclick = async () => {
      const name = $('#catName').value.trim();
      if (!name) return;
      const r = await api('POST', '/api/script-categories', { name });
      closeModal(); toast('已创建');
      scriptState.cats = await api('GET', '/api/script-categories');
      scriptState.cur = r.id;
      drawScriptCatBar(); loadScripts();
    };
  };
  const cur = scriptState.cats.find(c => c.id === scriptState.cur);
  if (cur && cur.name !== '未分类') {
    const mg = document.createElement('span');
    mg.style.marginLeft = '8px';
    mg.innerHTML = `<button class="icon-btn" id="renScriptCat" title="重命名分类">${ic('edit')}</button>
      <button class="icon-btn" id="delScriptCat" title="删除分类（脚本移到未分类）">${ic('trash')}</button>`;
    bar.appendChild(mg);
    $('#renScriptCat').onclick = () => {
      openModal(`<h3>重命名分类</h3>
        <div class="frow"><input id="catName" value="${esc(cur.name)}" maxlength="20"></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfRen">保存</button></div>`);
      $('#cfRen').onclick = async () => {
        const name = $('#catName').value.trim();
        if (!name) return;
        await api('PUT', '/api/script-categories/' + cur.id, { name });
        closeModal(); toast('已重命名');
        scriptState.cats = await api('GET', '/api/script-categories');
        drawScriptCatBar(); loadScripts();
      };
    };
    $('#delScriptCat').onclick = () => {
      openModal(`<h3>删除分类「${esc(cur.name)}」？</h3><p class="muted">里面的脚本会移到"未分类"。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/script-categories/' + cur.id);
        closeModal(); toast('已删除');
        scriptState.cats = await api('GET', '/api/script-categories');
        scriptState.cur = 0;
        drawScriptCatBar(); loadScripts();
      };
    };
  }
}
async function loadScripts() {
  let list = [];
  try { list = await api('GET', '/api/scripts?categoryId=' + scriptState.cur); } catch (e) {}
  const sortSel = $('.sortsel');
  const mode = sortSel ? sortSel.value : 'new';
  paintScripts(sortItems(list, mode));
  if (sortSel) sortSel.onchange = e => paintScripts(sortItems(list, e.target.value));
}
function paintScripts(list) {
  const box = $('#scriptList');
  if (!list.length) {
    box.innerHTML = `<div class="empty">${ic('code')}<div>还没有脚本，去 TG 粘贴一个吧</div></div>`;
    return;
  }
  box.innerHTML = `<div class="card" style="padding:4px 14px">` + list.map(c => `
    <div class="lrow" data-id="${c.id}">
      <span class="licon">${ic('code')}</span>
      <div class="grow">
        <div class="lab">${c.isPinned ? '<span class="pinmark">📌 </span>' : ''}${esc(c.name || '（未命名）')}${c.isUserscript ? ' <span class="tag accent">油猴脚本</span>' : ''}</div>
        <div class="smeta">${fmtDate(c.createdAt)}</div>
      </div>
      <div class="acts">
        <button class="btn ghost sm" data-act="view">${ic('eye')} 查看</button>
        <button class="btn ghost sm" data-act="copy">${ic('copy')} 复制</button>
        <button class="btn ghost sm" data-act="dl">${ic('download')} 下载</button>
        ${pinBtnHTML('scripts', c)}
        <button class="icon-btn" data-act="move" title="移动分类">${ic('folder')}</button>
        <button class="icon-btn" data-act="del" title="删除">${ic('trash')}</button>
      </div>
    </div>`).join('') + `</div>`;
  bindPinBtns(box, 'scripts', renderScripts);
  $$('.lrow', box).forEach(row => {
    const id = +row.dataset.id;
    const getSrc = () => api('GET', '/api/scripts/' + id);
    $('[data-act="view"]', row).onclick = async () => {
      const sc = await getSrc();
      openScriptModal(sc);
    };
    $('[data-act="copy"]', row).onclick = async () => copyText((await getSrc()).content || '');
    $('[data-act="dl"]', row).onclick = async () => {
      const sc = await getSrc();
      const a = document.createElement('a');
      a.href = URL.createObjectURL(new Blob([sc.content || ''], { type: 'text/javascript' }));
      a.download = (sc.name || 'script').replace(/[^\w\-.]+/g, '_') + '.user.js';
      a.click();
      setTimeout(() => URL.revokeObjectURL(a.href), 5000);
    };
    $('[data-act="move"]', row).onclick = async () => {
      const sc = await getSrc();
      openModal(`<h3>移动到分类</h3>
        <div class="frow"><select id="sCat">${scriptState.cats.map(c => `<option value="${c.id}"${c.id === sc.categoryId ? ' selected' : ''}>${esc(c.name)}</option>`).join('')}</select></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfMove">移动</button></div>`);
      $('#cfMove').onclick = async () => {
        await api('PUT', '/api/scripts/' + id, { categoryId: +$('#sCat').value });
        closeModal(); toast('已移动'); loadScripts();
      };
    };
    $('[data-act="move"]', row).onclick = async () => {
      const nn = await api('GET', '/api/notes/' + id);
      openModal(`<h3>移动到分类</h3>
        <div class="frow"><select id="nCatMv">${noteState.cats.map(c => `<option value="${c.id}"${c.id === nn.categoryId ? ' selected' : ''}>${esc(c.name)}</option>`).join('')}</select></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfMv">移动</button></div>`);
      $('#cfMv').onclick = async () => {
        await api('PUT', '/api/notes/' + id, { categoryId: +$('#nCatMv').value });
        closeModal(); toast('已移动'); loadNotes();
      };
    };
    $('[data-act="del"]', row).onclick = () => {
      openModal(`<h3>删除这个脚本？</h3><p class="muted">删除后无法恢复。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/scripts/' + id);
        closeModal(); toast('已删除'); loadScripts();
      };
    };
  });
}

/* ---------- boot ---------- */
async function boot() {
  try { localStorage.getItem('x'); } catch (e) {}
  const theme = (() => { try { return localStorage.getItem('sd-theme'); } catch (e) { return null; } })();
  const savedView = (() => { try { return localStorage.getItem('sd-view'); } catch (e) { return null; } })();
  if (savedView && ['merchants', 'scripts', 'links', 'notes', 'images', 'files', 'nodes', 'reminders', 'ssh', 'tghelp', 'settings', 'browser'].includes(savedView)) {
    state.view = savedView;
  }
  document.documentElement.dataset.theme = theme === 'dark' ? 'dark' : 'light';
  state.sbCollapsed = (() => { try { return localStorage.getItem('sd-sb-collapsed') === '1'; } catch (e) { return false; } })();
  // 检查是否需要初始设置
  try {
    const st = await api('GET', '/api/setup/status', undefined, { noReload: true });
    if (st.needSetup) { renderSetup(); return; }
  } catch (e) {}
  try {
    await api('GET', '/api/me', undefined, { noReload: true });
    state.authed = true;
  } catch (e) { state.authed = false; renderLogin(); return; }
  await loadBrowserCfg();
  // 如果浏览器未启用但记住了 browser 视图，回退到机密库
  if (state.view === 'browser' && !browserCfg.enabled) state.view = 'merchants';
  $('#app').innerHTML = shell('');
  bindShell();
  await refreshMerchants();
  await renderView();
  backfillMenuIcons();
}
document.addEventListener('DOMContentLoaded', boot);

/* ---------- files view ---------- */
let fileState = { cats: [], cur: 0, sort: 'new' };

function fileIcon(name) {
  const ext = (name.split('.').pop() || '').toLowerCase();
  const code = ['js', 'ts', 'py', 'go', 'sh', 'json', 'md', 'txt', 'html', 'css', 'xml', 'yaml', 'yml', 'ini', 'log'];
  if (code.includes(ext)) return 'code';
  if (['zip', 'rar', '7z', 'tar', 'gz'].includes(ext)) return 'archive';
  if (['pdf'].includes(ext)) return 'pdf';
  return 'file';
}

async function renderFiles() {
  const v = $('#view');
  v.innerHTML = `
    <div class="topbar">
      <h2>文件</h2>
      <span style="flex:1"></span>
      <span class="muted" style="font-size:12px">TG 直接发文件入库（配文=分类）</span>
      ${sortSelHTML('new')}
      <button class="btn sm" id="upFile">${ic('upload')} 上传文件</button>
      <input type="file" id="fileInput" multiple style="display:none">
    </div>
    <div class="catbar" id="fileCatBar"></div>
    <div id="fileList"><div class="empty"><div>加载中…</div></div></div>`;
  $('.sortsel').onchange = e => { fileState.sort = e.target.value; drawFiles(); };
  $('#upFile').onclick = () => $('#fileInput').click();
  $('#fileInput').onchange = async e => {
    const files = [...e.target.files];
    for (const f of files) {
      const fd = new FormData();
      fd.append('file', f);
      fd.append('categoryId', fileState.cur);
      await fetch('/api/files', { method: 'POST', body: fd, credentials: 'same-origin' });
    }
    e.target.value = '';
    toast(`已上传 ${files.length} 个文件`);
    drawFiles();
  };
  try { fileState.cats = await api('GET', '/api/file-categories'); } catch (e) { fileState.cats = []; }
  if (!fileState.cats.length) fileState.cats = [{ id: 0, name: '未分类' }];
  if (fileState.cur !== 0 && !fileState.cats.find(c => c.id === fileState.cur)) fileState.cur = fileState.cats[0].id;
  drawFileCatBar();
  await drawFiles();
}

function drawFileCatBar() {
  const bar = $('#fileCatBar');
  const allCats = [{ id: 0, name: '全部' }, ...fileState.cats];
  bar.innerHTML = allCats.map(c =>
    `<button class="pill ${c.id === fileState.cur ? 'active' : ''}" data-id="${c.id}">${esc(c.name)}</button>`
  ).join('') + `<button class="pill ghost" id="addFileCat">${ic('plus')} 分类</button>`;
  $$('.pill[data-id]', bar).forEach(b => b.onclick = () => { fileState.cur = +b.dataset.id; drawFileCatBar(); drawFiles(); });
  $('#addFileCat').onclick = () => {
    openModal(`<h3>新建分类</h3>
      <div class="frow"><input id="catName" placeholder="分类名称" maxlength="20"></div>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn" id="cfAdd">创建</button></div>`);
    $('#cfAdd').onclick = async () => {
      const name = $('#catName').value.trim();
      if (!name) return;
      const r = await api('POST', '/api/file-categories', { name });
      closeModal(); toast('已创建');
      fileState.cats = await api('GET', '/api/file-categories');
      fileState.cur = r.id;
      drawFileCatBar(); drawFiles();
    };
  };
  const cur = fileState.cats.find(c => c.id === fileState.cur);
  if (cur && cur.name !== '未分类') {
    const mg = document.createElement('span');
    mg.style.marginLeft = '8px';
    mg.innerHTML = `<button class="icon-btn" id="renFileCat" title="重命名分类">${ic('edit')}</button>
      <button class="icon-btn" id="delFileCat" title="删除分类（文件移到未分类）">${ic('trash')}</button>`;
    bar.appendChild(mg);
    $('#renFileCat').onclick = () => {
      openModal(`<h3>重命名分类</h3>
        <div class="frow"><input id="catName" value="${esc(cur.name)}" maxlength="20"></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfRen">保存</button></div>`);
      $('#cfRen').onclick = async () => {
        const name = $('#catName').value.trim();
        if (!name) return;
        await api('PUT', '/api/file-categories/' + cur.id, { name });
        closeModal(); toast('已重命名');
        fileState.cats = await api('GET', '/api/file-categories');
        drawFileCatBar(); drawFiles();
      };
    };
    $('#delFileCat').onclick = () => {
      openModal(`<h3>删除分类「${esc(cur.name)}」？</h3><p class="muted">里面的文件会移到"未分类"。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/file-categories/' + cur.id);
        closeModal(); toast('已删除');
        fileState.cats = await api('GET', '/api/file-categories');
        fileState.cur = fileState.cats[0].id;
        drawFileCatBar(); drawFiles();
      };
    };
  }
}

async function drawFiles() {
  const box = $('#fileList');
  let list = [];
  try { list = await api('GET', '/api/files?categoryId=' + fileState.cur); } catch (e) {}
  list = sortItems(list, fileState.sort);
  if (!list.length) {
    box.innerHTML = `<div class="empty">${ic('file')}<div>这个分类还没有文件</div></div>`;
    return;
  }
  box.innerHTML = `<div class="card" style="padding:4px 14px">` + list.map(f => `
    <div class="lrow" data-id="${f.id}">
      <span class="licon">${ic(fileIcon(f.name))}</span>
      <div class="grow" style="min-width:0">
        <div class="lab" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${esc(f.name || '（未命名）')}${f.isText ? ' <span class="tag">文本</span>' : ''}</div>
        <div class="smeta">${fmtDate(f.createdAt)}${f.fileSize ? ' · ' + fmtSize(f.fileSize) : ''}</div>
      </div>
      <div class="acts">
        ${f.isText ? `<button class="btn ghost sm" data-act="edit">${ic('edit')} 预览/编辑</button>` : ''}
        <button class="icon-btn" data-act="dl" title="下载">${ic('download')}</button>
        <button class="icon-btn" data-act="share" title="分享">${ic('share')}</button>
        <button class="icon-btn" data-act="rename" title="重命名">${ic('edit')}</button>
        <button class="icon-btn" data-act="move" title="移动分类">${ic('folder')}</button>
        <button class="icon-btn" data-act="del" title="删除">${ic('trash')}</button>
      </div>
    </div>`).join('') + `</div>`;
  $$('.lrow', box).forEach(row => {
    const id = +row.dataset.id;
    const f = list.find(x => x.id === id);
    const shBtn = $('[data-act="share"]', row);
    if (shBtn) shBtn.onclick = async (e) => {
      e.stopPropagation();
      const r = await api('POST', '/api/share', { kind: 'file', id });
      const url = location.origin + r.url;
      await navigator.clipboard.writeText(url);
      toast('分享链接已复制');
    };
    $('[data-act="dl"]', row).onclick = () => {
      const a = document.createElement('a');
      a.href = '/api/files/' + id;
      a.download = f.name;
      document.body.appendChild(a); a.click(); a.remove();
    };
    const edBtn = $('[data-act="edit"]', row);
    if (edBtn) edBtn.onclick = () => openFileEditor(id, f.name);
    $('[data-act="rename"]', row).onclick = () => {
      openModal(`<h3>重命名</h3>
        <div class="frow"><input id="fName" value="${esc(f.name)}" maxlength="120"></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfRen">保存</button></div>`);
      $('#cfRen').onclick = async () => {
        const name = $('#fName').value.trim();
        if (!name) return;
        await api('PUT', '/api/files/' + id, { name });
        closeModal(); toast('已重命名'); drawFiles();
      };
    };
    $('[data-act="move"]', row).onclick = () => {
      openModal(`<h3>移动到分类</h3>
        <div class="frow"><select id="fCat">${fileState.cats.map(c => `<option value="${c.id}"${c.id === f.categoryId ? ' selected' : ''}>${esc(c.name)}</option>`).join('')}</select></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfMove">移动</button></div>`);
      $('#cfMove').onclick = async () => {
        await api('PUT', '/api/files/' + id, { categoryId: +$('#fCat').value });
        closeModal(); toast('已移动'); drawFiles();
      };
    };
    $('[data-act="move"]', row).onclick = async () => {
      const nn = await api('GET', '/api/notes/' + id);
      openModal(`<h3>移动到分类</h3>
        <div class="frow"><select id="nCatMv">${noteState.cats.map(c => `<option value="${c.id}"${c.id === nn.categoryId ? ' selected' : ''}>${esc(c.name)}</option>`).join('')}</select></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfMv">移动</button></div>`);
      $('#cfMv').onclick = async () => {
        await api('PUT', '/api/notes/' + id, { categoryId: +$('#nCatMv').value });
        closeModal(); toast('已移动'); loadNotes();
      };
    };
    $('[data-act="del"]', row).onclick = () => {
      openModal(`<h3>删除这个文件？</h3><p class="muted">${esc(f.name)}</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/files/' + id);
        closeModal(); toast('已删除'); drawFiles();
      };
    };
  });
}

async function openFileEditor(id, name) {
  let content = '';
  try {
    const r = await api('GET', '/api/files/' + id + '/text');
    content = r.content;
  } catch (e) { toast('读取失败'); return; }
  openModal(`<h3>${esc(name)}</h3>
    <div class="frow"><textarea id="fText" style="min-height:50vh;font-family:monospace;font-size:13px" spellcheck="false">${esc(content)}</textarea></div>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="cfSave">保存</button></div>`, true);
  $('#cfSave').onclick = async () => {
    await api('PUT', '/api/files/' + id + '/text', { content: $('#fText').value });
    closeModal(); toast('已保存');
  };
}

/* ---------- TG 操作说明 ---------- */
function renderTgHelp() {
  const v = $('#view');
  const sec = (emoji, title, body) => `
    <div class="card" style="margin-bottom:14px;padding:18px 20px">
      <h3 style="margin:0 0 10px">${emoji} ${title}</h3>
      <div class="helpbody">${body}</div>
    </div>`;
  const code = t => `<code class="hl">${t}</code>`;
  v.innerHTML = `
    <div class="topbar"><h2>TG 操作说明</h2></div>
    <div style="max-width:860px">
    ${sec('🏦', '添加机密', `
      <p><strong>直接发服务商名</strong>（最方便），后面每行一条（标签+值）：</p>
      ${code('cloudflare<br>访问密钥：abc123<br>令牌值 xyz789')}
      <p>也可以第一行加 ${code('机密')} 或 ${code('机密库')} 关键字：</p>
      ${code('机密<br>cloudflare<br>访问密钥：abc123')}
      <ul>
        <li>标签支持中文，行尾可加 <code>30天</code> 设置过期</li>
        <li>服务商名 100% 匹配直接入库，否则列出相似候选</li>
        <li>入库后自动删除双方消息，只留一条回执</li>
      </ul>`)}
    ${sec('🌐', '订阅节点', `
      <p>第一行发 ${code('订阅')} / ${code('节点')} / ${code('订阅节点')} 三种都可以，第二行是名称（可选），后面是链接：</p>
      ${code('订阅<br>XX云<br>https://xxx.com/sub')}
      <ul>
        <li>http(s) 开头识别为<strong>订阅</strong>，vless://、vmess://、trojan:// 等识别为<strong>单节点</strong></li>
        <li>自动识别协议（VLESS/VMess/Trojan/SS 等）</li>
        <li>不给名称显示"未命名"；多个链接可一次粘贴多行</li>
        <li>发送后点选分类（可新建），1分钟不选自动进"未分类"</li>
        <li>直接发 vless:// 链接会先让你选存入订阅节点还是机密库</li>
        <li>网页打开订阅节点页会自动测试连通性</li>
      </ul>`)}
    ${sec('🔍', '查询机密', `
      <p>只发<strong>服务商名</strong>一行，机器人会索取查询密码。</p>
      <ul>
        <li>密码错误超过上限会被封禁（30分钟→3小时→24小时→3天→7天→永久），可在网页设置中解封</li>
        <li>验证通过后机密按条发送，N 分钟后自动删除（设置中可配）</li>
      </ul>`)}
    ${sec('📜', '添加脚本', `
      <p>第一行发 ${code('脚本')}，后面粘贴脚本内容：</p>
      ${code('脚本<br>// ==UserScript==<br>// @name 我的脚本<br>...')}
      <ul>
        <li>发送后点选分类（可新建），1分钟不选自动进"未分类"</li>
        <li><strong>分类不用写在内容里</strong>，全部点选</li>
        <li>自动识别油猴脚本的 @name、Surge/Loon 的 #!name=</li>
        <li>直接粘贴油猴脚本（不带"脚本"）也能自动识别</li>
      </ul>`)}
    ${sec('🔗', '添加网址', `
      <p>第一行发 ${code('网址')}，后面每行 <code>名字+网址</code>：</p>
      ${code('网址<br>GitHub https://github.com')}
      <ul>
        <li>发送后点选分类（可新建），1分钟不选自动进"未分类"</li>
        <li><strong>分类不用写在内容里</strong>，全部点选</li>
        <li><strong>单个网址直接发送</strong>，不用打"网址"，自动抓取网页标题做名称（带"自检名称"标记）</li>
        <li>裸域名自动补 https://</li>
      </ul>`)}
    ${sec('📝', '添加笔记', `
      <p>第一行发 ${code('笔记')}，第二行是标题，后面是正文：</p>
      ${code('笔记<br>购物清单<br>牛奶、鸡蛋、面包')}
      <ul>
        <li>发送后点选分类（可新建），1分钟不选自动进"未分类"</li>
        <li><strong>分类不用写在内容里</strong>，全部点选</li>
      </ul>`)}
    ${sec('🖼️', '添加图片', `
      <ul>
        <li><strong>直接发图片</strong>，不用任何前缀</li>
        <li>配文会作为图片备注（不再是分类）</li>
        <li>发送后点选分类（可新建），1分钟不选自动进"未分类"</li>
        <li>服务器自动生成大中小三档缩略图</li>
      </ul>`)}
    ${sec('📄', '添加文件', `
      <ul>
        <li><strong>直接发文件</strong>（图片以外的任何格式）</li>
        <li>发送后点选分类（可新建），1分钟不选自动进"未分类"</li>
        <li><strong>分类不用写在配文里</strong>，全部点选</li>
        <li>文本文件（JS/TXT/MD等）在网页端可预览编辑</li>
        <li>注意：发文件形式不会触发脚本识别，脚本内容请用"脚本"协议发送</li>
      </ul>`)}
    ${sec('🖥️', '添加SSH主机', `
      <p>第一行发 ${code('SSH')}，后面三行是地址、用户名、密码：</p>
      ${code('SSH<br>110.110.110.110:22<br>root<br>密码')}
      <ul>
        <li>IPv6 直接填，如 <code>2401:db00::1</code> 或 <code>[2401:db00::1]:22</code></li>
        <li>不写端口默认 22，不写用户名默认 root</li>
        <li>发送后点选分类（可新建），1分钟不选自动进"未分类"</li>
        <li>网页 SSH 菜单可连接终端、管理文件、查看系统状态</li>
      </ul>`)}
    ${sec('⏰', '添加提醒', `
      <p>第一行发 ${code('提醒')}，第二行是标题，后面可写备注：</p>
      ${code('提醒<br>CF续费<br>域名月底到期')}
      <ul>
        <li>发送后直接输入提醒时间，例如：<code>明天12点</code>、<code>每天8点</code>、<code>每周一9点</code>、<code>10-20 14:30</code></li>
        <li>支持：今天/明天/后天/大后天、M月D日、M-D、每天、每周X、每月X号、每年</li>
        <li>时间格式：12点、12:30、12点30分（不写时间默认9:00）</li>
        <li>到时间后按设置连发多条提醒（默认3条，设置中可配）</li>
      </ul>`)}
    ${sec('➕', '手动添加 /add', `
      <ul>
        <li>先选<strong>版块</strong>（机密库/脚本/网址/笔记/图片/文件）</li>
        <li>再选该版块下的<strong>分类</strong></li>
        <li>然后发送内容，自动进入所选分类</li>
        <li>每一步都有取消按钮</li>
      </ul>`)}
    ${sec('🔎', '分类查找 /search', `
      <ul>
        <li>先选版块，再选分类（可搜"全部"）</li>
        <li>输入关键词，模糊列出匹配结果（最多20条）</li>
        <li>搜索机密库需要先验证查询密码</li>
        <li>每一步都有取消按钮</li>
      </ul>`)}
    ${sec('⚙️', '其他', `
      <ul>
        <li>${code('/cancel')} 随时取消当前操作</li>
        <li>5 分钟无对话自动清空聊天记录（设置中可配）</li>
        <li>所有入库操作都会删除原文，只留回执，保护隐私</li>
      </ul>`)}
    </div>`;
}

/* ---------- reminders view ---------- */
let remState = { cats: [], cur: 0 };
async function renderReminders() {
  const v = $('#view');
  v.innerHTML = `
    <div class="topbar">
      <h2>提醒</h2>
      <span style="flex:1"></span>
      <span class="muted" style="font-size:12px">TG 里第一行发"提醒"创建</span>
      <button class="btn sm" id="addRem">${ic('plus')} 新建提醒</button>
    </div>
    <div class="catbar" id="remCatBar"></div>
    <div id="remList"><div class="empty"><div>加载中…</div></div></div>`;
  $('#addRem').onclick = () => openRemModal(null);
  try { remState.cats = await api('GET', '/api/reminder-categories'); } catch (e) { remState.cats = []; }
  if (remState.cur !== 0 && !remState.cats.find(c => c.id === remState.cur)) remState.cur = 0;
  drawRemCatBar();
  await loadReminders();
}
function drawRemCatBar() {
  const bar = $('#remCatBar');
  const cats = [{ id: 0, name: '全部' }, ...remState.cats];
  bar.innerHTML = cats.map(c =>
    `<button class="pill ${c.id === remState.cur ? 'active' : ''}" data-id="${c.id}">${esc(c.name)}</button>`
  ).join('') + `<button class="pill ghost" id="addRemCat">${ic('plus')} 分类</button>`;
  $$('.pill[data-id]', bar).forEach(b => b.onclick = () => { remState.cur = +b.dataset.id; drawRemCatBar(); loadReminders(); });
  $('#addRemCat').onclick = () => {
    openModal(`<h3>新建分类</h3>
      <div class="frow"><input id="catName" placeholder="分类名称" maxlength="20"></div>
      <div class="foot"><button class="btn ghost" data-close>取消</button>
      <button class="btn" id="cfAdd">创建</button></div>`);
    $('#cfAdd').onclick = async () => {
      const name = $('#catName').value.trim();
      if (!name) return;
      const r = await api('POST', '/api/reminder-categories', { name });
      closeModal(); toast('已创建');
      remState.cats = await api('GET', '/api/reminder-categories');
      remState.cur = r.id;
      drawRemCatBar(); loadReminders();
    };
  };
  const cur = remState.cats.find(c => c.id === remState.cur);
  if (cur && cur.name !== '未分类') {
    const mg = document.createElement('span');
    mg.style.marginLeft = '8px';
    mg.innerHTML = `<button class="icon-btn" id="renRemCat" title="重命名分类">${ic('edit')}</button>
      <button class="icon-btn" id="delRemCat" title="删除分类（提醒移到未分类）">${ic('trash')}</button>`;
    bar.appendChild(mg);
    $('#renRemCat').onclick = () => {
      openModal(`<h3>重命名分类</h3>
        <div class="frow"><input id="catName" value="${esc(cur.name)}" maxlength="20"></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfRen">保存</button></div>`);
      $('#cfRen').onclick = async () => {
        const name = $('#catName').value.trim();
        if (!name) return;
        await api('PUT', '/api/reminder-categories/' + cur.id, { name });
        closeModal(); toast('已重命名');
        remState.cats = await api('GET', '/api/reminder-categories');
        drawRemCatBar(); loadReminders();
      };
    };
    $('#delRemCat').onclick = () => {
      openModal(`<h3>删除分类「${esc(cur.name)}」？</h3><p class="muted">里面的提醒会移到"未分类"。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/reminder-categories/' + cur.id);
        closeModal(); toast('已删除');
        remState.cats = await api('GET', '/api/reminder-categories');
        remState.cur = 0;
        drawRemCatBar(); loadReminders();
      };
    };
  }
}
function remRepeatText(r) {
  if (r.repeatType === 'daily') return '每天';
  if (r.repeatType === 'weekly') {
    const wd = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
    const i = +r.repeatDetail;
    return '每周' + (wd[i] || '');
  }
  if (r.repeatType === 'monthly') return '每月' + r.repeatDetail + '号';
  if (r.repeatType === 'yearly') return '每年';
  return '不重复';
}
function fmtRemTime(ts) {
  const d = new Date(ts * 1000);
  const p = n => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
async function loadReminders() {
  const box = $('#remList');
  let list = [];
  try { list = await api('GET', '/api/reminders?categoryId=' + remState.cur); } catch (e) {}
  if (!list.length) {
    box.innerHTML = `<div class="empty">${ic('bell')}<div>还没有提醒，去 TG 发"提醒"创建一个吧</div></div>`;
    return;
  }
  box.innerHTML = list.map(r => `
    <div class="lrow ${r.isActive ? '' : 'dim'}" data-id="${r.id}">
      <div class="grow">
        <div style="font-weight:600">${esc(r.title)}</div>
        <div class="muted" style="font-size:12px">${fmtRemTime(r.remindAt)} · ${remRepeatText(r)} · 连发${r.times}条${r.note ? ' · ' + esc(r.note) : ''}</div>
      </div>
      <span class="tag">${esc(r.categoryName || '')}</span>
      <button class="icon-btn" data-act="toggle" title="${r.isActive ? '暂停' : '启用'}">${ic(r.isActive ? 'pause' : 'play')}</button>
      <button class="icon-btn" data-act="edit" title="编辑">${ic('edit')}</button>
      <button class="icon-btn" data-act="del" title="删除">${ic('trash')}</button>
    </div>`).join('');
  $$('.lrow', box).forEach(row => {
    const id = +row.dataset.id;
    $('[data-act="toggle"]', row).onclick = async () => {
      const r = list.find(x => x.id === id);
      await api('PUT', '/api/reminders/' + id, { isActive: !r.isActive });
      toast(r.isActive ? '已暂停' : '已启用'); loadReminders();
    };
    $('[data-act="edit"]', row).onclick = () => openRemModal(list.find(x => x.id === id));
    $('[data-act="move"]', row).onclick = async () => {
      const nn = await api('GET', '/api/notes/' + id);
      openModal(`<h3>移动到分类</h3>
        <div class="frow"><select id="nCatMv">${noteState.cats.map(c => `<option value="${c.id}"${c.id === nn.categoryId ? ' selected' : ''}>${esc(c.name)}</option>`).join('')}</select></div>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn" id="cfMv">移动</button></div>`);
      $('#cfMv').onclick = async () => {
        await api('PUT', '/api/notes/' + id, { categoryId: +$('#nCatMv').value });
        closeModal(); toast('已移动'); loadNotes();
      };
    };
    $('[data-act="del"]', row).onclick = () => {
      openModal(`<h3>删除这个提醒？</h3><p class="muted">删除后无法恢复。</p>
        <div class="foot"><button class="btn ghost" data-close>取消</button>
        <button class="btn danger" id="cfDel">删除</button></div>`);
      $('#cfDel').onclick = async () => {
        await api('DELETE', '/api/reminders/' + id);
        closeModal(); toast('已删除'); loadReminders();
      };
    };
  });
}
function openRemModal(r) {
  const isNew = !r;
  r = r || {};
  const dt = r.remindAt ? new Date(r.remindAt * 1000) : new Date(Date.now() + 3600000);
  const p = n => String(n).padStart(2, '0');
  const dtLocal = `${dt.getFullYear()}-${p(dt.getMonth() + 1)}-${p(dt.getDate())}T${p(dt.getHours())}:${p(dt.getMinutes())}`;
  const catOpts = remState.cats.map(c => `<option value="${c.id}"${c.id === (r.categoryId || remState.cur) ? ' selected' : ''}>${esc(c.name)}</option>`).join('');
  openModal(`<h3>${isNew ? '新建提醒' : '编辑提醒'}</h3>
    <div class="frow"><label>标题</label><input id="rTitle" value="${esc(r.title || '')}" maxlength="60" placeholder="例如：CF续费"></div>
    <div class="frow"><label>备注</label><input id="rNote" value="${esc(r.note || '')}" maxlength="200" placeholder="可选"></div>
    <div class="frow"><label>分类</label><select id="rCat">${catOpts}</select></div>
    <div class="frow"><label>时间</label><input type="datetime-local" id="rTime" value="${dtLocal}"></div>
    <div class="frow"><label>重复</label><select id="rRepeat">
      <option value="none"${r.repeatType === 'none' ? ' selected' : ''}>不重复</option>
      <option value="daily"${r.repeatType === 'daily' ? ' selected' : ''}>每天</option>
      <option value="weekly"${r.repeatType === 'weekly' ? ' selected' : ''}>每周</option>
      <option value="monthly"${r.repeatType === 'monthly' ? ' selected' : ''}>每月</option>
      <option value="yearly"${r.repeatType === 'yearly' ? ' selected' : ''}>每年</option>
    </select></div>
    <div class="frow" id="rDetailRow" style="display:none"><label>详细</label><input id="rDetail" value="${esc(r.repeatDetail || '')}" placeholder="每周填0-6（周日=0），每月填1-31"></div>
    <div class="frow"><label>连发条数</label><input type="number" id="rTimes" min="1" max="10" value="${r.times || 3}"></div>
    <div class="foot"><button class="btn ghost" data-close>取消</button>
    <button class="btn" id="cfSave">保存</button></div>`, true, true);
  const updDetail = () => {
    const v = $('#rRepeat').value;
    $('#rDetailRow').style.display = (v === 'weekly' || v === 'monthly') ? '' : 'none';
  };
  $('#rRepeat').onchange = updDetail; updDetail();
  $('#cfSave').onclick = async () => {
    const title = $('#rTitle').value.trim();
    if (!title) { toast('标题不能为空'); return; }
    const dtVal = $('#rTime').value;
    if (!dtVal) { toast('请选择时间'); return; }
    const ts = Math.floor(new Date(dtVal).getTime() / 1000);
    const body = {
      title, note: $('#rNote').value.trim(),
      categoryId: +$('#rCat').value, remindAt: ts,
      repeatType: $('#rRepeat').value, repeatDetail: $('#rDetail').value.trim(),
      times: Math.min(10, Math.max(1, +$('#rTimes').value || 3)),
    };
    if (isNew) await api('POST', '/api/reminders', body);
    else await api('PUT', '/api/reminders/' + r.id, body);
    closeModal(); toast('已保存'); loadReminders();
  };
}

// ---- 订阅节点 ----
const protoLabel = p => ({
  vless: 'VLESS', vmess: 'VMess', trojan: 'Trojan', shadowsocks: 'SS',
  shadowsocksr: 'SSR', hysteria2: 'Hysteria2', hysteria: 'Hysteria',
  tuic: 'TUIC', wireguard: 'WireGuard', http: '订阅', https: '订阅'
}[p] || (p ? p.toUpperCase() : '未知'));

function statusDot(n) {
  if (nodeState.testing && !n._tested) return `<span class="dot testing" title="测试中"></span>`;
  if (n.lastStatus === 'ok') return `<span class="dot ok" title="畅通"></span>`;
  if (n.lastStatus === 'fail') return `<span class="dot fail" title="不通"></span>`;
  return `<span class="dot idle" title="未测试"></span>`;
}

async function renderNodes() {
  const v = $('#view');
  v.innerHTML = `
    <div class="topbar">
      <h2>订阅节点</h2>
      <span style="flex:1"></span>
      <span class="muted" style="font-size:12px">TG 里第一行发"订阅/节点/订阅节点"粘贴入库</span>
      <button class="btn ghost sm" id="testAll">${ic('zap')} 测试全部</button>
      <button class="btn sm" id="addN">${ic('plus')} 添加节点</button>
    </div>
    <div class="catbar" id="catBar"></div>
    <div id="nodeList"><div class="empty"><div>加载中…</div></div></div>`;
  $('#addN').onclick = () => nodeModal(null);
  $('#testAll').onclick = () => testNodes(null);
  try { nodeState.cats = await api('GET', '/api/node-categories'); } catch (e) { nodeState.cats = []; }
  if (!nodeState.cats.length) nodeState.cats = [{ id: 0, name: '未分类' }];
  if (nodeState.cur !== 0 && !nodeState.cats.find(c => c.id === nodeState.cur)) nodeState.cur = nodeState.cats[0].id;
  drawNodeCatBar();
  await loadNodes();
  // 打开页面自动测试
  testNodes('auto');
}

function drawNodeCatBar() {
  const bar = $('#catBar');
  const allCats = [{ id: 0, name: '全部' }, ...nodeState.cats];
  bar.innerHTML = allCats.map(c =>
    `<button class="pill ${c.id === nodeState.cur ? 'active' : ''}" data-id="${c.id}">${esc(c.name)}</button>`
  ).join('') + `<button class="pill ghost" id="addCat">${ic('plus')} 分类</button>`;
  $$('.pill[data-id]', bar).forEach(b => b.onclick = () => {
    nodeState.cur = +b.dataset.id; drawNodeCatBar(); loadNodes(true);
  });
  $('#addCat').onclick = () => {
    openModal(`<h3>新建分类</h3>
      <div class="frow"><input id="catName" placeholder="分类名称" maxlength="20"></div>
      <div class="mfoot"><button class="btn ghost" id="mCancel">取消</button><button class="btn" id="mOk">确定</button></div>`);
    $('#mCancel').onclick = closeModal;
    $('#mOk').onclick = async () => {
      const name = $('#catName').value.trim();
      if (!name) return toast('请输入分类名称', 'err');
      try {
        await api('POST', '/api/node-categories', { name });
        nodeState.cats = await api('GET', '/api/node-categories');
        drawNodeCatBar(); toast('已创建');
      } catch (e) { toast('创建失败', 'err'); }
      closeModal();
    };
  };
  // 长按/右键分类可重命名删除（简化：双击）
  $$('.pill[data-id]', bar).forEach(b => {
    b.ondblclick = () => {
      const id = +b.dataset.id;
      if (id === 0) return;
      const c = nodeState.cats.find(x => x.id === id);
      openModal(`<h3>分类：${esc(c.name)}</h3>
        <div class="frow"><input id="catName" value="${esc(c.name)}" maxlength="20"></div>
        <div class="mfoot"><button class="btn danger" id="mDel">删除</button><span style="flex:1"></span><button class="btn ghost" id="mCancel">取消</button><button class="btn" id="mOk">保存</button></div>`);
      $('#mCancel').onclick = closeModal;
      $('#mOk').onclick = async () => {
        const name = $('#catName').value.trim();
        if (!name) return;
        await api('PUT', `/api/node-categories/${id}`, { name });
        nodeState.cats = await api('GET', '/api/node-categories');
        drawNodeCatBar(); closeModal(); toast('已保存');
      };
      $('#mDel').onclick = async () => {
        if (!confirm('删除分类，节点移到未分类？')) return;
        await api('DELETE', `/api/node-categories/${id}`);
        nodeState.cats = await api('GET', '/api/node-categories');
        if (nodeState.cur === id) nodeState.cur = 0;
        drawNodeCatBar(); loadNodes(true); closeModal();
      };
    };
  });
}

async function loadNodes(skipAuto) {
  try {
    nodeState.list = await api('GET', `/api/nodes?categoryId=${nodeState.cur}`);
  } catch (e) { nodeState.list = []; }
  drawNodes();
}

function drawNodes() {
  const el = $('#nodeList');
  if (!nodeState.list.length) {
    el.innerHTML = `<div class="empty"><div>暂无节点</div><div class="muted" style="font-size:12px;margin-top:6px">TG 里发"订阅/节点/订阅节点"粘贴，或点右上添加</div></div>`;
    return;
  }
  el.innerHTML = nodeState.list.map(n => {
    const lat = n.lastCheckAt ? (n.lastStatus === 'ok' ? `<span class="lat ok">${n.lastLatency}ms</span>` : `<span class="lat fail">不通</span>`) : `<span class="lat idle">未测</span>`;
    const time = n.lastCheckAt ? `<span class="muted" style="font-size:11px">${new Date(n.lastCheckAt * 1000).toLocaleString('zh-CN', { hour12: false, month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>` : '';
    return `<div class="nrow" data-id="${n.id}">
      ${statusDot(n)}
      <div class="nmain">
        <div class="ntitle">${esc(n.name)} <span class="ptag">${protoLabel(n.proto)}</span> <span class="ktag">${n.kind === 'sub' ? '订阅' : '节点'}</span></div>
        <div class="nurl muted">${esc(n.url.length > 60 ? n.url.slice(0, 60) + '…' : n.url)}</div>
      </div>
      <div class="nmeta">${lat}<br>${time}</div>
      <button class="icon-btn" data-act="test" title="测试">${ic('zap')}</button>
      <button class="icon-btn" data-act="copy" title="复制链接">${ic('copy')}</button>
      <button class="icon-btn" data-act="edit" title="编辑">${ic('edit')}</button>
      <button class="icon-btn danger" data-act="del" title="删除">${ic('trash')}</button>
    </div>`;
  }).join('');
  $$('.nrow', el).forEach(row => {
    const id = +row.dataset.id;
    const n = nodeState.list.find(x => x.id === id);
    $('[data-act="test"]', row).onclick = () => testNodes([id]);
    $('[data-act="copy"]', row).onclick = () => { navigator.clipboard.writeText(n.url); toast('已复制'); };
    $('[data-act="edit"]', row).onclick = () => nodeModal(n);
    $('[data-act="del"]', row).onclick = async () => {
      if (!confirm(`删除「${n.name}」？`)) return;
      await api('DELETE', `/api/nodes/${id}`);
      nodeState.list = nodeState.list.filter(x => x.id !== id);
      drawNodes(); toast('已删除');
    };
  });
}

async function testNodes(ids) {
  // ids: null=全部, 'auto'=自动(静默), [id]=指定
  const auto = ids === 'auto';
  if (nodeState.testing) return;
  if (!nodeState.list.length) return;
  nodeState.testing = true;
  nodeState.list.forEach(n => n._tested = false);
  if (!auto) drawNodes();
  try {
    const body = ids && ids !== 'auto' ? { ids } : { categoryId: nodeState.cur };
    const results = await api('POST', '/api/nodes-test', body);
    const map = {};
    results.forEach(r => map[r.id] = r);
    nodeState.list.forEach(n => {
      const r = map[n.id];
      if (r) {
        n.lastStatus = r.ok ? 'ok' : 'fail';
        n.lastLatency = r.latency;
        n.lastCheckAt = Math.floor(Date.now() / 1000);
        n._tested = true;
      }
    });
    const okCount = results.filter(r => r.ok).length;
    if (!auto) toast(`测试完成：${okCount}/${results.length} 畅通`);
  } catch (e) {
    if (!auto) toast('测试失败', 'err');
  }
  nodeState.testing = false;
  drawNodes();
}

function nodeModal(n) {
  const cats = nodeState.cats.filter(c => c.id !== 0);
  openModal(`<h3>${n ? '编辑' : '添加'}节点/订阅</h3>
    <div class="frow"><label>名称</label><input id="nName" value="${n ? esc(n.name) : ''}" placeholder="可选，不填自动识别" maxlength="50"></div>
    <div class="frow"><label>链接</label><input id="nUrl" value="${n ? esc(n.url) : ''}" placeholder="vless://… 或 https://订阅链接" ${n ? 'disabled' : ''}></div>
    <div class="frow"><label>分类</label><select id="nCat">${cats.map(c => `<option value="${c.id}" ${n && n.categoryId === c.id ? 'selected' : ''}>${esc(c.name)}</option>`).join('')}</select></div>
    <div class="mfoot"><button class="btn ghost" id="mCancel">取消</button><button class="btn" id="mOk">保存</button></div>`);
  $('#mCancel').onclick = closeModal;
  $('#mOk').onclick = async () => {
    const name = $('#nName').value.trim();
    const catId = +$('#nCat').value;
    try {
      if (n) {
        await api('PUT', `/api/nodes/${n.id}`, { name, categoryId: catId });
      } else {
        const url = $('#nUrl').value.trim();
        if (!url) return toast('请输入链接', 'err');
        await api('POST', '/api/nodes', { name, url, categoryId: catId });
      }
      closeModal(); toast('已保存'); loadNodes(true);
    } catch (e) { toast('保存失败', 'err'); }
  };
}

/* ============ SSH ============ */
const sshState = { cats: [], cur: 0, list: [] };

// 国家 -> 两位代码（用于 flagcdn 国旗图片）
const COUNTRY_CODES = {
  'CN': 'cn', '中国': 'cn', 'HK': 'hk', '香港': 'hk', 'TW': 'tw', '台湾': 'tw',
  'US': 'us', '美国': 'us', 'SG': 'sg', '新加坡': 'sg', 'JP': 'jp', '日本': 'jp',
  'DE': 'de', '德国': 'de', 'UK': 'gb', '英国': 'gb', 'FR': 'fr', '法国': 'fr',
  'KR': 'kr', '韩国': 'kr', 'NL': 'nl', '荷兰': 'nl', 'CA': 'ca', '加拿大': 'ca',
  'AU': 'au', '澳大利亚': 'au', 'RU': 'ru', '俄罗斯': 'ru', 'IN': 'in', '印度': 'in',
};
// 系统 LOGO（Simple Icons CDN）
function osLogo(osId, osVer) {
  if (!osId) return '';
  const m = { debian: 'debian', ubuntu: 'ubuntu', centos: 'centos', rhel: 'redhat', rocky: 'rockylinux', alma: 'almalinux', fedora: 'fedora', arch: 'archlinux', alpine: 'alpinelinux', opensuse: 'opensuse', gentoo: 'gentoo' };
  const slug = m[osId.toLowerCase()];
  if (!slug) return '';
  const title = osVer ? `${osId} ${osVer}` : osId;
  return `<img src="https://cdn.simpleicons.org/${slug}" alt="${esc(osId)}" title="${esc(title)}" style="width:22px;height:22px;opacity:.85">`;
}
function countryFlag(c) {
  if (!c) return '';
  const code = COUNTRY_CODES[c.trim()] || COUNTRY_CODES[c.trim().toUpperCase()];
  if (!code) return '';
  return `<img src="https://flagcdn.com/w40/${code}.png" class="flag-img" alt="${esc(c)}">`;
}

async function renderSSH() {
  const v = $('#view');
  v.innerHTML = `
    <div class="topbar">
      <h2>SSH</h2>
      <span style="flex:1"></span>
      <span class="muted" style="font-size:12px">TG 里第一行发"SSH"按格式粘贴入库</span>
      <button class="btn ghost sm" id="sshTestAll">${ic('zap')} 测试全部</button>
      <button class="btn sm" id="sshAdd">${ic('plus')} 添加主机</button>
    </div>
    <div class="catbar" id="sshCatBar"></div>
    <div id="sshList"><div class="empty"><div>加载中…</div></div></div>`;
  $('#sshAdd').onclick = () => sshModal(null);
  $('#sshTestAll').onclick = () => sshTestAll();
  try { sshState.cats = await api('GET', '/api/ssh-categories'); } catch (e) { sshState.cats = []; }
  if (!sshState.cats.length) sshState.cats = [{ id: 0, name: '未分类' }];
  if (sshState.cur !== 0 && !sshState.cats.find(c => c.id === sshState.cur)) sshState.cur = sshState.cats[0].id;
  drawSSHCatBar();
  await loadSSHHosts();
}

function drawSSHCatBar() {
  const bar = $('#sshCatBar');
  const allCats = [{ id: 0, name: '全部' }, ...sshState.cats];
  bar.innerHTML = allCats.map(c =>
    `<button class="pill ${c.id === sshState.cur ? 'active' : ''}" data-id="${c.id}">${esc(c.name)}</button>`
  ).join('') + `<button class="pill ghost" id="sshAddCat">${ic('plus')} 分类</button>`;
  $$('.pill[data-id]', bar).forEach(b => b.onclick = () => {
    sshState.cur = +b.dataset.id; drawSSHCatBar(); loadSSHHosts();
  });
  $('#sshAddCat').onclick = () => {
    openModal(`<h3>新建分类</h3>
      <div class="frow"><input id="catName" placeholder="分类名称" maxlength="20"></div>
      <div class="mfoot"><button class="btn ghost" id="mCancel">取消</button><button class="btn" id="mOk">确定</button></div>`);
    $('#mCancel').onclick = closeModal;
    $('#mOk').onclick = async () => {
      const name = $('#catName').value.trim();
      if (!name) return toast('请输入分类名称', 'err');
      try {
        await api('POST', '/api/ssh-categories', { name });
        sshState.cats = await api('GET', '/api/ssh-categories');
        drawSSHCatBar(); toast('已创建');
      } catch (e) { toast('创建失败', 'err'); }
      closeModal();
    };
  };
  $$('.pill[data-id]', bar).forEach(b => {
    b.ondblclick = () => {
      const id = +b.dataset.id;
      if (id === 0) return;
      const c = sshState.cats.find(x => x.id === id);
      openModal(`<h3>分类：${esc(c.name)}</h3>
        <div class="frow"><input id="catName" value="${esc(c.name)}" maxlength="20"></div>
        <div class="mfoot"><button class="btn danger" id="mDel">删除</button><span style="flex:1"></span><button class="btn ghost" id="mCancel">取消</button><button class="btn" id="mOk">保存</button></div>`);
      $('#mCancel').onclick = closeModal;
      $('#mOk').onclick = async () => {
        const name = $('#catName').value.trim();
        if (!name) return;
        await api('PUT', `/api/ssh-categories/${id}`, { name });
        sshState.cats = await api('GET', '/api/ssh-categories');
        drawSSHCatBar(); closeModal(); toast('已保存');
      };
      $('#mDel').onclick = async () => {
        if (!confirm('删除分类，主机移到未分类？')) return;
        await api('DELETE', `/api/ssh-categories/${id}`);
        sshState.cats = await api('GET', '/api/ssh-categories');
        if (sshState.cur === id) sshState.cur = 0;
        drawSSHCatBar(); loadSSHHosts(); closeModal();
      };
    };
  });
}

async function loadSSHHosts() {
  try {
    sshState.list = await api('GET', `/api/ssh-hosts?categoryId=${sshState.cur}`);
  } catch (e) { sshState.list = []; }
  drawSSHHosts();
  startSSHCardRefresh();
}

function sshStatusDot(h) {
  let dot = `<span class="dot idle"></span>`;
  if (h.lastStatus === 'ok') dot = `<span class="dot ok"></span>`;
  if (h.lastStatus === 'fail') dot = `<span class="dot fail"></span>`;
  const lat = h.lastLatency ? `<small class="muted">${h.lastLatency}ms</small>` : '';
  const ns = sshNetSpeed[h.id] ? `<small class="muted" style="margin-left:6px">↑${sshNetSpeed[h.id].tx} ↓${sshNetSpeed[h.id].rx}</small>` : '';
  return `<span class="ssh-status">${dot}${lat}${ns}</span>`;
}
// 卡片网速缓存 + 5秒刷新
let sshNetSpeed = {};
let sshCardTimer = null;
function startSSHCardRefresh() {
  if (sshCardTimer) clearInterval(sshCardTimer);
  const tick = async () => {
    if (!sshState.list || !sshState.list.length) return;
    // 只在 SSH 页面可见时刷
    if ($('#view-ssh') && $('#view-ssh').style.display === 'none') return;
    for (const h of sshState.list) {
      try {
        const r = await api('GET', `/api/ssh-netspeed?id=${h.id}`);
        if (r.netspeed) {
          sshNetSpeed[h.id] = {
            tx: fmtNet(r.netspeed.txKBs),
            rx: fmtNet(r.netspeed.rxKBs)
          };
        }
      } catch (e) {}
    }
    drawSSHHosts();
  };
  tick();
  sshCardTimer = setInterval(tick, 5000);
}
function fmtNet(kbs) {
  if (kbs >= 1024) return (kbs/1024).toFixed(1) + 'M/s';
  return kbs.toFixed(1) + 'K/s';
}

function drawSSHHosts() {
  const el = $('#sshList');
  if (!sshState.list.length) {
    el.innerHTML = `<div class="empty"><div>暂无主机</div><div class="muted" style="font-size:12px;margin-top:6px">TG 里发"SSH"按格式粘贴，或点右上添加</div></div>`;
    return;
  }
  el.innerHTML = `<div class="ssh-grid">` + sshState.list.map(h => `
    <div class="ssh-card" data-id="${h.id}" style="position:relative">
      <div class="ssh-card-head">
        <span class="ssh-flag">${countryFlag(h.country)}${h.region ? `<small style="font-size:13px;color:var(--text)">${esc(h.region)}</small>` : ''}</span>
        <div class="ssh-card-title">
          <b>${esc(h.name || h.host)}</b>
          <small class="muted">${esc(h.host)}:${h.port}</small>
        </div>
        ${sshStatusDot(h)}
      </div>
      ${h.osId ? `<div style="position:absolute;right:10px;bottom:10px">${osLogo(h.osId, h.osVersion)}</div>` : ''}
      <div class="ssh-card-actions">
        <button class="btn sm" data-act="connect">${ic('terminal')} 连接</button>
        <button class="btn ghost sm" data-act="edit">${ic('edit')}</button>
        <button class="btn ghost sm danger" data-act="del">${ic('trash')}</button>
      </div>
    </div>`).join('') + `</div>`;
  $$('.ssh-card', el).forEach(card => {
    const id = +card.dataset.id;
    const h = sshState.list.find(x => x.id === id);
    card.querySelector('[data-act="connect"]').onclick = () => sshConnect(h);
    card.querySelector('[data-act="edit"]').onclick = () => sshModal(h);
    card.querySelector('[data-act="del"]').onclick = async () => {
      if (!confirm(`删除主机 ${h.name || h.host}？`)) return;
      await api('DELETE', `/api/ssh-hosts/${id}`);
      loadSSHHosts(); toast('已删除');
    };
  });
}

async function sshTestAll() {
  const ids = sshState.list.map(h => h.id);
  if (!ids.length) return toast('没有主机', 'err');
  toast('测试中…');
  try {
    await api('POST', '/api/ssh-test', { ids });
    await loadSSHHosts();
    toast('测试完成');
  } catch (e) { toast('测试失败', 'err'); }
}

function sshModal(h) {
  const isEdit = !!h;
  const cats = sshState.cats.filter(c => c.id !== 0);
  openModal(`<h3>${isEdit ? '编辑主机' : '添加主机'}</h3>
    <div class="frow"><label>名称</label><input id="sshName" value="${esc(h?.name || '')}" placeholder="给主机起个名"></div>
    <div class="frow"><label>分类</label><select id="sshCat">${cats.map(c => `<option value="${c.id}" ${h && h.categoryId === c.id ? 'selected' : ''}>${esc(c.name)}</option>`).join('')}</select></div>
    <div class="frow"><label>主机</label><input id="sshHost" value="${esc(h?.host || '')}" placeholder="IP 或域名，IPv6 直接填"></div>
    <div class="frow"><label>端口</label><input id="sshPort" type="number" value="${h?.port || 22}"></div>
    <div class="frow"><label>用户名</label><input id="sshUser" value="${esc(h?.username || 'root')}"></div>
    <div class="frow"><label>密码${isEdit ? '（留空不改）' : ''}</label><input id="sshPass" type="password" placeholder="${isEdit ? '留空不修改' : 'SSH 密码'}"></div>
    <div class="frow"><label>国家</label><input id="sshCountry" value="${esc(h?.country || '')}" placeholder="如 中国 / 美国 / CN / US"></div>
    <div class="frow"><label>地区</label><input id="sshRegion" value="${esc(h?.region || '')}" placeholder="如 西雅图 / 圣何塞"></div>
    <div class="mfoot"><button class="btn ghost" id="mCancel">取消</button><button class="btn" id="mOk">保存</button></div>`);
  $('#mCancel').onclick = closeModal;
  $('#mOk').onclick = async () => {
    const data = {
      categoryId: +$('#sshCat').value,
      name: $('#sshName').value.trim(),
      host: $('#sshHost').value.trim(),
      port: +$('#sshPort').value || 22,
      username: $('#sshUser').value.trim() || 'root',
      password: $('#sshPass').value,
      country: $('#sshCountry').value.trim(),
      region: $('#sshRegion').value.trim(),
    };
    if (!data.host) return toast('请填写主机', 'err');
    if (!isEdit && !data.password) return toast('请填写密码', 'err');
    try {
      if (isEdit) await api('PUT', `/api/ssh-hosts/${h.id}`, data);
      else await api('POST', '/api/ssh-hosts', data);
      closeModal(); loadSSHHosts(); toast('已保存');
    } catch (e) { toast('保存失败', 'err'); }
  };
}

// ============ SSH 终端 ============
// xterm.js 浅色主题：白底黑字，带彩色
const XTERM_LIGHT_THEME = {
  background: '#ffffff',
  foreground: '#1a1a1a',
  cursor: '#1a1a1a',
  selectionBackground: '#b3d7ff',
  black: '#1a1a1a', red: '#c41a1a', green: '#1a8c1a', yellow: '#8c6a1a',
  blue: '#1a1ac4', magenta: '#8c1a8c', cyan: '#1a8c8c', white: '#e0e0e0',
  brightBlack: '#666666', brightRed: '#e01a1a', brightGreen: '#1ac41a', brightYellow: '#c4a01a',
  brightBlue: '#1a1ae0', brightMagenta: '#c41ac4', brightCyan: '#1ac4c4', brightWhite: '#ffffff',
};

function loadXterm() {
  return new Promise((resolve, reject) => {
    if (window.Terminal) {
      if (window.FitAddon) return resolve();
      const sc2 = document.createElement('script');
      sc2.src = 'https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.js';
      sc2.onload = resolve;
      sc2.onerror = () => resolve();
      document.head.appendChild(sc2);
      return;
    }
    const css = document.createElement('link');
    css.rel = 'stylesheet';
    css.href = 'https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.css';
    document.head.appendChild(css);
    const sc = document.createElement('script');
    sc.src = 'https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.js';
    sc.onload = () => {
      const sc2 = document.createElement('script');
      sc2.src = 'https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.js';
      sc2.onload = resolve;
      sc2.onerror = () => resolve();
      document.head.appendChild(sc2);
    };
    sc.onerror = reject;
    document.head.appendChild(sc);
  });
}
let sshTermFit = null;

let sshTerms = {}; // id -> {term, ws, host, path, heartbeat, minimized, fit, pingTs}
let activeSshId = null;
let sshTermResizeHandler = null;

// 状态点颜色：<150绿 151-300黄 300+橙 断开红
function sshConnColor(rtt) {
  if (rtt == null) return 'fail';
  if (rtt <= 150) return 'ok';
  if (rtt <= 300) return 'warn';
  return 'orange';
}

async function sshConnect(h) {
  const id = h.id;
  // 对话框被意外销毁但会话还在：清理重建
  if (sshTerms[id] && !document.querySelector('.ssh-win')) {
    const s = sshTerms[id];
    try { s.ws.close(); } catch (e) {}
    try { s.term.dispose(); } catch (e) {}
    delete sshTerms[id];
  }
  // 已有会话：切换到它（确保对话框可见）
  if (sshTerms[id]) {
    ensureSshDialogVisible();
    if (sshTerms[id].minimized) restoreSshTerm(id);
    else switchSshTab(id);
    return;
  }
  try { await loadXterm(); }
  catch (e) { return toast('终端组件加载失败', 'err'); }

  // 首次打开：建对话框框架
  if (!document.querySelector('.ssh-win')) {
    openModal(`<div class="ssh-win">
      <div class="ssh-tabs" id="sshTabs"></div>
      <div class="ssh-win-body" id="sshBody"></div>
    </div>`, true, true, 'ssh-modal');
    let rszT = null;
    sshTermResizeHandler = () => {
      clearTimeout(rszT);
      rszT = setTimeout(() => {
        const bg = document.querySelector('.modal-bg');
        if (bg && bg.style.display === 'none') return;
        const s = sshTerms[activeSshId];
        if (s && !s.minimized) sshFitTerm(activeSshId);
      }, 150);
    };
    window.addEventListener('resize', sshTermResizeHandler);
  }

  // 建标签页内容
  const body = $('#sshBody');
  const pane = document.createElement('div');
  pane.className = 'ssh-pane';
  pane.id = `sshPane${id}`;
  pane.innerHTML = `
    <div class="ssh-win-head" style="flex-wrap:wrap;row-gap:8px">
      <span class="ssh-win-title"><small class="muted">${esc(h.username)}@${esc(h.host)}:${h.port}</small></span>
      <span class="ssh-conn" id="sshConn${id}" title="连接状态"><span class="dot idle"></span><small class="muted" id="sshRtt${id}"></small></span>
      <span style="flex:1;min-width:20px"></span>
      <button class="icon-btn" data-ssh-reconnect="${id}" title="重连" style="flex-shrink:0">${ic('refresh')}</button>
      <button class="icon-btn" data-ssh-minall title="最小化" style="flex-shrink:0">${ic('minus')}</button>
      <button class="icon-btn" data-ssh-close="${id}" title="关闭" style="flex-shrink:0">${ic('x')}</button>
    </div>
    <div class="ssh-pane-body">
      <div class="ssh-side">
        <div class="ssh-panel">
          <div class="ssh-panel-title">系统状态</div>
          <div id="sshStats${id}"><div class="muted" style="font-size:12px">连接中…</div></div>
          <div id="sshNet${id}" style="font-size:12px;color:var(--muted);margin-top:6px"></div>
        </div>
        <div class="ssh-panel ssh-files-panel">
          <div class="ssh-panel-title">文件 <span style="flex:1"></span><button class="btn ghost sm" data-ssh-upload="${id}">上传</button></div>
          <div class="ssh-pathbar"><button class="btn ghost sm" data-ssh-up="${id}">↑</button><span id="sshPath${id}">/root</span></div>
          <div id="sshFiles${id}"><div class="muted" style="font-size:12px">加载中…</div></div>
          <input type="file" id="sshUpload${id}" style="display:none">
        </div>
      </div>
      <div class="ssh-main">
        <div id="sshTermEl${id}"></div>
        <div class="ssh-cmdbar">
          <input id="sshCmd${id}" placeholder="输入命令回车发送（支持粘贴）" style="flex:1">
          <button class="btn sm" data-ssh-send="${id}">发送</button>
        </div>
      </div>
    </div>`;
  body.appendChild(pane);

  const term = new Terminal({
    theme: XTERM_LIGHT_THEME,
    fontSize: 14,
    cols: 94,
    rows: 24,
    fontFamily: 'Menlo, Consolas, "Courier New", monospace',
    cursorBlink: true,
  });
  let fit = null;
  try {
    if (window.FitAddon) {
      fit = new FitAddon.FitAddon();
      term.loadAddon(fit);
    }
  } catch (e) {}
  term.open($('#sshTermEl' + id));

  const sess = { term, fit, host: h, path: '/root', minimized: false, heartbeat: null, pingTs: 0, ws: null };
  sshTerms[id] = sess;

  // 绑定按钮
  pane.querySelector('[data-ssh-minall]').onclick = () => minimizeAllSsh();
  pane.querySelector(`[data-ssh-close="${id}"]`).onclick = () => closeSshTerm(id);
  pane.querySelector(`[data-ssh-reconnect="${id}"]`).onclick = () => reconnectSsh(id);
  pane.querySelector(`[data-ssh-up="${id}"]`).onclick = () => {
    const p = sess.path;
    loadSSHFiles(id, p === '/' ? '/' : p.substring(0, p.lastIndexOf('/')) || '/');
  };
  pane.querySelector(`[data-ssh-upload="${id}"]`).onclick = () => $('#sshUpload' + id).click();
  $('#sshUpload' + id).onchange = (e) => sshUploadFile(id, e.target.files[0]);
  const sendCmd = () => {
    const inp = $('#sshCmd' + id);
    const cmd = inp.value;
    if (!cmd) return;
    if (sess.ws && sess.ws.readyState === 1) sess.ws.send(JSON.stringify({ type: 'input', data: cmd + '\n' }));
    else reconnectSsh(id); // 断线时回车=重连
    inp.value = '';
    term.focus();
  };
  pane.querySelector(`[data-ssh-send="${id}"]`).onclick = sendCmd;
  $('#sshCmd' + id).onkeydown = (e) => { if (e.key === 'Enter') sendCmd(); };
  // 终端回车重连
  term.onData(d => {
    if (sess.ws && sess.ws.readyState === 1) {
      sess.ws.send(JSON.stringify({ type: 'input', data: d }));
    } else if (d === '\r') {
      reconnectSsh(id);
    }
  });

  ensureSshDialogVisible();
  switchSshTab(id);
  const doFit = () => sshFitTerm(id);
  setTimeout(doFit, 100);
  setTimeout(doFit, 400);
  setTimeout(doFit, 800);
  // 容器尺寸变化自动 fit
  if (window.ResizeObserver) {
    const ro = new ResizeObserver(() => sshFitTerm(id));
    ro.observe($('#sshTermEl' + id));
    sess._ro = ro;
  }
  connectSshWs(id);
  // 连接成功后自动识别系统
  setTimeout(async () => {
    try {
      const r = await api('GET', `/api/ssh-detect-os?id=${id}`);
      if (r.os && r.os.id) loadSSHHosts();
    } catch (e) {}
  }, 3000);
}

// 适配终端尺寸：按容器实际宽度算，不撑出对话框
function sshFitTerm(id) {
  const sess = sshTerms[id];
  if (!sess || !sess.term) return;
  const el = document.getElementById('sshTermEl' + id);
  let cols = 80, rows = 24;
  if (el && el.clientWidth > 100) {
    cols = Math.max(70, Math.min(110, Math.floor(el.clientWidth / 7.5)));
    rows = Math.max(20, Math.min(40, Math.floor(el.clientHeight / 17)));
  }
  try {
    sess.term.resize(cols, rows);
    if (sess.ws && sess.ws.readyState === 1)
      sess.ws.send(JSON.stringify({ type: 'resize', cols, rows }));
  } catch (e) {}
}
function ensureSshDialogVisible() {
  const bg = document.querySelector('.modal-bg');
  if (bg && bg.style.display === 'none') bg.style.display = '';
  drawSshMinBars();
}
function connectSshWs(id) {
  const sess = sshTerms[id];
  if (!sess) return;
  const h = sess.host;
  const term = sess.term;
  // 关旧连接
  if (sess.ws) { try { sess.ws.close(); } catch (e) {} }
  stopSshHeartbeat(id);

  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(`${proto}//${location.host}/ws/ssh?id=${id}&cols=120&rows=30`);
  sess.ws = ws;

  ws.onopen = () => {
    term.writeln('\x1b[32m连接成功。\x1b[0m');
    setSshConn(id, 'ok', '');
    startSshHeartbeat(id);
    loadSSHStats(id);
    loadSSHFiles(id, sess.path);
  };
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data);
      if (msg.type === 'output') term.write(msg.data);
      else if (msg.type === 'error') term.writeln(`\r\n\x1b[31m${esc(msg.data)}\x1b[0m`);
      else if (msg.type === 'pong') handleSshPong(id, msg.data);
    } catch (e) {}
  };
  ws.onclose = () => {
    setSshConn(id, 'fail', '断开');
    stopSshHeartbeat(id);
    term.writeln('\r\n\x1b[31m连接已断开，按回车重连\x1b[0m');
  };
  ws.onerror = () => term.writeln('\r\n\x1b[31m连接出错\x1b[0m');
  term.onResize(({ cols, rows }) => {
    if (ws.readyState === 1) ws.send(JSON.stringify({ type: 'resize', cols, rows }));
  });
}

function reconnectSsh(id) {
  const sess = sshTerms[id];
  if (!sess) return;
  sess.term.writeln('\r\n\x1b[33m重连中…\x1b[0m');
  connectSshWs(id);
}

function switchSshTab(id) {
  activeSshId = id;
  // 标签页
  const tabs = $('#sshTabs');
  if (tabs) {
    tabs.innerHTML = Object.keys(sshTerms).map(k => {
      const s = sshTerms[k];
      return `<button class="ssh-tab ${+k === id ? 'active' : ''}" data-tab="${k}">${countryFlag(s.host.country)} ${esc(s.host.name || s.host.host)} <span class="dot ${sshDotClass(k)}" style="width:8px;height:8px"></span></button>`;
    }).join('');
    tabs.querySelectorAll('[data-tab]').forEach(b => b.onclick = () => switchSshTab(+b.dataset.tab));
  }
  // 内容
  document.querySelectorAll('.ssh-pane').forEach(p => p.style.display = 'none');
  const pane = $('#sshPane' + id);
  if (pane) pane.style.display = '';
  const sess = sshTerms[id];
  if (sess) setTimeout(() => sshFitTerm(id), 80);
  if (sess) sess.term.focus();
}

function sshDotClass(id) {
  const el = $('#sshConn' + id + ' .dot');
  if (!el) return 'idle';
  if (el.classList.contains('ok')) return 'ok';
  if (el.classList.contains('warn')) return 'warn';
  if (el.classList.contains('orange')) return 'orange';
  if (el.classList.contains('fail')) return 'fail';
  return 'idle';
}

function minimizeSshTerm(id) {
  const sess = sshTerms[id];
  if (!sess) return;
  sess.minimized = true;
  drawSshMinBars();
}

// 最小化全部：一键
function minimizeAllSsh() {
  Object.keys(sshTerms).forEach(k => { sshTerms[k].minimized = true; });
  const bg = document.querySelector('.modal-bg');
  if (bg) bg.style.display = 'none';
  activeSshId = null;
  drawSshMinBars();
}

function restoreSshTerm(id) {
  const sess = sshTerms[id];
  if (!sess) return;
  // 恢复全部最小化的（点悬浮条恢复整个对话框）
  Object.keys(sshTerms).forEach(k => { sshTerms[k].minimized = false; });
  const bg = document.querySelector('.modal-bg');
  if (bg) bg.style.display = '';
  drawSshMinBars();
  switchSshTab(id);
  // 恢复后重算终端尺寸
  const s = sshTerms[id];
  if (s) setTimeout(() => sshFitTerm(id), 100);
}

// 右下角竖向堆叠
function drawSshMinBars() {
  let wrap = $('#sshMinWrap');
  if (!wrap) {
    wrap = document.createElement('div');
    wrap.id = 'sshMinWrap';
    document.body.appendChild(wrap);
  }
  const mins = Object.keys(sshTerms).filter(k => sshTerms[k].minimized);
  // 按创建时间排序（先开的在上）
  wrap.innerHTML = mins.map(k => {
    const s = sshTerms[k];
    return `<div class="ssh-minbar" data-restore="${k}">${countryFlag(s.host.country)} <b>${esc(s.host.name || s.host.host)}</b> <span class="dot ${sshDotClass(k)}"></span></div>`;
  }).join('');
  wrap.querySelectorAll('[data-restore]').forEach(b => b.onclick = () => restoreSshTerm(+b.dataset.restore));
}

function closeSshTerm(id) {
  const ids = Object.keys(sshTerms);
  if (ids.length <= 1) { doCloseSshTerm(id); return; }
  // 三选一内联菜单
  let menu = $('#sshCloseMenu');
  if (menu) menu.remove();
  menu = document.createElement('div');
  menu.id = 'sshCloseMenu';
  menu.innerHTML = `<button data-c="one">关闭当前连接</button><button data-c="all">关闭全部连接</button><button data-c="cancel">取消</button>`;
  document.body.appendChild(menu);
  // 定位到点击的 X 按钮附近
  const btn = document.querySelector(`[data-ssh-close="${id}"]`);
  if (btn) {
    const r = btn.getBoundingClientRect();
    menu.style.top = (r.bottom + 6) + 'px';
    menu.style.right = (window.innerWidth - r.right) + 'px';
  }
  const close = () => { menu.remove(); document.removeEventListener('click', onDoc); };
  const onDoc = (e) => { if (!menu.contains(e.target)) close(); };
  setTimeout(() => document.addEventListener('click', onDoc), 10);
  menu.querySelector('[data-c="one"]').onclick = () => { close(); doCloseSshTerm(id); };
  menu.querySelector('[data-c="all"]').onclick = () => { close(); [...ids].forEach(k => doCloseSshTerm(+k)); };
  menu.querySelector('[data-c="cancel"]').onclick = close;
}

function doCloseSshTerm(id) {
  const sess = sshTerms[id];
  if (sess) {
    stopSshHeartbeat(id);
    try { sess.statsTimer && clearInterval(sess.statsTimer); } catch (e) {}
    try { sess.netTimer && clearInterval(sess.netTimer); } catch (e) {}
    try { sess._ro && sess._ro.disconnect(); } catch (e) {}
    try { sess.ws.close(); } catch (e) {}
    try { sess.term.dispose(); } catch (e) {}
    delete sshTerms[id];
  }
  const pane = $('#sshPane' + id);
  if (pane) pane.remove();
  drawSshMinBars();
  const allIds = Object.keys(sshTerms);
  // 所有会话都关了才销毁对话框
  if (!allIds.length) {
    if (sshTermResizeHandler) { window.removeEventListener('resize', sshTermResizeHandler); sshTermResizeHandler = null; }
    const bg = document.querySelector('.modal-bg');
    if (bg) bg.remove();
    const wrap = $('#sshMinWrap');
    if (wrap) wrap.remove();
    activeSshId = null;
  } else {
    // 还有会话：切到第一个可见的
    const visible = allIds.filter(k => !sshTerms[k].minimized);
    if (visible.length) switchSshTab(+visible[0]);
    else ensureSshDialogVisible();
  }
}

// 心跳
function startSshHeartbeat(id) {
  const sess = sshTerms[id];
  if (!sess) return;
  if (sess.heartbeat) clearInterval(sess.heartbeat);
  sess.heartbeat = setInterval(() => {
    if (!sshTerms[id] || !sess.ws || sess.ws.readyState !== 1) {
      setSshConn(id, 'fail', '断开');
      return;
    }
    const ts = Date.now();
    sess.pingTs = ts;
    sess.ws.send(JSON.stringify({ type: 'ping', data: String(ts) }));
    setTimeout(() => {
      if (sshTerms[id] && sess.pingTs === ts) setSshConn(id, 'warn', '丢包');
    }, 3000);
  }, 5000);
}
function stopSshHeartbeat(id) {
  const sess = sshTerms[id];
  if (sess && sess.heartbeat) { clearInterval(sess.heartbeat); sess.heartbeat = null; }
}
function setSshConn(id, color, rtt) {
  const el = $('#sshConn' + id);
  if (!el) return;
  const dot = el.querySelector('.dot');
  dot.className = 'dot ' + color;
  const rttEl = $('#sshRtt' + id);
  if (rttEl) rttEl.textContent = rtt || '';
  drawSshMinBars();
}
function handleSshPong(id, ts) {
  if (!sshTerms[id]) return;
  const rtt = Date.now() - parseInt(ts);
  sshTerms[id].pingTs = 0;
  setSshConn(id, sshConnColor(rtt), rtt + 'ms');
}


async function loadSSHStats(id) {
  const sess = sshTerms[id];
  // 清旧定时器
  if (sess && sess.statsTimer) clearInterval(sess.statsTimer);
  const fetch = async () => {
  try {
    const r = await api('GET', `/api/ssh-stats?id=${id}`);
    const st = r.stats;
    const bar = (pct) => {
      const c = pct > 80 ? '#e01a1a' : pct > 60 ? '#c4a01a' : '#1a8c1a';
      return `<div class="stat-bar"><div style="width:${Math.min(pct,100)}%;background:${c}"></div></div>`;
    };
    const el = $('#sshStats' + id);
    if (!el) return;
    el.innerHTML = `
      <div class="stat-row"><span>CPU</span><b>${st.cpuPercent.toFixed(1)}%</b></div>${bar(st.cpuPercent)}
      <div class="stat-row"><span>内存</span><b>${st.memUsedMB}/${st.memTotalMB}MB (${st.memPercent.toFixed(0)}%)</b></div>${bar(st.memPercent)}
      <div class="stat-row"><span>磁盘</span><b>${st.diskUsedGB.toFixed(1)}/${st.diskTotalGB.toFixed(0)}G (${st.diskPercent.toFixed(0)}%)</b></div>${bar(st.diskPercent)}
      <div class="muted" style="font-size:11px;margin-top:4px">${esc(st.uptime)}</div>
      ${r.procs && r.procs.length ? `<div style="margin-top:8px;font-size:12px"><b>进程</b></div>` + r.procs.map(p =>
        `<div class="proc-row"><span>${p.cpu}%</span><span>${esc(p.cmd.slice(0,20))}</span></div>`).join('') : ''}`;
  } catch (e) {
    const el = $('#sshStats' + id);
    if (el) el.innerHTML = `<div class="muted" style="font-size:12px">状态获取失败</div>`;
  }
  };
  await fetch();
  // 每10秒刷新
  if (sess) sess.statsTimer = setInterval(() => { if (sshTerms[id]) fetch(); }, 10000);
  // 网速每3秒刷新
  const netFetch = async () => {
    try {
      const r = await api('GET', `/api/ssh-netspeed?id=${id}`);
      const el = $('#sshNet' + id);
      if (el && r.netspeed) {
        el.innerHTML = `↑ ${fmtNet(r.netspeed.txKBs)} ↓ ${fmtNet(r.netspeed.rxKBs)}`;
      }
    } catch (e) {}
  };
  await netFetch();
  if (sess) sess.netTimer = setInterval(() => { if (sshTerms[id]) netFetch(); }, 3000);
}

async function loadSSHFiles(id, path) {
  const sess = sshTerms[id];
  if (sess) sess.path = path;
  const pathEl = $('#sshPath' + id);
  if (pathEl) pathEl.textContent = path;
  const filesEl = $('#sshFiles' + id);
  if (!filesEl) return;
  try {
    const r = await api('GET', `/api/ssh-files?id=${id}&path=${encodeURIComponent(path)}`);
    const files = r.files || [];
    files.sort((a, b) => (b.isDir - a.isDir) || a.name.localeCompare(b.name));
    filesEl.innerHTML = files.map((f, i) => `
      <div class="file-row" data-i="${i}">
        <span>${f.isDir ? '📁' : '📄'}</span>
        <span class="file-name">${esc(f.name)}</span>
        ${!f.isDir ? `<button class="btn ghost sm" data-dl="${i}" title="下载">↓</button>` : ''}
      </div>`).join('') || `<div class="muted" style="font-size:12px">空目录</div>`;
    filesEl.querySelectorAll('.file-row').forEach(row => {
      const f = files[+row.dataset.i];
      if (f.isDir) {
        row.onclick = () => loadSSHFiles(id, (path === '/' ? '' : path) + '/' + f.name);
      } else {
        row.querySelector('[data-dl]').onclick = (e) => {
          e.stopPropagation();
          window.open(`/api/ssh-download?id=${id}&path=${encodeURIComponent((path === '/' ? '' : path) + '/' + f.name)}`, '_blank');
        };
      }
    });
  } catch (e) {
    filesEl.innerHTML = `<div class="muted" style="font-size:12px">加载失败</div>`;
  }
}

async function sshUploadFile(id, file) {
  if (!file) return;
  const sess = sshTerms[id];
  toast('上传中…');
  const reader = new FileReader();
  reader.onload = async () => {
    const b64 = reader.result.split(',')[1];
    try {
      await api('POST', '/api/ssh-upload', {
        id, path: (sess ? sess.path : '/root') + '/' + file.name, data: b64,
      });
      toast('上传成功');
      loadSSHFiles(id, sess ? sess.path : '/root');
    } catch (e) { toast('上传失败', 'err'); }
  };
  reader.readAsDataURL(file);
}
