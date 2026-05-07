// jtpost — drag-drop / file-picker uploader для markdown editor.
// Использует POST /ui/upload (multipart). Вставляет markdown в #md-textarea
// в позиции курсора и триггерит htmx для обновления preview.
async function jtpostUploadFiles(files) {
    if (!files || !files.length) return;
    const ta = document.getElementById('md-textarea');
    if (!ta) return;
    for (const f of files) {
        try {
            const fd = new FormData();
            fd.append('file', f);
            const res = await fetch('/ui/upload', { method: 'POST', body: fd, credentials: 'same-origin' });
            if (!res.ok) {
                const txt = await res.text();
                alert('Ошибка загрузки: ' + (txt || res.status));
                continue;
            }
            const data = await res.json();
            jtpostInsertAtCursor(ta, '\n' + data.markdown + '\n');
        } catch (e) {
            alert('Ошибка сети: ' + e);
        }
    }
}

function jtpostInsertAtCursor(ta, text) {
    const start = ta.selectionStart ?? ta.value.length;
    const end = ta.selectionEnd ?? ta.value.length;
    ta.value = ta.value.slice(0, start) + text + ta.value.slice(end);
    const pos = start + text.length;
    ta.selectionStart = ta.selectionEnd = pos;
    ta.focus();
    // Триггерим htmx-input для live-preview.
    ta.dispatchEvent(new Event('input', { bubbles: true }));
}
