package httpx

func AnalyticsCockpitBundle() string {
	return `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  window.ClinicModuleBundles["analytics-cockpit"] = {
    render: async function(ctx) {
      const payload = await ctx.api('/ui/data/analytics/snapshot');
      const text = function(en, id) { return ctx.locale === "id" ? id : en; };
      const ensureStyles = function() {
        if (document.getElementById('analytics-cockpit-styles')) return;
        const style = document.createElement('style');
        style.id = 'analytics-cockpit-styles';
        style.textContent = ''
          + '.analytics-cockpit { display: grid; gap: 1.5rem; }'
          + '.analytics-cockpit__hero { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 1rem; padding: 1.5rem; border: 1px solid var(--color-line); border-radius: 1rem; background: linear-gradient(135deg, color-mix(in srgb, var(--color-accent-soft) 70%, var(--color-surface) 30%), var(--color-surface)); box-shadow: var(--shadow-panel); }'
          + '.analytics-cockpit__hero-copy { max-width: 40rem; }'
          + '.analytics-cockpit__eyebrow { display: inline-flex; align-items: center; padding: 0.35rem 0.65rem; border-radius: 999px; background: color-mix(in srgb, var(--color-accent) 10%, var(--color-surface)); color: var(--color-accent-dark); font-size: 0.72rem; font-weight: 700; letter-spacing: 0.14em; text-transform: uppercase; }'
          + '.analytics-cockpit__hero h2 { margin: 0.9rem 0 0.45rem; color: var(--color-body); font-size: clamp(1.8rem, 3vw, 2.6rem); line-height: 1.1; }'
          + '.analytics-cockpit__hero p { margin: 0; color: var(--color-muted); font-size: 0.98rem; line-height: 1.6; }'
          + '.analytics-cockpit__hero-actions { display: flex; flex-wrap: wrap; gap: 0.75rem; align-items: flex-start; }'
          + '.analytics-cockpit__button { appearance: none; border: 1px solid var(--color-line); border-radius: 0.85rem; background: var(--color-surface); color: var(--color-body); padding: 0.8rem 1rem; font: inherit; font-weight: 600; cursor: pointer; transition: transform 0.18s ease, border-color 0.18s ease, background 0.18s ease; }'
          + '.analytics-cockpit__button:hover { transform: translateY(-1px); border-color: var(--color-accent); }'
          + '.analytics-cockpit__button--primary { background: var(--color-accent); border-color: var(--color-accent); color: #fff; }'
          + '.analytics-cockpit__button--ghost { background: color-mix(in srgb, var(--color-surface) 82%, transparent); }'
          + '.analytics-cockpit__metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr)); gap: 1rem; }'
          + '.analytics-cockpit__metric { border: 1px solid var(--color-line); border-radius: 1rem; background: var(--color-surface); box-shadow: var(--shadow-panel); padding: 1.15rem 1.2rem; }'
          + '.analytics-cockpit__metric-label { color: var(--color-muted); font-size: 0.78rem; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; }'
          + '.analytics-cockpit__metric-value { margin-top: 0.55rem; color: var(--color-body); font-size: 2rem; font-weight: 800; line-height: 1; }'
          + '.analytics-cockpit__metric-hint { margin-top: 0.45rem; color: var(--color-muted); font-size: 0.86rem; }'
          + '.analytics-cockpit__grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr)); gap: 1rem; }'
          + '.analytics-cockpit__panel { border: 1px solid var(--color-line); border-radius: 1rem; background: var(--color-surface); box-shadow: var(--shadow-panel); overflow: hidden; }'
          + '.analytics-cockpit__panel-header { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; padding: 1rem 1.1rem; border-bottom: 1px solid var(--color-line); background: color-mix(in srgb, var(--color-accent-soft) 35%, var(--color-surface)); }'
          + '.analytics-cockpit__panel-header h3 { margin: 0; color: var(--color-body); font-size: 1rem; }'
          + '.analytics-cockpit__panel-header span { color: var(--color-muted); font-size: 0.82rem; }'
          + '.analytics-cockpit__panel-body { padding: 1.1rem; }'
          + '.analytics-cockpit__detail-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr)); gap: 0.85rem; }'
          + '.analytics-cockpit__detail-item { border: 1px solid var(--color-line); border-radius: 0.85rem; padding: 0.9rem; background: color-mix(in srgb, var(--color-shell) 45%, var(--color-surface)); }'
          + '.analytics-cockpit__detail-item strong { display: block; margin-top: 0.45rem; color: var(--color-body); font-size: 1.35rem; line-height: 1; }'
          + '.analytics-cockpit__detail-item span { color: var(--color-muted); font-size: 0.8rem; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; }'
          + '.analytics-cockpit__table-wrap { overflow: auto; border: 1px solid var(--color-line); border-radius: 0.9rem; background: var(--color-surface); }'
          + '.analytics-cockpit__table { width: 100%; border-collapse: collapse; min-width: 24rem; }'
          + '.analytics-cockpit__table thead { background: var(--color-accent-soft); }'
          + '.analytics-cockpit__table th { padding: 0.9rem 1rem; text-align: left; color: var(--color-accent-dark); font-size: 0.74rem; font-weight: 800; letter-spacing: 0.14em; text-transform: uppercase; }'
          + '.analytics-cockpit__table td { padding: 0.95rem 1rem; color: var(--color-body); border-top: 1px solid var(--color-line); vertical-align: top; }'
          + '.analytics-cockpit__table td:last-child, .analytics-cockpit__table th:last-child { text-align: right; }'
          + '.analytics-cockpit__row-primary { font-weight: 700; }'
          + '.analytics-cockpit__empty { color: var(--color-muted); text-align: center; }'
          + '.analytics-cockpit__snapshot summary { cursor: pointer; list-style: none; }'
          + '.analytics-cockpit__snapshot summary::-webkit-details-marker { display: none; }'
          + '.analytics-cockpit__snapshot-toggle { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 1rem 1.1rem; }'
          + '.analytics-cockpit__snapshot pre { margin: 0; padding: 0 1.1rem 1.1rem; overflow: auto; color: var(--color-body); font-size: 0.82rem; font-family: var(--font-family-mono, ui-monospace, monospace); }'
          + '.dark .analytics-cockpit__hero { background: linear-gradient(135deg, color-mix(in srgb, var(--color-accent-soft) 55%, var(--color-shell) 45%), var(--color-surface)); }'
          + '.dark .analytics-cockpit__button--ghost { background: color-mix(in srgb, var(--color-surface) 88%, transparent); }'
          + '.dark .analytics-cockpit__detail-item { background: color-mix(in srgb, var(--color-shell) 65%, var(--color-surface)); }'
          + '.dark .analytics-cockpit__table thead { background: color-mix(in srgb, var(--color-ink) 78%, transparent); }'
          + '.dark .analytics-cockpit__table th { color: var(--color-body); }'
          + '@media (max-width: 720px) { .analytics-cockpit__hero { padding: 1.2rem; } .analytics-cockpit__hero-actions { width: 100%; } .analytics-cockpit__button { width: 100%; justify-content: center; } }';
        document.head.appendChild(style);
      };
      const escapeHTML = function(value) {
        return String(value == null ? '' : value).replace(/[&<>"]/g, function(char) {
          return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[char];
        });
      };
      const formatInt = function(value) {
        return new Intl.NumberFormat(ctx.locale === "id" ? "id-ID" : "en-US").format(Number(value || 0));
      };
      const formatPercent = function(value) {
        return (Number(value || 0) * 100).toFixed(1) + '%';
      };
      const totalDocuments = (payload.documents.created || 0) + (payload.documents.draft || 0) + (payload.documents.submitted || 0) + (payload.documents.approved || 0) + (payload.documents.rejected || 0) + (payload.documents.cancelled || 0);
      const renderRows = function(entries, emptyLabel) {
        if (!entries.length) {
          return '<tr><td colspan="3" class="analytics-cockpit__empty">' + escapeHTML(emptyLabel) + '</td></tr>';
        }
        return entries.map(function(entry) {
          return '<tr><td><div class="analytics-cockpit__row-primary">' + escapeHTML(entry.label) + '</div></td><td>' + escapeHTML(entry.primary) + '</td><td>' + escapeHTML(entry.secondary) + '</td></tr>';
        }).join('');
      };
      const typeRows = Object.keys((payload.segments && payload.segments.by_document_type) || {}).sort().map(function(key) {
        const item = payload.segments.by_document_type[key] || {};
        return {
          label: key,
          primary: formatInt((item.submitted || 0) + (item.approved || 0)),
          secondary: formatInt(item.draft || 0)
        };
      });
      const locationRows = Object.keys((payload.segments && payload.segments.by_location) || {}).sort().map(function(key) {
        const item = payload.segments.by_location[key] || {};
        return {
          label: key || text('Unassigned', 'Tanpa Lokasi'),
          primary: formatInt((item.approved || 0) + (item.submitted || 0)),
          secondary: formatInt(item.rejected || 0)
        };
      });
      const metrics = Object.keys(payload.metrics || {}).sort(function(a, b) {
        return (payload.metrics[b] || 0) - (payload.metrics[a] || 0);
      }).slice(0, 8).map(function(key) {
        return {label: key, value: payload.metrics[key]};
      });
      const metricsRows = metrics.length ? metrics.map(function(item) {
        return '<tr><td><div class="analytics-cockpit__row-primary">' + escapeHTML(item.label) + '</div></td><td>' + escapeHTML(formatInt(item.value)) + '</td><td></td></tr>';
      }).join('') : '<tr><td colspan="3" class="analytics-cockpit__empty">' + text('No metrics captured yet.', 'Belum ada metrik yang terekam.') + '</td></tr>';
      ensureStyles();
      ctx.mount.innerHTML = ''
        + '<section class="analytics-cockpit">'
        +   '<section class="analytics-cockpit__hero">'
        +     '<div class="analytics-cockpit__hero-copy">'
        +       '<span class="analytics-cockpit__eyebrow">' + text('Operations Dashboard', 'Dasbor Operasional') + '</span>'
        +       '<h2>' + text('Analytics Cockpit', 'Kokpit Analitik') + '</h2>'
        +       '<p>' + text('Operational analytics overview for documents, workflow, and reliability.', 'Ringkasan analitik untuk dokumen, workflow, dan reliabilitas.') + '</p>'
        +     '</div>'
        +     '<div class="analytics-cockpit__hero-actions">'
        +       (ctx.print && ctx.print.resolved ? '<button type="button" class="analytics-cockpit__button analytics-cockpit__button--ghost" data-print-preview="1">' + text('Preview', 'Pratinjau') + '</button><button type="button" class="analytics-cockpit__button analytics-cockpit__button--ghost" data-print-window="1">' + text('Print', 'Cetak') + '</button><button type="button" class="analytics-cockpit__button analytics-cockpit__button--ghost" data-print-pdf="1">' + text('Download PDF', 'Unduh PDF') + '</button>' : '')
        +       '<button type="button" class="analytics-cockpit__button analytics-cockpit__button--primary" data-nav="/ui/documents">' + text('Open Requests', 'Buka Permintaan') + '</button>'
        +       '<button type="button" class="analytics-cockpit__button analytics-cockpit__button--ghost" data-nav="/ui/monitoring">' + text('Open Monitoring', 'Buka Monitoring') + '</button>'
        +     '</div>'
        +   '</section>'
        +   '<section class="analytics-cockpit__metrics">'
        +     '<article class="analytics-cockpit__metric"><div class="analytics-cockpit__metric-label">' + text('Documents', 'Dokumen') + '</div><div class="analytics-cockpit__metric-value">' + formatInt(totalDocuments) + '</div><div class="analytics-cockpit__metric-hint">' + text('All tracked document states', 'Semua status dokumen yang dipantau') + '</div></article>'
        +     '<article class="analytics-cockpit__metric"><div class="analytics-cockpit__metric-label">' + text('Pending Approvals', 'Persetujuan Tertunda') + '</div><div class="analytics-cockpit__metric-value">' + formatInt(payload.workflow.pending_approvals) + '</div><div class="analytics-cockpit__metric-hint">' + text('Items waiting for sign-off', 'Item yang menunggu persetujuan') + '</div></article>'
        +     '<article class="analytics-cockpit__metric"><div class="analytics-cockpit__metric-label">' + text('Approval Rate', 'Tingkat Persetujuan') + '</div><div class="analytics-cockpit__metric-value">' + formatPercent(payload.workflow.approval_rate) + '</div><div class="analytics-cockpit__metric-hint">' + text('Submitted items that reached approval', 'Proporsi item diajukan yang disetujui') + '</div></article>'
        +     '<article class="analytics-cockpit__metric"><div class="analytics-cockpit__metric-label">' + text('Dead Letter Rate', 'Tingkat Dead Letter') + '</div><div class="analytics-cockpit__metric-value">' + formatPercent(payload.reliability.dead_letter_rate) + '</div><div class="analytics-cockpit__metric-hint">' + text('Delivery failures across dispatch pipeline', 'Kegagalan pengiriman pada pipeline dispatch') + '</div></article>'
        +   '</section>'
        +   '<section class="analytics-cockpit__grid">'
        +     '<section class="analytics-cockpit__panel">'
        +       '<div class="analytics-cockpit__panel-header"><div><h3>' + text('Document Flow', 'Arus Dokumen') + '</h3><span>' + text('Volume by lifecycle state', 'Volume per status siklus hidup') + '</span></div></div>'
        +       '<div class="analytics-cockpit__panel-body"><div class="analytics-cockpit__detail-grid">'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Draft', 'Draf') + '</span><strong>' + formatInt(payload.documents.draft) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Submitted', 'Diajukan') + '</span><strong>' + formatInt(payload.documents.submitted) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Approved', 'Disetujui') + '</span><strong>' + formatInt(payload.documents.approved) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Rejected', 'Ditolak') + '</span><strong>' + formatInt(payload.documents.rejected) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Cancelled', 'Dibatalkan') + '</span><strong>' + formatInt(payload.documents.cancelled) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Created', 'Dibuat') + '</span><strong>' + formatInt(payload.documents.created) + '</strong></article>'
        +       '</section>'
        +       '</div>'
        +     '</section>'
        +     '<section class="analytics-cockpit__panel">'
        +       '<div class="analytics-cockpit__panel-header"><div><h3>' + text('Workflow Health', 'Kesehatan Workflow') + '</h3><span>' + text('Task throughput and rejection profile', 'Throughput tugas dan profil penolakan') + '</span></div></div>'
        +       '<div class="analytics-cockpit__panel-body"><div class="analytics-cockpit__detail-grid">'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Open Tasks', 'Tugas Terbuka') + '</span><strong>' + formatInt(payload.workflow.open_tasks) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Completed Tasks', 'Tugas Selesai') + '</span><strong>' + formatInt(payload.workflow.completed_tasks) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Rejection Rate', 'Tingkat Penolakan') + '</span><strong>' + formatPercent(payload.workflow.rejection_rate) + '</strong></article>'
        +       '</div></div>'
        +     '</section>'
        +   '</section>'
        +   '<section class="analytics-cockpit__grid">'
        +     '<section class="analytics-cockpit__panel">'
        +       '<div class="analytics-cockpit__panel-header"><div><h3>' + text('By Document Type', 'Per Jenis Dokumen') + '</h3><span>' + text('Where operational volume is concentrated', 'Konsentrasi volume operasional') + '</span></div></div>'
        +       '<div class="analytics-cockpit__panel-body"><div class="analytics-cockpit__table-wrap"><table class="analytics-cockpit__table"><thead><tr><th>' + text('Document Type', 'Jenis Dokumen') + '</th><th>' + text('Active Flow', 'Arus Aktif') + '</th><th>' + text('Draft', 'Draf') + '</th></tr></thead><tbody>' + renderRows(typeRows, text('No document activity yet.', 'Belum ada aktivitas dokumen.')) + '</tbody></table></div></div>'
        +       '</section>'
        +     '<section class="analytics-cockpit__panel">'
	      +       '<div class="analytics-cockpit__panel-header"><div><h3>' + text('By Location', 'Per Lokasi') + '</h3><span>' + text('Operational activity split by site', 'Aktivitas per situs') + '</span></div></div>'
        +       '<div class="analytics-cockpit__panel-body"><div class="analytics-cockpit__table-wrap"><table class="analytics-cockpit__table"><thead><tr><th>' + text('Location', 'Lokasi') + '</th><th>' + text('Active Flow', 'Arus Aktif') + '</th><th>' + text('Rejected', 'Ditolak') + '</th></tr></thead><tbody>' + renderRows(locationRows, text('No location activity yet.', 'Belum ada aktivitas lokasi.')) + '</tbody></table></div></div>'
        +       '</section>'
        +   '</section>'
        +   '<section class="analytics-cockpit__grid">'
        +     '<section class="analytics-cockpit__panel">'
        +       '<div class="analytics-cockpit__panel-header"><div><h3>' + text('Reliability', 'Reliabilitas') + '</h3><span>' + text('Dispatch and outbox health', 'Kesehatan dispatch dan outbox') + '</span></div></div>'
        +       '<div class="analytics-cockpit__panel-body"><div class="analytics-cockpit__detail-grid">'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Outbox Pending', 'Outbox Tertunda') + '</span><strong>' + formatInt(payload.reliability.outbox_pending) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Dead Letters', 'Dead Letter') + '</span><strong>' + formatInt(payload.reliability.outbox_dead_letters) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Dispatch Success', 'Dispatch Berhasil') + '</span><strong>' + formatInt(payload.reliability.dispatch_success) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Dispatch Retries', 'Dispatch Retry') + '</span><strong>' + formatInt(payload.reliability.dispatch_retries) + '</strong></article>'
        +       '</div></div>'
        +     '</section>'
        +     '<section class="analytics-cockpit__panel">'
        +       '<div class="analytics-cockpit__panel-header"><div><h3>' + text('Coverage', 'Cakupan') + '</h3><span>' + text('Read model and audit footprint', 'Jejak read model dan audit') + '</span></div></div>'
        +       '<div class="analytics-cockpit__panel-body"><div class="analytics-cockpit__detail-grid">'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Document Summaries', 'Ringkasan Dokumen') + '</span><strong>' + formatInt(payload.coverage.document_summaries) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Projection Coverage', 'Cakupan Proyeksi') + '</span><strong>' + formatPercent(payload.coverage.projection_coverage) + '</strong></article>'
        +         '<article class="analytics-cockpit__detail-item"><span>' + text('Audit Events', 'Event Audit') + '</span><strong>' + formatInt(payload.coverage.audit_events) + '</strong></article>'
        +       '</div></div>'
        +     '</section>'
        +   '</section>'
        +   '<section class="analytics-cockpit__panel">'
        +     '<div class="analytics-cockpit__panel-header"><div><h3>' + text('Top Metrics', 'Metrik Utama') + '</h3><span>' + text('Highest-volume counters captured by telemetry', 'Counter telemetri dengan volume tertinggi') + '</span></div></div>'
        +     '<div class="analytics-cockpit__panel-body"><div class="analytics-cockpit__table-wrap"><table class="analytics-cockpit__table"><thead><tr><th>' + text('Metric', 'Metrik') + '</th><th>' + text('Value', 'Nilai') + '</th><th></th></tr></thead><tbody>' + metricsRows + '</tbody></table></div></div>'
        +   '</section>'
        +   '<details class="analytics-cockpit__panel analytics-cockpit__snapshot"><summary class="analytics-cockpit__snapshot-toggle"><strong>' + text('Raw Snapshot', 'Snapshot Mentah') + '</strong><span>' + text('Inspect raw payload', 'Lihat payload mentah') + '</span></summary><pre></pre></details>'
        + '</section>';
      ctx.mount.querySelectorAll('[data-nav]').forEach(function(node) {
        node.addEventListener('click', function() {
          const destination = node.getAttribute('data-nav');
          if (destination) window.location.assign(destination);
        });
      });
      const previewButton = ctx.mount.querySelector('[data-print-preview]');
      if (previewButton && ctx.print && ctx.print.resolved) {
        previewButton.addEventListener('click', function() {
          ctx.print.preview({sample: true});
        });
      }
      const printButton = ctx.mount.querySelector('[data-print-window]');
      if (printButton && ctx.print && ctx.print.resolved) {
        printButton.addEventListener('click', function() {
          ctx.print.open({sample: true});
        });
      }
      const pdfButton = ctx.mount.querySelector('[data-print-pdf]');
      if (pdfButton && ctx.print && ctx.print.resolved) {
        pdfButton.addEventListener('click', function() {
          ctx.print.downloadPDF({sample: true});
        });
      }
      ctx.mount.querySelector('pre').textContent = JSON.stringify(payload, null, 2);
    }
  };
})();`
}
