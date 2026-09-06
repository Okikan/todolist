// 通过 Wails 绑定调用 Go 后端（延迟获取，等待运行时注入完成）
let api = null;
let ctxTarget = null;

// 列表数据缓存（筛选/搜索时重新渲染用）
let lastActive = [];
let lastDone = [];
let searchQ = '';
let filterMode = 'all';
let filterTag = '';
let calTagFilter = '';

// 标签定义 [{name, color}]
let tagDefs = [];
const tagColors = ['#7c6cf0', '#4a90e2', '#3aa76d', '#e8a33d', '#e5534b', '#d357a5', '#5b5eae', '#8a8f3d'];
let selectedTagColor = tagColors[0];
let currentView = 'active';

// 对话框状态：null = 新建模式，数字 = 正在编辑的任务 id
let editingId = null;

// 应用设置
let settings = { darkMode: false, newTaskHotkey: 'Ctrl+N' };
let recordingHotkey = false;

function $(id) { return document.getElementById(id); }

function showError(msg) {
  const el = $('summary');
  if (el) el.textContent = '⚠ ' + msg;
}

window.onerror = function (msg, src, line) {
  showError('JS错误: ' + msg + ' (行' + line + ')');
  return false;
};
window.addEventListener('unhandledrejection', function (e) {
  showError('调用失败: ' + String(e.reason && e.reason.message ? e.reason.message : e.reason));
});

function initApi() {
  if (window.go && window.go.main && window.go.main.App) {
    api = window.go.main.App;
    return true;
  }
  return false;
}

function waitForApi(timeoutMs) {
  return new Promise(function (resolve) {
    const start = Date.now();
    (function poll() {
      if (initApi()) { resolve(true); return; }
      if (Date.now() - start >= timeoutMs) { resolve(false); return; }
      setTimeout(poll, 100);
    })();
  });
}

// ---------- 渲染 ----------
let refreshing = false;

async function refresh() {
  if (refreshing) return;
  refreshing = true;
  try {
    try {
      const todos = await api.GetTodos();
      lastActive = todos.filter(t => !t.done);
      lastDone = todos.filter(t => t.done);
      renderAll();
      $('count-active').textContent = lastActive.length;
      $('count-done').textContent = lastDone.length;
      $('summary').textContent = lastActive.length > 0
        ? '还有 ' + lastActive.length + ' 件事待办'
        : (lastDone.length > 0 ? '全部完成，太棒了！' : '暂无任务，添加一条吧');
    } catch (err) {
      showError('读取失败: ' + String(err && err.message ? err.message : err));
    }
  } finally {
    refreshing = false;
  }
}

function renderAll() {
  renderActive(filterActive(lastActive));
  renderDone(filterDone(lastDone));
  renderCalendar();
}

function formatDue(ts) {
  const d = new Date(ts * 1000);
  const now = new Date();
  const pad = function (n) { return String(n).padStart(2, '0'); };
  const hm = pad(d.getHours()) + ':' + pad(d.getMinutes());
  const md = (d.getMonth() + 1) + '月' + d.getDate() + '日 ' + hm;
  return d.getFullYear() === now.getFullYear() ? md : d.getFullYear() + '年' + md;
}

// 构造一条任务卡片
function buildTodoItem(t, isDone) {
  const li = document.createElement('li');
  li.className = 'todo-item' + (isDone ? ' done' : '') + (!isDone && t.urge ? ' urged' : '');
  li.dataset.id = t.id;

  const cb = document.createElement('button');
  cb.className = 'check';
  cb.textContent = isDone ? '✓' : '';
  cb.title = isDone ? '恢复为未完成' : '标记完成';
  cb.addEventListener('click', function () { toggle(Number(li.dataset.id)); });

  const span = document.createElement('span');
  span.className = 'title';
  span.textContent = t.title;

  const grip = document.createElement('span');
  grip.className = 'grip';
  grip.textContent = '⠿';

  li.append(cb, span, grip);

  // 标签
  if (t.tag) {
    const def = tagDefs.find(function (d) { return d.name === t.tag; });
    const chip = document.createElement('span');
    chip.className = 'tag-chip';
    const dot = document.createElement('span');
    dot.className = 'tag-dot';
    dot.style.background = def ? def.color : '#9b95ad';
    chip.append(dot, document.createTextNode(t.tag));
    li.insertBefore(chip, grip);
  }

  if (t.due_at) {
    const due = document.createElement('span');
    due.className = 'due' + (!isDone && t.due_at * 1000 < Date.now() ? ' overdue' : '');
    due.textContent = '📅 ' + formatDue(t.due_at);
    li.insertBefore(due, grip);
  }

  // 备注：有内容时显示图标，悬停可预览，点击进入编辑
  if (t.note) {
    const noteIcon = document.createElement('button');
    noteIcon.className = 'note-icon';
    noteIcon.textContent = '🗒';
    noteIcon.title = t.note;
    noteIcon.addEventListener('click', function () { openEditDialog(t); });
    li.insertBefore(noteIcon, grip);
  }

  if (!isDone && t.urge) {
    const badge = document.createElement('span');
    badge.className = 'urge-badge';
    badge.textContent = '⚡ 督促';
    li.insertBefore(badge, grip);
  }
  return li;
}

