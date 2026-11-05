(function () {
  'use strict';

  var token = new URLSearchParams(location.search).get('t') || '';
  var app = document.getElementById('app');

  var st = null;            // /api/state response
  var extras = [];          // names added from the list (besides required)
  var values = {};          // name -> typed value (kept across re-renders)
  var shown = {};           // name -> bool, reveal toggle
  var savedNames = {};      // name -> true once saved in this session
  var errors = {};          // name -> error message from last save
  var warnings = [];        // warning strings
  var banner = '';          // top-level message
  var busy = false;

  function el(tag, props, children) {
    var e = document.createElement(tag);
    if (props) {
      Object.keys(props).forEach(function (k) {
        if (k === 'class') e.className = props[k];
        else if (k === 'text') e.textContent = props[k];
        else if (k.slice(0, 2) === 'on') e.addEventListener(k.slice(2), props[k]);
        else e.setAttribute(k, props[k]);
      });
    }
    (children || []).forEach(function (c) { if (c) e.appendChild(c); });
    return e;
  }

  function pad(n) { return n < 10 ? '0' + n : '' + n; }
  function fmtTime(s) {
    var d = new Date(s);
    if (isNaN(d.getTime())) return null;
    return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) +
      ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes());
  }

  function api(method, path) {
    var body = arguments[2];
    var opts = { method: method, cache: 'no-store', credentials: 'same-origin' };
    if (body !== undefined) {
      opts.headers = { 'Content-Type': 'application/json' };
      opts.body = JSON.stringify(body);
    }
    return fetch(path + '?t=' + encodeURIComponent(token), opts).then(function (r) {
      return r.json().catch(function () { return {}; }).then(function (j) {
        return { status: r.status, body: j };
      });
    });
  }

  function networkDown() {
    banner = 'coffer 命令已结束或已超时，请回到聊天';
    busy = false;
    render();
  }

  function existingMap() {
    var m = {};
    (st.existing || []).forEach(function (e) { m[e.name] = e; });
    return m;
  }

  function requiredNames() { return st.required.map(function (r) { return r.name; }); }

  function cardNames() { return requiredNames().concat(extras); }

  function overwriteHint(name, status, updatedAt) {
    if (status !== 'set') return null;
    var t = updatedAt ? fmtTime(updatedAt) : null;
    return t ? '将覆盖 ' + t + ' 设置的值' : '将覆盖现有值';
  }

  function renderCard(name) {
    var req = null;
    st.required.forEach(function (r) { if (r.name === name) req = r; });
    var ex = existingMap()[name];
    var isExtra = !req;
    var card = el('div', { class: 'card' + (errors[name] ? ' error' : '') });

    var head = el('div', { class: 'card-head' }, [el('span', { class: 'card-name', text: name })]);
    if (isExtra && !savedNames[name]) {
      head.appendChild(el('button', { type: 'button', text: '移除', onclick: function () {
        extras = extras.filter(function (n) { return n !== name; });
        delete values[name]; delete errors[name];
        render();
      } }));
    }
    card.appendChild(head);
    if (req && req.description) card.appendChild(el('div', { class: 'desc', text: req.description }));

    if (savedNames[name]) {
      card.appendChild(el('div', { class: 'ok', text: '✅ 已保存' }));
      return card;
    }

    var input = el('input', {
      type: shown[name] ? 'text' : 'password',
      autocomplete: 'off', autocapitalize: 'off', spellcheck: 'false',
      'aria-label': name + ' 的值',
      oninput: function (ev) { values[name] = ev.target.value; }
    });
    input.value = values[name] || '';
    var toggle = el('button', { type: 'button', text: shown[name] ? '隐藏' : '显示', onclick: function () {
      shown[name] = !shown[name];
      render();
    } });
    card.appendChild(el('div', { class: 'row' }, [input, toggle]));

    var hint;
    if (req) hint = overwriteHint(name, req.exists ? 'set' : '', req.updated_at);
    else if (ex) hint = overwriteHint(name, ex.status, ex.updated_at);
    if (hint) card.appendChild(el('div', { class: 'hint', text: hint }));
    if (errors[name]) card.appendChild(el('div', { class: 'err', text: '保存失败：' + errors[name] }));
    return card;
  }

  function renderList() {
    var tbody = el('tbody');
    (st.existing || []).forEach(function (e) {
      var inCards = cardNames().indexOf(e.name) >= 0;
      var statusText = e.status === 'set' ? '✅ 已设置' : '⚠️ 配置中登记但存储缺值';
      var t = e.updated_at ? fmtTime(e.updated_at) : null;
      var btn = el('button', {
        type: 'button',
        text: e.status === 'set' ? '更新' : '补设',
        onclick: function () { extras.push(e.name); render(); }
      });
      if (inCards) btn.disabled = true;
      tbody.appendChild(el('tr', null, [
        el('td', { class: 'card-name', text: e.name }),
        el('td', { text: statusText }),
        el('td', { class: 'muted', text: t || '—' }),
        el('td', null, [btn])
      ]));
    });
    var thead = el('thead', null, [el('tr', null, [
      el('th', { text: '名字' }), el('th', { text: '状态' }), el('th', { text: '更新时间' }), el('th')
    ])]);
    return el('table', { class: 'list' }, [thead, tbody]);
  }

  function render() {
    if (!st) return;
    // Preserve the focused input's caret roughly by not re-rendering on typing (oninput only stores).
    app.textContent = '';
    app.appendChild(el('h1', null, [
      document.createTextNode('设置密钥 · ' + st.namespace + ' '),
      el('span', { class: 'scope', text: '(' + st.scope + ')' })
    ]));
    if (st.storage_note) app.appendChild(el('div', { class: 'note', text: st.storage_note }));
    if (banner) app.appendChild(el('div', { class: 'banner', role: 'alert', text: banner }));

    app.appendChild(el('h2', { text: '待提交' }));
    var names = cardNames();
    if (names.length === 0) app.appendChild(el('p', { class: 'muted', text: '请从下方列表选择要更新的密钥。' }));
    names.forEach(function (n) { app.appendChild(renderCard(n)); });

    var save = el('button', { type: 'button', class: 'primary', text: '保存', onclick: onSave });
    var cancel = el('button', { type: 'button', text: '取消', onclick: onCancel });
    if (busy) { save.disabled = true; cancel.disabled = true; }
    app.appendChild(el('div', { class: 'actions' }, [save, cancel]));

    if ((st.existing || []).length > 0) {
      app.appendChild(el('h2', { text: '已有密钥' }));
      app.appendChild(renderList());
    }
  }

  function finalPage(text, extraLines) {
    app.textContent = '';
    var box = el('div', { class: 'final' }, [el('div', { text: text })]);
    (extraLines || []).forEach(function (l) { box.appendChild(el('div', { class: 'warn', text: l })); });
    app.appendChild(box);
  }

  function onSave() {
    if (busy) return;
    banner = '';
    var todo = cardNames().filter(function (n) { return !savedNames[n]; });
    var empty = todo.filter(function (n) { return !(values[n] || '').replace(/[\r\n]+$/, ''); });
    if (todo.length === 0) { banner = '没有需要保存的内容'; render(); return; }
    if (empty.length > 0) {
      empty.forEach(function (n) { errors[n] = '请填写值'; });
      banner = '请填写：' + empty.join('、');
      render();
      return;
    }
    busy = true;
    render();
    api('POST', '/api/save', { secrets: todo.map(function (n) { return { name: n, value: values[n] }; }) })
      .then(function (r) {
        busy = false;
        if (r.status !== 200) {
          banner = '保存被拒绝：' + (r.body.error || ('HTTP ' + r.status));
          render();
          return;
        }
        errors = {};
        (r.body.results || []).forEach(function (res) {
          if (res.saved) {
            savedNames[res.name] = true;
            delete values[res.name];
            if (res.warning) warnings.push(res.name + ': ' + res.warning);
          } else {
            errors[res.name] = res.error || '未知错误';
          }
        });
        if (r.body.done) {
          var names = Object.keys(savedNames).sort();
          finalPage('✅ 已保存：' + names.join('、') + '，可以关闭此页', warnings.map(function (w) { return '提示 ' + w; }));
          return;
        }
        banner = '部分密钥保存失败，请修正后重试失败项';
        render();
      })
      .catch(networkDown);
  }

  function onCancel() {
    if (busy) return;
    busy = true;
    api('POST', '/api/cancel', {})
      .then(function (r) {
        if (r.status === 200) { finalPage('已取消，可以关闭此页'); return; }
        busy = false;
        banner = '取消失败：HTTP ' + r.status;
        render();
      })
      .catch(networkDown);
  }

  api('GET', '/api/state')
    .then(function (r) {
      if (r.status !== 200) { banner = '无法加载页面（HTTP ' + r.status + '）'; app.textContent = banner; return; }
      st = r.body;
      // Required names no longer pending were already saved (e.g. page reload).
      var pend = {};
      (st.pending || []).forEach(function (n) { pend[n] = true; });
      requiredNames().forEach(function (n) { if (!pend[n]) savedNames[n] = true; });
      render();
    })
    .catch(function () { app.textContent = 'coffer 命令已结束或已超时，请回到聊天'; });
})();
