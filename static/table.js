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

// addFilesToBundle fetches metadata for paths and merges new entries into the bundle.
async function addFilesToBundle(paths) {
  showLoadProgress(true, 0, 1);
  if (window.runtime?.EventsOn) {
    window.runtime.EventsOn('meta:progress', (data) => {
      showLoadProgress(true, data.done, data.total);
    });
  }
  try {
    const metas = await window.go.main.App.GetFileMeta(paths);
    metas.forEach(m => {
      if (!bundle.find(b => b.path === m.path)) bundle.push(m);
    });
    renderTable();
  } finally {
    if (window.runtime?.EventsOff) window.runtime.EventsOff('meta:progress');
    showLoadProgress(false, 0, 0);
  }
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