function emptyHint(text) {
  const div = document.createElement('div');
  div.className = 'empty-hint';
  div.textContent = text;
  return div;
}

function renderActive(items) {
  const view = $('active-view');
  view.innerHTML = '';
  if (items.length === 0) {
    const filtered = searchQ !== '' || filterMode !== 'all';
    view.appendChild(emptyHint(filtered ? '没有符合条件的任务' : '暂无任务，点右上角新建吧'));
    return;
  }
  const ul = document.createElement('ul');
  ul.className = 'todo-list';
  for (const t of items) ul.appendChild(buildTodoItem(t, false));
  view.appendChild(ul);
}

function monthKey(ts) {
  const d = new Date(ts * 1000);
  return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0');
}

function monthLabel(key) {
  const parts = key.split('-');
  return parts[0] + '年' + Number(parts[1]) + '月';
}

// 已完成按年月分组：最近完成的月份在最上面，组内按完成时间倒序
function renderDone(items) {
  const wrap = $('done-view');
  wrap.innerHTML = '';
  if (items.length === 0) {
    const filtered = searchQ !== '';
    wrap.appendChild(emptyHint(filtered ? '没有符合条件的任务' : '还没有完成的任务'));
    return;
  }
  const groups = {};
  for (const t of items) {
    const ts = t.completed_at || t.created_at || 0;
    const key = monthKey(ts);
    (groups[key] = groups[key] || []).push({ t: t, ts: ts });
  }
  const keys = Object.keys(groups).sort().reverse();
  for (const key of keys) {
    const group = document.createElement('div');
    group.className = 'month-group';

    const title = document.createElement('div');
    title.className = 'month-title';
    const name = document.createElement('span');
    name.textContent = monthLabel(key);
    const count = document.createElement('span');
    count.className = 'count';
    count.textContent = groups[key].length;
    title.append(name, count);

    const ul = document.createElement('ul');
    ul.className = 'todo-list';
    groups[key]
      .sort(function (a, b) { return b.ts - a.ts || b.t.id - a.t.id; })
      .forEach(function (e) { ul.appendChild(buildTodoItem(e.t, true)); });

    group.append(title, ul);
    wrap.appendChild(group);
  }
}

// ---------- 搜索与筛选 ----------
function matchSearch(t) {
  return !searchQ || t.title.toLowerCase().includes(searchQ);
}

function matchTag(t) {
  return !filterTag || t.tag === filterTag;
}

function filterActive(items) {
  const now = Date.now() / 1000;
  return items.filter(function (t) {
    if (!matchSearch(t) || !matchTag(t)) return false;
    switch (filterMode) {
      case 'week': return !!t.due_at && t.due_at > now && t.due_at <= now + 7 * 86400;
      case 'overdue': return !!t.due_at && t.due_at < now;
      case 'urge': return !!t.urge;
      default: return true;
    }
  });
}

function filterDone(items) {
  return items.filter(function (t) { return matchSearch(t) && matchTag(t); });
}

$('search-box').addEventListener('input', function () {
  searchQ = this.value.trim().toLowerCase();
  renderAll();
});

$('filter-select').addEventListener('change', function () {
  filterMode = this.value;
  renderAll();
});

$('tag-select').addEventListener('change', function () {
  filterTag = this.value;
  renderAll();
});

