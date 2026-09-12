import { UploadFile, ListFiles, DownloadFile } from '../wailsjs/go/main/App.js';

const uploadBtn = document.getElementById('upload-btn');
const grid = document.getElementById('file-grid');
const emptyState = document.getElementById('empty-state');
const toast = document.getElementById('toast');

let toastTimer = null;

function showToast(message, isError = false) {
    toast.textContent = message;
    toast.classList.toggle('error', isError);
    toast.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { toast.hidden = true; }, 3500);
}

function formatBytes(bytes) {
    if (bytes < 1024) return `${bytes} B`;
    const units = ['KB', 'MB', 'GB', 'TB'];
    let value = bytes / 1024;
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
        value /= 1024;
        unit++;
    }
    return `${value.toFixed(value < 10 ? 2 : 1)} ${units[unit]}`;
}

function formatDate(iso) {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString();
}

function downloadIcon() {
    return `<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
        <path fill="currentColor" d="M12 3v10.17l3.59-3.58L17 11l-5 5-5-5 1.41-1.41L11.5 13.17V3h.5zM5 19h14v2H5z"/>
    </svg>`;
}

function spinnerIcon() {
    return `<svg class="spinner" viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
        <path fill="currentColor" d="M12 4a8 8 0 018 8h-2a6 6 0 00-6-6V4z"/>
    </svg>`;
}

function renderFileCard(file) {
    const card = document.createElement('article');
    card.className = 'file-card';
    card.dataset.id = String(file.id);

    card.innerHTML = `
        <div class="file-card-header">
            <p class="file-name" title="${escapeHtml(file.name)}">${escapeHtml(file.name)}</p>
            <button class="icon-btn" title="Reconstruir e baixar arquivo" aria-label="Baixar arquivo reconstruído">
                ${downloadIcon()}
            </button>
        </div>
        <div class="file-meta">
            <span><strong>${formatBytes(file.size)}</strong></span>
            <span>${file.chunkCount} blocos</span>
            <span>${formatDate(file.createdAt)}</span>
        </div>
        <div class="dedup-bar">
            <div class="dedup-bar-fill" style="width: ${Math.min(file.savedPercent, 100)}%"></div>
        </div>
        <div class="dedup-label">
            <strong>${file.savedPercent}%</strong> economizado por deduplicação (${formatBytes(file.savedBytes)})
        </div>
    `;

    const downloadBtn = card.querySelector('.icon-btn');
    downloadBtn.addEventListener('click', () => handleDownload(file.id, downloadBtn));

    return card;
}

function escapeHtml(value) {
    const div = document.createElement('div');
    div.textContent = value;
    return div.innerHTML;
}

async function handleDownload(id, button) {
    const original = button.innerHTML;
    button.disabled = true;
    button.innerHTML = spinnerIcon();
    try {
        const savedPath = await DownloadFile(id);
        if (savedPath) {
            showToast(`Arquivo reconstruído e salvo em: ${savedPath}`);
        }
    } catch (err) {
        showToast(`Falha ao reconstruir arquivo: ${err}`, true);
    } finally {
        button.disabled = false;
        button.innerHTML = original;
    }
}

async function refreshFileList() {
    try {
        const files = await ListFiles();
        grid.innerHTML = '';
        const list = files || [];
        emptyState.hidden = list.length > 0;
        for (const file of list) {
            grid.appendChild(renderFileCard(file));
        }
    } catch (err) {
        showToast(`Falha ao carregar arquivos: ${err}`, true);
    }
}

async function handleUpload() {
    uploadBtn.disabled = true;
    const originalLabel = uploadBtn.innerHTML;
    uploadBtn.innerHTML = 'Enviando…';
    try {
        const file = await UploadFile();
        if (file) {
            showToast(`"${file.name}" enviado e deduplicado.`);
            await refreshFileList();
        }
    } catch (err) {
        showToast(`Falha ao enviar arquivo: ${err}`, true);
    } finally {
        uploadBtn.disabled = false;
        uploadBtn.innerHTML = originalLabel;
    }
}

uploadBtn.addEventListener('click', handleUpload);

refreshFileList();
