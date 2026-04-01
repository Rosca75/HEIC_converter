'use strict';

// --- State ---
let bundle     = [];  // array of FileMeta objects currently in the table
let outputPath = '';  // user-selected output directory

// showLoadProgress shows or hides the progress bar with current file counts.
function showLoadProgress(show, done, total) {
  const el  = document.getElementById('loadProgress');
  const bar = document.getElementById('loadProgressBar');
  const txt = document.getElementById('loadProgressText');
  if (!show) { el.classList.add('hidden'); return; }
  el.classList.remove('hidden');
  bar.style.width = (total > 0 ? (done / total) * 100 : 0) + '%';
  txt.textContent = `Loading ${done} / ${total}`;
}

// addFilesToBundle uses streaming events so rows appear as metadata arrives.
async function addFilesToBundle(paths) {
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
    await window.go.main.App.GetFileMetaStreaming(paths);
  } catch (err) {
    if (window.runtime?.EventsOff) {
      window.runtime.EventsOff('meta:start');
      window.runtime.EventsOff('meta:file');
      window.runtime.EventsOff('meta:done');
    }
    showLoadProgress(false, 0, 0);
  }
}

// appendTableRow adds a single row to the file table without a full re-render.
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
    `<td>${item.width}\u00d7${item.height}</td>` +
    `<td>${item.createdAt}</td>` +
    `<td>${item.camera}</td>` +
    `<td><button class="btn-remove" data-idx="${idx}">\u2715</button></td>`;
  tbody.appendChild(tr);

  tr.querySelector('.btn-remove').addEventListener('click', () => {
    bundle.splice(Number(tr.querySelector('.btn-remove').dataset.idx), 1);
    renderTable();
  });
}

// renderTable rebuilds the file bundle table from the current bundle state.
function renderTable() {
  const tbody = document.getElementById('fileTableBody');
  if (!bundle.length) {
    tbody.innerHTML = '<tr id="emptyRow"><td colspan="7" class="empty-msg">No files selected.</td></tr>';
    return;
  }
  tbody.innerHTML = '';
  bundle.forEach((item, idx) => {
    const tr = document.createElement('tr');
    const thumb = item.thumbBase64
      ? `<img src="${item.thumbBase64}" width="48" height="48" class="thumb-img">`
      : '';
    tr.innerHTML =
      `<td>${thumb}</td>` +
      `<td>${item.name}</td>` +
      `<td class="path-cell" title="${item.path}">${item.path}</td>` +
      `<td>${item.width}\u00d7${item.height}</td>` +
      `<td>${item.createdAt}</td>` +
      `<td>${item.camera}</td>` +
      `<td><button class="btn-remove" data-idx="${idx}">\u2715</button></td>`;
    tbody.appendChild(tr);
  });
  document.querySelectorAll('.btn-remove').forEach(btn => {
    btn.addEventListener('click', () => {
      bundle.splice(Number(btn.dataset.idx), 1);
      renderTable();
    });
  });
}