// ---------- 视图切换 ----------
function switchView(view) {
  currentView = view;
  const isActive = view === 'active';
  const isDone = view === 'done';
  const isCal = view === 'calendar';
  const isSettings = view === 'settings';
  $('tab-active').classList.toggle('active', isActive);
  $('tab-done').classList.toggle('active', isDone);
  $('tab-calendar').classList.toggle('active', isCal);
  $('tab-settings').classList.toggle('active', isSettings);
  $('active-view').classList.toggle('hidden', !isActive);
  $('done-view').classList.toggle('hidden', !isDone);
  $('calendar-view').classList.toggle('hidden', !isCal);
  $('settings-view').classList.toggle('hidden', !isSettings);
  $('filter-bar').classList.toggle('hidden', isCal || isSettings);
  $('view-title').textContent = isActive ? '未完成' : (isDone ? '已完成' : (isCal ? '月历' : '设置'));
  $('filter-select').disabled = !isActive;
  $('tag-select').disabled = !isActive && !isDone;
}

$('tab-active').addEventListener('click', function () { switchView('active'); });
$('tab-done').addEventListener('click', function () { switchView('done'); });
$('tab-calendar').addEventListener('click', function () { switchView('calendar'); });
$('tab-settings').addEventListener('click', function () {
  switchView('settings');
  loadTags();
  loadTrash();
});

// ---------- 月历视图 ----------
let calYear = new Date().getFullYear();
let calMonth = new Date().getMonth() + 1; // 1~12
let calSelected = null; // 'YYYY-MM-DD' 或 null

function dateStr(y, m, d) {
  return y + '-' + String(m).padStart(2, '0') + '-' + String(d).padStart(2, '0');
}

function dueTasksByDate() {
  const map = {};
  for (const t of lastActive) {
    if (!t.due_at) continue;
    if (calTagFilter && t.tag !== calTagFilter) continue;
    const d = new Date(t.due_at * 1000);
    const key = dateStr(d.getFullYear(), d.getMonth() + 1, d.getDate());
    (map[key] = map[key] || []).push(t);
  }
  return map;
}

function renderCalendar() {
  const grid = $('cal-grid');
  grid.innerHTML = '';
  $('cal-title').textContent = calYear + '年' + calMonth + '月';

  for (const w of ['一', '二', '三', '四', '五', '六', '日']) {
    const wd = document.createElement('div');
    wd.className = 'cal-weekday';
    wd.textContent = '周' + w;
    grid.appendChild(wd);
  }

  const now = new Date();
  const todayKey = dateStr(now.getFullYear(), now.getMonth() + 1, now.getDate());
  const byDate = dueTasksByDate();
  const nowSec = Date.now() / 1000;

  const first = new Date(calYear, calMonth - 1, 1);
  const offset = (first.getDay() + 6) % 7; // 周一为第一列
  const daysInMonth = new Date(calYear, calMonth, 0).getDate();
  const prevDays = new Date(calYear, calMonth - 1, 0).getDate();

  const cells = [];
  for (let i = offset - 1; i >= 0; i--) cells.push({ d: prevDays - i, other: true });
  for (let d = 1; d <= daysInMonth; d++) cells.push({ d: d, other: false });
  let nextD = 1;
  while (cells.length % 7 !== 0 || cells.length < 35) cells.push({ d: nextD++, other: true });

  for (const cell of cells) {
    const el = document.createElement('div');
    el.className = 'cal-cell' + (cell.other ? ' other-month' : '');
    if (!cell.other) {
      const key = dateStr(calYear, calMonth, cell.d);
      if (key === todayKey) el.classList.add('today');
      if (key === calSelected) el.classList.add('selected');

      const num = document.createElement('div');
      num.className = 'cal-day-num';
      num.textContent = cell.d;
      el.appendChild(num);

      const list = byDate[key] || [];
      if (list.length > 0) {
        const hasOverdue = list.some(function (t) { return t.due_at < nowSec; });
        const hasUrge = list.some(function (t) { return t.urge && t.due_at >= nowSec; });
        const chip = document.createElement('span');
        chip.className = 'cal-chip' + (hasOverdue ? ' overdue' : (hasUrge ? ' urge' : ''));
        chip.textContent = list.length + ' 项截止';
        el.appendChild(chip);
      }
      el.addEventListener('click', function () {
        calSelected = calSelected === key ? null : key;
        renderCalendar();
      });
    } else {
      const num = document.createElement('div');
      num.className = 'cal-day-num';
      num.textContent = cell.d;
      el.appendChild(num);
    }
    grid.appendChild(el);
  }
  renderCalDayPanel(byDate);
}

