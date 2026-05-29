// Auto-refresh do grid de status: busca o fragmento server-rendered e troca o nó.
// Reaproveita o basic auth do browser (mesma origem) e o Cache-Control: no-store
// já definido pelo servidor. Sem dependências externas.
(function () {
  function refreshStatus() {
    if (!document.getElementById("status-grid")) return; // só existe no dashboard e em /apps
    fetch("/fragments/status", { credentials: "same-origin" })
      .then(function (r) { return r.ok ? r.text() : null; })
      .then(function (html) {
        var el = document.getElementById("status-grid");
        if (el && html) el.outerHTML = html; // o fragmento mantém id="status-grid"
      })
      .catch(function () { /* falha transitória: tenta de novo no próximo tick */ });
  }

  if (document.getElementById("status-grid")) {
    setInterval(refreshStatus, 10000);
  }
})();
