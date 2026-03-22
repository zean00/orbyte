package httpx

func AnalyticsCockpitBundle() string {
	return `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  window.ClinicModuleBundles["analytics-cockpit"] = {
    render: async function(ctx) {
      const payload = await ctx.api('/ui/data/analytics/snapshot');
      const text = function(en, id) { return ctx.locale === "id" ? id : en; };
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
          return '<tr><td colspan="3" class="status">' + escapeHTML(emptyLabel) + '</td></tr>';
        }
        return entries.map(function(entry) {
          return '<tr><td><div class="row-primary">' + escapeHTML(entry.label) + '</div></td><td>' + escapeHTML(entry.primary) + '</td><td>' + escapeHTML(entry.secondary) + '</td></tr>';
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
        return '<tr><td><div class="row-primary">' + escapeHTML(item.label) + '</div></td><td>' + escapeHTML(formatInt(item.value)) + '</td><td></td></tr>';
      }).join('') : '<tr><td colspan="3" class="status">' + text('No metrics captured yet.', 'Belum ada metrik yang terekam.') + '</td></tr>';
      ctx.mount.innerHTML = ''
        + '<section class="page-panel">'
        +   '<div class="page-header">'
        +     '<div>'
        +       '<h3>' + text('Analytics Cockpit', 'Kokpit Analitik') + '</h3>'
        +       '<p class="status">' + text('Operational analytics overview for documents, workflow, and reliability.', 'Ringkasan analitik operasional untuk dokumen, workflow, dan reliabilitas.') + '</p>'
        +     '</div>'
        +     '<div class="actions">'
        +       (ctx.print && ctx.print.resolved ? '<button type="button" class="secondary" data-print-preview="1">' + text('Preview', 'Pratinjau') + '</button><button type="button" class="secondary" data-print-window="1">' + text('Print', 'Cetak') + '</button><button type="button" class="secondary" data-print-pdf="1">' + text('Download PDF', 'Unduh PDF') + '</button>' : '')
        +       '<button type="button" class="secondary" data-nav="#/documents">' + text('Open Requests', 'Buka Permintaan') + '</button>'
        +       '<button type="button" class="secondary" data-nav="#/monitoring">' + text('Open Monitoring', 'Buka Monitoring') + '</button>'
        +     '</div>'
        +   '</div>'
        +   '<div class="page-body">'
        +     '<div class="metric-grid">'
        +       '<article class="metric-card"><span class="meta">' + text('Documents', 'Dokumen') + '</span><strong>' + formatInt(totalDocuments) + '</strong></article>'
        +       '<article class="metric-card"><span class="meta">' + text('Pending Approvals', 'Persetujuan Tertunda') + '</span><strong>' + formatInt(payload.workflow.pending_approvals) + '</strong></article>'
        +       '<article class="metric-card"><span class="meta">' + text('Approval Rate', 'Tingkat Persetujuan') + '</span><strong>' + formatPercent(payload.workflow.approval_rate) + '</strong></article>'
        +       '<article class="metric-card"><span class="meta">' + text('Dead Letter Rate', 'Tingkat Dead Letter') + '</span><strong>' + formatPercent(payload.reliability.dead_letter_rate) + '</strong></article>'
        +     '</div>'
        +     '<div class="admin-shell-grid">'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('Document Flow', 'Arus Dokumen') + '</h3></div>'
        +         '<div class="section-body"><div class="detail-grid">'
        +           '<article class="detail-item"><span class="meta">' + text('Draft', 'Draf') + '</span><strong>' + formatInt(payload.documents.draft) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Submitted', 'Diajukan') + '</span><strong>' + formatInt(payload.documents.submitted) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Approved', 'Disetujui') + '</span><strong>' + formatInt(payload.documents.approved) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Rejected', 'Ditolak') + '</span><strong>' + formatInt(payload.documents.rejected) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Cancelled', 'Dibatalkan') + '</span><strong>' + formatInt(payload.documents.cancelled) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Created', 'Dibuat') + '</span><strong>' + formatInt(payload.documents.created) + '</strong></article>'
        +         '</div></div>'
        +       '</section>'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('Workflow Health', 'Kesehatan Workflow') + '</h3></div>'
        +         '<div class="section-body"><div class="detail-grid">'
        +           '<article class="detail-item"><span class="meta">' + text('Open Tasks', 'Tugas Terbuka') + '</span><strong>' + formatInt(payload.workflow.open_tasks) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Completed Tasks', 'Tugas Selesai') + '</span><strong>' + formatInt(payload.workflow.completed_tasks) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Rejection Rate', 'Tingkat Penolakan') + '</span><strong>' + formatPercent(payload.workflow.rejection_rate) + '</strong></article>'
        +         '</div></div>'
        +       '</section>'
        +     '</div>'
        +     '<div class="admin-shell-grid">'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('By Document Type', 'Per Jenis Dokumen') + '</h3></div>'
        +         '<div class="section-body"><div class="table-shell"><table class="data-table"><thead><tr><th>' + text('Document Type', 'Jenis Dokumen') + '</th><th>' + text('Active Flow', 'Arus Aktif') + '</th><th>' + text('Draft', 'Draf') + '</th></tr></thead><tbody>' + renderRows(typeRows, text('No document activity yet.', 'Belum ada aktivitas dokumen.')) + '</tbody></table></div></div>'
        +       '</section>'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('By Location', 'Per Lokasi') + '</h3></div>'
        +         '<div class="section-body"><div class="table-shell"><table class="data-table"><thead><tr><th>' + text('Location', 'Lokasi') + '</th><th>' + text('Active Flow', 'Arus Aktif') + '</th><th>' + text('Rejected', 'Ditolak') + '</th></tr></thead><tbody>' + renderRows(locationRows, text('No location activity yet.', 'Belum ada aktivitas lokasi.')) + '</tbody></table></div></div>'
        +       '</section>'
        +     '</div>'
        +     '<div class="admin-shell-grid">'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('Reliability', 'Reliabilitas') + '</h3></div>'
        +         '<div class="section-body"><div class="detail-grid">'
        +           '<article class="detail-item"><span class="meta">' + text('Outbox Pending', 'Outbox Tertunda') + '</span><strong>' + formatInt(payload.reliability.outbox_pending) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Dead Letters', 'Dead Letter') + '</span><strong>' + formatInt(payload.reliability.outbox_dead_letters) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Dispatch Success', 'Dispatch Berhasil') + '</span><strong>' + formatInt(payload.reliability.dispatch_success) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Dispatch Retries', 'Dispatch Retry') + '</span><strong>' + formatInt(payload.reliability.dispatch_retries) + '</strong></article>'
        +         '</div></div>'
        +       '</section>'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('Coverage', 'Cakupan') + '</h3></div>'
        +         '<div class="section-body"><div class="detail-grid">'
        +           '<article class="detail-item"><span class="meta">' + text('Document Summaries', 'Ringkasan Dokumen') + '</span><strong>' + formatInt(payload.coverage.document_summaries) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Projection Coverage', 'Cakupan Proyeksi') + '</span><strong>' + formatPercent(payload.coverage.projection_coverage) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Audit Events', 'Event Audit') + '</span><strong>' + formatInt(payload.coverage.audit_events) + '</strong></article>'
        +         '</div></div>'
        +       '</section>'
        +     '</div>'
        +     '<section class="stack-card">'
        +       '<div class="section-head"><h3>' + text('Top Metrics', 'Metrik Utama') + '</h3></div>'
        +       '<div class="section-body"><div class="table-shell"><table class="data-table"><thead><tr><th>' + text('Metric', 'Metrik') + '</th><th>' + text('Value', 'Nilai') + '</th><th></th></tr></thead><tbody>' + metricsRows + '</tbody></table></div></div>'
        +     '</section>'
        +     '<details class="stack-card"><summary class="row-primary">' + text('Raw Snapshot', 'Snapshot Mentah') + '</summary><div class="section-body"><pre></pre></div></details>'
        +   '</div>'
        + '</section>';
      ctx.mount.querySelectorAll('[data-nav]').forEach(function(node) {
        node.addEventListener('click', function() {
          window.location.hash = node.getAttribute('data-nav');
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