function renderCalDayPanel(byDate) {
  const panel = $('cal-day-panel');
  if (!calSelected) {
    panel.classList.add('hidden');
    panel.innerHTML = '';
    return;
  }
  const parts = calSelected.split('-').map(Number);
  const list = byDate[calSelected] || [];
  const weekday = '周' + '日一二三四五六'[new Date(parts[0], parts[1] - 1, parts[2]).getDay()];
  panel.classList.remove('hidden');
  panel.innerHTML = '';

  const title = document.createElement('div');
  title.className = 'cal-day-title';
  title.textContent = (parts[1]) + '月' + parts[2] + '日 ' + weekday + ' · ' + list.length + ' 项截止';
  panel.appendChild(title);

  if (list.length === 0) {
    panel.appendChild(emptyHint('这一天没有截止的任务'));
    return;
  }
  const ul = document.createElement('ul');
  ul.className = 'todo-list';
  for (const t of list) ul.appendChild(buildTodoItem(t, false));
  panel.appendChild(ul);
}

function calShiftMonth(delta) {
  calMonth += delta;
  if (calMonth < 1) { calMonth = 12; calYear--; }
  if (calMonth > 12) { calMonth = 1; calYear++; }
  calSelected = null;
  renderCalendar();
}

$('cal-prev').addEventListener('click', function () { calShiftMonth(-1); });
$('cal-next').addEventListener('click', function () { calShiftMonth(1); });
$('cal-today').addEventListener('click', function () {
  const now = new Date();
  calYear = now.getFullYear();
  calMonth = now.getMonth() + 1;
  calSelected = dateStr(calYear, calMonth, now.getDate());
  renderCalendar();
});

$('cal-tag-select').addEventListener('change', function () {
  calTagFilter = this.value;
  renderCalendar();
});

// ---------- 标签 ----------
async function loadTags() {
  if (!api || !api.GetTags) return;
  try {
    tagDefs = (await api.GetTags()) || [];
  } catch (err) {
    tagDefs = [];
  }
  refreshTagSelects();
  renderAll();
  if (currentView === 'settings') renderTagManager();
}

// 重建对话框 / 筛选栏 / 月历里的标签下拉
function refreshTagSelects() {
  const dialog = $('task-tag');
  dialog.innerHTML = '';
  dialog.appendChild(new Option('无标签', ''));
  for (const d of tagDefs) dialog.appendChild(new Option(d.name, d.name));

  const filter = $('tag-select');
  filter.innerHTML = '';
  filter.appendChild(new Option('全部标签', ''));
  for (const d of tagDefs) filter.appendChild(new Option(d.name, d.name));
  if (filterTag && !tagDefs.some(function (d) { return d.name === filterTag; })) filterTag = '';
  filter.value = filterTag;

  const cal = $('cal-tag-select');
  cal.innerHTML = '';
  cal.appendChild(new Option('全部标签', ''));
  for (const d of tagDefs) cal.appendChild(new Option(d.name, d.name));
  if (calTagFilter && !tagDefs.some(function (d) { return d.name === calTagFilter; })) calTagFilter = '';
  cal.value = calTagFilter;
}

function renderSwatches() {
  const row = $('tag-color-row');
  row.innerHTML = '';
  for (const c of tagColors) {
    const s = document.createElement('button');
    s.type = 'button';
    s.className = 'swatch' + (c === selectedTagColor ? ' selected' : '');
    s.style.background = c;
    s.title = c;
    s.addEventListener('click', function () {
      selectedTagColor = c;
      renderSwatches();
    });
    row.appendChild(s);
  }
}

function renderTagManager() {
  renderSwatches();
  const wrap = $('tag-list');
  wrap.innerHTML = '';
  if (tagDefs.length === 0) {
    wrap.appendChild(emptyHint('还没有标签，在下方添加一个吧'));
    return;
  }
  for (const d of tagDefs) {
    const row = document.createElement('div');
    row.className = 'tag-row';

    const dot = document.createElement('span');
    dot.className = 'tag-dot';
    dot.style.background = d.color;

    const name = document.createElement('span');
    name.className = 'tag-name';
    name.textContent = d.name;

    const count = lastActive.concat(lastDone).filter(function (t) { return t.tag === d.name; }).length;
    const cnt = document.createElement('span');
    cnt.className = 'count';
    cnt.textContent = count;

    const del = document.createElement('button');
    del.type = 'button';
    del.className = 'trash-btn danger';
    del.textContent = '删除';
    del.addEventListener('click', function () {
      if (!del.dataset.armed) {
        del.dataset.armed = '1';
        del.textContent = '确认删除？';
        setTimeout(function () {
          if (del.dataset.armed) {
            delete del.dataset.armed;
            del.textContent = '删除';
          }
        }, 3000);
        return;
      }
      deleteTag(d.name);
    });

    row.append(dot, name, cnt, del);
    wrap.appendChild(row);
  }
}

