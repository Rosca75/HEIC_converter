'use strict';

// --- State (module-level globals shared with app.js) ---
let bundle     = [];  // array of FileMeta objects currently in the table
let outputPath = '';  // user-selected output directory

// showProgress toggles a progress bar by element prefix ('load' or 'convert').
function showProgress(prefix, show, done, total) {
  const el  = document.getElementById(prefix + 'Progress');
  const bar = document.getElementById(prefix + 'ProgressBar');
  const txt = document.getElementById(prefix + 'ProgressText');
  if (!el) return;
  if (!show) { el.classList.add('hidden'); return; }
  el.classList.remove('hidden');
  bar.style.width = (total > 0 ? (done / total) * 100 : 0) + '%';
  txt.textContent = prefix === 'load'
    ? `Loading ${done} / ${total}`
    : `Converting ${done} / ${total}`;
}
const showLoadProgress    = (show, d, t) => showProgress('load', show, d, t);
const showConvertProgress = (show, d, t) => showProgress('convert', show, d, t);

// bindConvertProgress wires the Wails event stream for the conversion run.
// Called right before window.go.main.App.ConvertFiles(...) is invoked.
function bindConvertProgress() {
  if (!window.runtime?.EventsOn) return;
  let total = 0;
  window.runtime.EventsOn('convert:start', t => { total = t; showConvertProgress(true, 0, total); });
  window.runtime.EventsOn('convert:file',  m => { showConvertProgress(true, m.done, m.total); });
  window.runtime.EventsOn('convert:done',  () => {
    window.runtime.EventsOff('convert:start');
    window.runtime.EventsOff('convert:file');
    window.runtime.EventsOff('convert:done');
  });
}

// addFilesToBundle calls GetFileMetaStreaming and appends rows as events arrive.
async function addFilesToBundle(paths, recursive) {
  showLoadProgress(true, 0, 1);
  let total = 0;
  let done  = 0;
  if (window.runtime?.EventsOn) {
    window.runtime.EventsOn('meta:start', (t) => { total = t; });
    window.runtime.EventsOn('meta:file', (m) => {
      done++;
      showLoadProgress(true, done, total || 1);
      if (!bundle.find(b => b.path === m.path)) {
        bundle.push(m);
        appendTableRow(m, bundle.length - 1);
      }
    });
    window.runtime.EventsOn('meta:done', () => {
      window.runtime.EventsOff('meta:start');
      window.runtime.EventsOff('meta:file');
      window.runtime.EventsOff('meta:done');
      showLoadProgress(false, 0, 0);
    });
  }
  try {
    await window.go.main.App.GetFileMetaStreaming(paths, !!recursive);
  } catch {
    if (window.runtime?.EventsOff) {
      window.runtime.EventsOff('meta:start');
      window.runtime.EventsOff('meta:file');
      window.runtime.EventsOff('meta:done');
    }
    showLoadProgress(false, 0, 0);
  }
}

// appendTableRow adds one FileMeta row to the table body without a full rerender.
function appendTableRow(item, idx) {
  const tbody = document.getElementById('fileTableBody');
  const empty = document.getElementById('emptyRow');
  if (empty) empty.remove();
  const tr = document.createElement('tr');
  const thumb = item.thumbBase64
    ? `<img src="${item.thumbBase64}" width="48" height="48" class="thumb-img">`
    : '';
  tr.innerHTML =
    `<td>${thumb}</td>` +
    `<td>${item.name}</td>` +
    `<td class="path-cell" title="${item.path}">${item.path}</td>` +
    `<td>${item.width}×${item.height}</td>` +
    `<td>${item.createdAt}</td>` +
    `<td>${item.camera}</td>` +
    `<td><button class="btn-remove" data-idx="${idx}">✕</button></td>`;
  tbody.appendChild(tr);
  tr.querySelector('.btn-remove').addEventListener('click', () => {
    bundle.splice(Number(tr.querySelector('.btn-remove').dataset.idx), 1);
    renderTable();
  });
}

// renderTable rebuilds the entire table body from the current bundle state.
function renderTable() {
  const tbody = document.getElementById('fileTableBody');
  if (!bundle.length) {
    tbody.innerHTML = '<tr id="emptyRow"><td colspan="7" class="empty-msg">No files selected.</td></tr>';
    return;
  }
  tbody.innerHTML = '';
  bundle.forEach((item, idx) => appendTableRow(item, idx));
}
