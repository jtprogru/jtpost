// jtpost — Server-Sent Events listener.
// Открывает EventSource и при post-событиях триггерит htmx-event
// 'jtpost:posts-changed' на body, чтобы dashboard перезагрузил таблицу.
(function () {
    if (!window.EventSource) return;
    let es;
    function connect() {
        try {
            es = new EventSource('/ui/events');
        } catch (e) { return; }
        const handler = function () {
            document.body.dispatchEvent(new CustomEvent('jtpost:posts-changed', { bubbles: true }));
        };
        es.addEventListener('post.created', handler);
        es.addEventListener('post.updated', handler);
        es.addEventListener('post.deleted', handler);
        es.onerror = function () {
            // Авто-reconnect через 5s если соединение порвалось.
            es.close();
            setTimeout(connect, 5000);
        };
    }
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', connect);
    } else {
        connect();
    }
})();