async function deleteTag(name) {
  try {
    await api.DeleteTag(name);
  } catch (err) {
    showError('删除标签失败');
    return;
  }
  await loadTags();
  await loadTrash();
}

$('add-tag-btn').addEventListener('click', async function () {
  const input = $('new-tag-name');
  const name = input.value.trim();
  if (!name) return;
  if (tagDefs.some(function (d) { return d.name === name; })) {
    input.classList.add('invalid');
    return;
  }
  input.classList.remove('invalid');
  tagDefs.push({ name: name, color: selectedTagColor });
  try {
    await api.SaveTags(tagDefs);
  } catch (err) {
    showError('保存标签失败');
    return;
  }
  input.value = '';
  await loadTags();
});
$('new-tag-name').addEventListener('input', function () {
  this.classList.remove('invalid');
});

// ---------- 回收站 ----------
function twoClickConfirm(btn, action) {
  if (btn.dataset.armed) {
    delete btn.dataset.armed;
    btn.classList.remove('armed');
    btn.textContent = btn.dataset.orig;
    action();
    return;
  }
  btn.dataset.armed = '1';
  btn.dataset.orig = btn.textContent;
  btn.classList.add('armed');
  btn.textContent = '确认？';
  setTimeout(function () {
    if (btn.dataset.armed) {
      delete btn.dataset.armed;
      btn.classList.remove('armed');
      btn.textContent = btn.dataset.orig;
    }
  }, 3000);
}

async function loadTrash() {
  if (!api || !api.GetTrash) return;
  const wrap = $('trash-list');
  wrap.innerHTML = '';
  let items = [];
  try {
    items = (await api.GetTrash()) || [];
  } catch (err) {
    return;
  }
  if (items.length === 0) {
    wrap.appendChild(emptyHint('回收站是空的'));
    return;
  }
  for (const it of items) {
    const row = document.createElement('div');
    row.className = 'trash-row';

    const info = document.createElement('div');
    info.className = 'trash-info';
    const title = document.createElement('div');
    title.className = 'trash-title' + (it.done ? ' done' : '');
    title.textContent = it.title;
    const meta = document.createElement('div');
    meta.className = 'trash-meta';
    meta.textContent = '删除于 ' + formatDue(it.deleted_at) + (it.tag ? ' · ' + it.tag : '');
    info.append(title, meta);

    const actions = document.createElement('div');
    actions.className = 'settings-btns';
    const restore = document.createElement('button');
    restore.type = 'button';
    restore.className = 'settings-btn';
    restore.textContent = '恢复';
    restore.addEventListener('click', async function () {
      try { await api.RestoreTodo(it.id); } catch (err) { showError('恢复失败'); return; }
      await loadTrash();
      await refresh();
    });
    const purge = document.createElement('button');
    purge.type = 'button';
    purge.className = 'settings-btn danger-btn';
    purge.textContent = '彻底删除';
    purge.addEventListener('click', async function () {
      try { await api.PurgeTrash(it.id); } catch (err) { showError('删除失败'); return; }
      await loadTrash();
    });
    actions.append(restore, purge);

    row.append(info, actions);
    wrap.appendChild(row);
  }
}

$('clear-trash-btn').addEventListener('click', function () {
  twoClickConfirm(this, async function () {
    try { await api.ClearTrash(); } catch (err) { showError('清空失败'); return; }
    await loadTrash();
  });
});

// ---------- 设置：数据导出 / 导入 ----------
function flashBtn(btn, text) {
  if (btn.dataset.busy) return;
  btn.dataset.busy = '1';
  const orig = btn.textContent;
  btn.textContent = text;
  setTimeout(function () {
    btn.textContent = orig;
    delete btn.dataset.busy;
  }, 2400);
}

$('export-btn').addEventListener('click', async function () {
  if (!api || !api.ExportTodos) return;
  try {
    const n = await api.ExportTodos();
    if (n > 0) flashBtn(this, '✓ 已导出 ' + n + ' 条');
  } catch (err) {
    showError('导出失败');
  }
});

$('import-btn').addEventListener('click', async function () {
  if (!api || !api.ImportTodos) return;
  try {
    const res = await api.ImportTodos();
    if (res && (res.imported > 0 || res.skipped > 0)) {
      await refresh();
      flashBtn(this, '✓ 导入 ' + res.imported + ' 条' + (res.skipped > 0 ? ' · 跳过 ' + res.skipped : ''));
    }
  } catch (err) {
    showError(String(err && err.message ? err.message : '导入失败'));
  }
});

// ---------- 刷新 ----------
// 手动刷新：重新拉取数据，督促/逾期状态会按当前时间重算（后端排序也一并更新）
$('refresh-btn').addEventListener('click', async function () {
  const btn = this;
  btn.classList.remove('spinning');
  void btn.offsetWidth; // 强制重排以重新触发旋转动画
  btn.classList.add('spinning');
  await refresh();
});

// 自动重检：窗口久开时也能及时发现跨过截止时间/督促期的任务。
// 右键菜单打开时跳过，避免菜单被重渲染吃掉
setInterval(function () {
  if ($('ctx-menu').classList.contains('hidden')) refresh();
}, 60000);

// ---------- 操作 ----------
function findTodo(id) {
  return lastActive.concat(lastDone).find(function (t) { return t.id === id; });
}

async function toggle(id) {
  try { await api.ToggleTodo(id); } catch (err) { showError('操作失败'); }
  await refresh();
}

async function removeTodo(id) {
  try { await api.DeleteTodo(id); } catch (err) { showError('删除失败'); }
  await refresh();
}

// ---------- 右键菜单 ----------
document.addEventListener('contextmenu', function (e) {
  const li = e.target.closest('.todo-item');
  if (!li) return;
  e.preventDefault();
  ctxTarget = li;
  showMenu(e.clientX, e.clientY, li.classList.contains('done'));
});

function showMenu(x, y, isDone) {
  const menu = $('ctx-menu');
  menu.innerHTML = '';
  const id = Number(ctxTarget.dataset.id);
  const items = [
    { label: '编辑任务', fn: function () { const t = findTodo(id); if (t) openEditDialog(t); } }
  ];
  if (isDone) {
    items.push({ label: '恢复为未完成', fn: function () { toggle(id); } });
  } else {
    items.push({ label: '标记完成', fn: function () { toggle(id); } });
  }
  items.push({ label: '删除任务', danger: true, fn: function () { removeTodo(id); } });

  for (const it of items) {
    const b = document.createElement('button');
    b.textContent = it.label;
    if (it.danger) b.className = 'danger';
    b.addEventListener('click', function () { hideMenu(); it.fn(); });
    menu.appendChild(b);
  }
  menu.classList.remove('hidden');
  const rect = menu.getBoundingClientRect();
  menu.style.left = Math.max(8, Math.min(x, window.innerWidth - rect.width - 8)) + 'px';
  menu.style.top = Math.max(8, Math.min(y, window.innerHeight - rect.height - 8)) + 'px';
}

function hideMenu() { $('ctx-menu').classList.add('hidden'); }
document.addEventListener('click', hideMenu);
window.addEventListener('blur', hideMenu);

// ---------- 新建 / 编辑任务对话框 ----------
const overlay = $('modal-overlay');

function resetForm() {
  $('task-title').value = '';
  $('task-due').value = '';
  $('task-urge').value = '3';
  $('task-urge').disabled = true;
  $('task-note').value = '';
  $('task-tag').value = '';
  $('task-title').classList.remove('invalid');
  $('title-error').classList.add('hidden');
}

function openDialog() {
  editingId = null;
  $('dialog-title').textContent = '新建任务';
  $('submit-btn').textContent = '创建任务';
  resetForm();
  overlay.classList.remove('hidden');
  $('task-title').focus();
}

function toLocalInput(ts) {
  const d = new Date(ts * 1000);
  const pad = function (n) { return String(n).padStart(2, '0'); };
  return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) +
    'T' + pad(d.getHours()) + ':' + pad(d.getMinutes());
}

function openEditDialog(t) {
  editingId = t.id;
  $('dialog-title').textContent = '编辑任务';
  $('submit-btn').textContent = '保存修改';
  resetForm();
  $('task-title').value = t.title;
  if (t.tag) {
    // 标签可能已被删除，动态补一个选项以便回显
    if (!tagDefs.some(function (d) { return d.name === t.tag; })) {
      const opt = document.createElement('option');
      opt.value = t.tag;
      opt.textContent = t.tag + '（已删除）';
      $('task-tag').appendChild(opt);
    }
    $('task-tag').value = t.tag;
  }
  if (t.due_at) {
    $('task-due').value = toLocalInput(t.due_at);
    $('task-urge').disabled = false;
    $('task-urge').value = (t.urge_days != null && t.urge_days >= 0) ? t.urge_days : 3;
  }
  $('task-note').value = t.note || '';
  overlay.classList.remove('hidden');
  $('task-title').focus();
}

function closeDialog() { overlay.classList.add('hidden'); }

$('new-task-btn').addEventListener('click', openDialog);
$('cancel-btn').addEventListener('click', closeDialog);
overlay.addEventListener('mousedown', function (e) {
  if (e.target === overlay) closeDialog();
});
document.addEventListener('keydown', function (e) {
  if (e.key === 'Escape' && !overlay.classList.contains('hidden')) closeDialog();
});

$('task-title').addEventListener('input', function () {
  this.classList.remove('invalid');
  $('title-error').classList.add('hidden');
});

// 填了截止时间才允许设置督促时间
$('task-due').addEventListener('input', function () {
  $('task-urge').disabled = !this.value;
});

$('task-title').addEventListener('keydown', function (e) {
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
    e.preventDefault();
    $('task-form').requestSubmit();
  }
});

$('task-form').addEventListener('submit', async function (e) {
  e.preventDefault();
  const titleEl = $('task-title');
  const title = titleEl.value.trim();
  if (!title) {
    titleEl.classList.add('invalid');
    $('title-error').classList.remove('hidden');
    titleEl.focus();
    return;
  }
  const note = $('task-note').value.trim();
  const tag = $('task-tag').value;
  const dueRaw = $('task-due').value;
  let dueAt = null;
  let urgeDays = 3;
  if (dueRaw) {
    const ms = new Date(dueRaw).getTime();
    if (!isNaN(ms)) {
      dueAt = Math.floor(ms / 1000);
      const n = parseInt($('task-urge').value, 10);
      if (!isNaN(n) && n >= 0) urgeDays = n;
    }
  }
  try {
    if (editingId != null) {
      await api.UpdateTodo(editingId, title, dueAt, urgeDays, note, tag);
    } else {
      await api.AddTodo(title, dueAt, urgeDays, note, tag);
    }
  } catch (err) {
    showError(editingId != null ? '保存失败' : '添加失败');
  }
  closeDialog();
  await refresh();
});

// ---------- 设置：加载与应用 ----------
async function loadSettings() {
  if (!api || !api.GetSettings) return;
  try {
    const s = await api.GetSettings();
    if (s) {
      settings.darkMode = !!s.dark_mode;
      if (s.new_task_hotkey) settings.newTaskHotkey = s.new_task_hotkey;
    }
  } catch (err) { /* 读取失败时用默认设置 */ }
  applySettings();
}

function applySettings() {
  document.body.classList.toggle('dark', settings.darkMode);
  $('darkmode-switch').classList.toggle('on', settings.darkMode);
  $('hotkey-btn').textContent = settings.newTaskHotkey.split('+').join(' + ');
}

async function saveSettings() {
  if (!api || !api.SaveSettings) return;
  try {
    await api.SaveSettings(settings.darkMode, settings.newTaskHotkey);
  } catch (err) {
    showError('保存设置失败');
  }
}

$('darkmode-switch').addEventListener('click', function () {
  settings.darkMode = !settings.darkMode;
  applySettings();
  saveSettings();
});

// ---------- 设置：新建任务快捷键 ----------
const hotkeyHint = $('hotkey-hint');

function showHotkeyHint(text, isError) {
  hotkeyHint.textContent = text;
  hotkeyHint.classList.toggle('error', !!isError);
  hotkeyHint.classList.remove('hidden');
}

function hideHotkeyHint() { hotkeyHint.classList.add('hidden'); }

function stopRecording() {
  recordingHotkey = false;
  $('hotkey-btn').classList.remove('recording');
  hideHotkeyHint();
  applySettings();
}

// 从按键事件构造组合键字符串（如 "Ctrl+N"）；不合法返回 null
function comboFromEvent(e) {
  if (['Control', 'Alt', 'Shift', 'Meta'].includes(e.key)) return null;
  let main;
  if (/^[a-z]$/i.test(e.key)) main = e.key.toUpperCase();
  else if (/^[0-9]$/.test(e.key)) main = e.key;
  else if (/^F([1-9]|1[0-2])$/.test(e.key)) main = e.key;
  else if (e.key === 'Enter') main = 'Enter';
  else if (e.key === ' ') main = 'Space';
  else return null;
  if (!e.ctrlKey && !e.altKey && !/^F/.test(main)) return null; // 必须带 Ctrl/Alt，或为 F 键
  const mods = [];
  if (e.ctrlKey) mods.push('Ctrl');
  if (e.altKey) mods.push('Alt');
  if (e.shiftKey) mods.push('Shift');
  mods.push(main);
  return mods.join('+');
}

// 判断按键事件是否命中组合键
function comboMatch(e, combo) {
  if (!combo) return false;
  const parts = combo.split('+');
  const main = parts[parts.length - 1];
  let mainKey = main;
  if (main === 'Space') mainKey = ' ';
  else if (main.length === 1) mainKey = main.toUpperCase();
  const k = e.key.length === 1 ? e.key.toUpperCase() : e.key;
  if (k !== mainKey) return false;
  if (parts.includes('Ctrl') !== e.ctrlKey) return false;
  if (parts.includes('Alt') !== e.altKey) return false;
  if (parts.includes('Shift') !== e.shiftKey) return false;
  return !e.metaKey;
}

$('hotkey-btn').addEventListener('click', function () {
  if (recordingHotkey) return;
  recordingHotkey = true;
  this.classList.add('recording');
  this.textContent = '按下快捷键…';
  showHotkeyHint('按下新的组合键（需包含 Ctrl 或 Alt，或为 F1~F12）；Esc 取消，Backspace 恢复默认 Ctrl + N', false);
});

// 捕获阶段统一处理：录制快捷键 + 应用内新建任务快捷键
document.addEventListener('keydown', function (e) {
  if (recordingHotkey) {
    e.preventDefault();
    e.stopPropagation();
    if (e.key === 'Escape') { stopRecording(); return; }
    if (e.key === 'Backspace') {
      settings.newTaskHotkey = 'Ctrl+N';
      stopRecording();
      saveSettings();
      return;
    }
    const combo = comboFromEvent(e);
    if (!combo) return;
    if (combo === 'Ctrl+Alt+Space') {
      showHotkeyHint('该组合已用于呼出 / 隐藏窗口，请换一个', true);
      return;
    }
    settings.newTaskHotkey = combo;
    stopRecording();
    saveSettings();
    return;
  }
  // 应用内新建任务快捷键（对话框开着时不触发）
  if (overlay.classList.contains('hidden') && comboMatch(e, settings.newTaskHotkey)) {
    e.preventDefault();
    openDialog();
  }
}, true);

// ---------- 开机自启开关 ----------
async function initAutoStartSwitch() {
  const btn = $('autostart-switch');
  if (!api || !api.GetAutoStart) {
    btn.classList.add('hidden');
    return;
  }
  try {
    const on = await api.GetAutoStart();
    btn.classList.toggle('on', !!on);
  } catch (err) {
    btn.classList.add('hidden');
    return;
  }
  btn.addEventListener('click', async function () {
    const target = !btn.classList.contains('on');
    try {
      const on = await api.SetAutoStart(target);
      btn.classList.toggle('on', !!on);
    } catch (err) {
      showError('设置开机自启失败');
    }
  });
}

// ---------- 启动 ----------
(async function boot() {
  if (!(await waitForApi(10000))) {
    showError('后端连接失败');
    return;
  }
  // 托盘"新建任务"菜单 → 打开主窗口并弹出新建对话框
  if (window.runtime && window.runtime.EventsOn) {
    window.runtime.EventsOn('open-new-task', openDialog);
  }
  switchView('active');
  await loadSettings();
  initAutoStartSwitch();
  await loadTags();
  await refresh();
})();
