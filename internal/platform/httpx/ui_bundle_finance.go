package httpx

func FinanceReportsBundle() string {
	return `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  window.ClinicModuleBundles['finance-reports'] = {
    render: async function(ctx) {
      const text = function(en, id) { return ctx.locale === 'id' ? id : en; };
      const path = window.location.pathname || '';
      const reportKey = path.includes('period-close') ? 'period-close'
        : path.includes('inventory-valuation-as-of') ? 'inventory-valuation-as-of'
        : path.includes('inventory-valuation') ? 'inventory-valuation'
        : path.includes('inventory-gl-reconciliation') ? 'inventory-gl-reconciliation'
        : path.includes('inventory-adjustment-review') ? 'inventory-adjustment-review'
        : path.includes('ar-statements') ? 'ar-statements'
        : path.includes('ap-statements') ? 'ap-statements'
        : path.includes('collections') ? 'collections'
        : path.includes('settlement-exceptions') ? 'settlement-exceptions'
        : path.includes('trial-balance') ? 'trial-balance'
        : path.includes('profit-and-loss') ? 'profit-and-loss'
        : path.includes('balance-sheet') ? 'balance-sheet'
        : path.includes('tax-summary') ? 'tax-summary'
        : path.includes('ar-aging') ? 'ar-aging'
        : path.includes('ap-aging') ? 'ap-aging'
        : path.includes('ar-reconciliation') ? 'ar-reconciliation'
        : path.includes('ap-reconciliation') ? 'ap-reconciliation'
        : 'journal-ledger';
      const title = reportKey === 'period-close' ? text('Period Close', 'Tutup Periode')
        : reportKey === 'inventory-valuation-as-of' ? text('Inventory Valuation As Of', 'Penilaian Inventori Per Tanggal')
        : reportKey === 'inventory-valuation' ? text('Inventory Valuation', 'Penilaian Inventori')
        : reportKey === 'inventory-gl-reconciliation' ? text('Inventory GL Reconciliation', 'Rekonsiliasi GL Inventori')
        : reportKey === 'inventory-adjustment-review' ? text('Inventory Adjustment Review', 'Tinjauan Penyesuaian Inventori')
        : reportKey === 'ar-statements' ? text('AR Statements', 'Statement Piutang')
        : reportKey === 'ap-statements' ? text('AP Statements', 'Statement Utang')
        : reportKey === 'collections' ? text('Collections', 'Penagihan')
        : reportKey === 'settlement-exceptions' ? text('Settlement Exceptions', 'Pengecualian Settlement')
        : reportKey === 'trial-balance' ? text('Trial Balance', 'Neraca Saldo')
        : reportKey === 'profit-and-loss' ? text('Profit and Loss', 'Laba Rugi')
        : reportKey === 'balance-sheet' ? text('Balance Sheet', 'Neraca')
        : reportKey === 'tax-summary' ? text('Tax Summary', 'Ringkasan Pajak')
        : reportKey === 'ar-aging' ? text('AR Aging', 'Umur Piutang')
        : reportKey === 'ap-aging' ? text('AP Aging', 'Umur Utang')
        : reportKey === 'ar-reconciliation' ? text('AR Reconciliation', 'Rekonsiliasi Piutang')
        : reportKey === 'ap-reconciliation' ? text('AP Reconciliation', 'Rekonsiliasi Utang')
        : text('Journal Ledger', 'Buku Jurnal');
      const mount = ctx.mount;
      const params = new URLSearchParams(window.location.search);
      const filters = {
        from_date: params.get('from_date') || '',
        to_date: params.get('to_date') || '',
        as_of_date: params.get('as_of_date') || '',
        warehouse_code: params.get('warehouse_code') || '',
        party_id: params.get('party_id') || '',
        vendor_id: params.get('vendor_id') || '',
        account_code: params.get('account_code') || '',
        aging_bucket: params.get('aging_bucket') || '',
        period_id: params.get('period_id') || '',
        kind: params.get('kind') || '',
        status: params.get('status') || ''
      };
      const usesAsOf = reportKey === 'balance-sheet' || reportKey === 'inventory-valuation-as-of' || reportKey === 'inventory-gl-reconciliation' || reportKey === 'ar-aging' || reportKey === 'ap-aging' || reportKey === 'ar-reconciliation' || reportKey === 'ap-reconciliation' || reportKey === 'ar-statements' || reportKey === 'ap-statements' || reportKey === 'settlement-exceptions';
      function escapeHTML(value) {
        return String(value == null ? '' : value).replace(/[&<>"]/g, function(char) {
          return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[char];
        });
      }
      function money(value) {
        return new Intl.NumberFormat(ctx.locale === 'id' ? 'id-ID' : 'en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(Number(value || 0));
      }
      function csrfToken() {
        const hit = document.cookie.split('; ').find(function(item) { return item.indexOf('orbyte_csrf=') === 0; });
        return hit ? decodeURIComponent(hit.split('=').slice(1).join('=')) : '';
      }
      async function apiJSON(url, options) {
        const response = await fetch(url, Object.assign({ credentials: 'include' }, options || {}));
        if (!response.ok) {
          let message = response.status + ' ' + response.statusText;
          try {
            const payload = await response.json();
            message = payload && payload.error && payload.error.message ? payload.error.message : message;
          } catch (_) {}
          throw new Error(message);
        }
        return response.json();
      }
      async function postJSON(url, body) {
        return apiJSON(url, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': csrfToken()
          },
          body: JSON.stringify(body || {})
        });
      }
      function ensureStyles() {
        if (document.getElementById('finance-reports-styles')) return;
        const style = document.createElement('style');
        style.id = 'finance-reports-styles';
        style.textContent = ''
          + '.finance-report{display:grid;gap:1rem;}'
          + '.finance-report__hero{display:flex;justify-content:space-between;gap:1rem;flex-wrap:wrap;padding:1rem 1.2rem;border:1px solid var(--color-line);border-radius:1rem;background:linear-gradient(135deg,color-mix(in srgb,var(--color-accent-soft) 70%,var(--color-surface) 30%),var(--color-surface));box-shadow:var(--shadow-panel);}'
          + '.finance-report__hero h2{margin:0;color:var(--color-body);font-size:1.5rem;}'
          + '.finance-report__hero p{margin:0.4rem 0 0;color:var(--color-muted);}'
          + '.finance-report__panel{border:1px solid var(--color-line);border-radius:1rem;background:var(--color-surface);box-shadow:var(--shadow-panel);padding:1rem;}'
          + '.finance-report__filters{display:flex;gap:0.75rem;flex-wrap:wrap;align-items:end;}'
          + '.finance-report__field{display:flex;flex-direction:column;gap:0.35rem;min-width:12rem;}'
          + '.finance-report__field span{font-size:0.74rem;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:var(--color-muted);}'
          + '.finance-report__field input,.finance-report__field select{height:2.5rem;border:1px solid var(--color-line);border-radius:0.75rem;background:var(--color-surface);color:var(--color-body);padding:0 0.8rem;font:inherit;}'
          + '.finance-report__button{appearance:none;border:1px solid var(--color-line);border-radius:0.8rem;background:var(--color-surface);color:var(--color-body);padding:0.8rem 1rem;font:inherit;font-weight:700;cursor:pointer;}'
          + '.finance-report__button--primary{background:var(--color-accent);border-color:var(--color-accent);color:#fff;}'
          + '.finance-report__button--danger{border-color:#c66;color:#8b1f1f;background:#fff6f6;}'
          + '.finance-report__nav{display:flex;gap:0.5rem;flex-wrap:wrap;}'
          + '.finance-report__nav a{display:inline-flex;padding:0.55rem 0.8rem;border-radius:999px;border:1px solid var(--color-line);color:var(--color-body);text-decoration:none;font-weight:700;}'
          + '.finance-report__nav a.is-active{background:var(--color-accent);border-color:var(--color-accent);color:#fff;}'
          + '.finance-report__table-wrap{overflow:auto;border:1px solid var(--color-line);border-radius:0.9rem;}'
          + '.finance-report__table{width:100%;border-collapse:collapse;min-width:48rem;}'
          + '.finance-report__table th,.finance-report__table td{padding:0.8rem 0.7rem;border-top:1px solid var(--color-line);text-align:left;vertical-align:top;}'
          + '.finance-report__table th{font-size:0.74rem;font-weight:800;letter-spacing:0.08em;text-transform:uppercase;color:var(--color-muted);}'
          + '.finance-report__cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(13rem,1fr));gap:0.75rem;}'
          + '.finance-report__card{border:1px solid var(--color-line);border-radius:0.9rem;padding:0.9rem;background:color-mix(in srgb,var(--color-shell) 35%,var(--color-surface));}'
          + '.finance-report__card span{display:block;color:var(--color-muted);font-size:0.74rem;font-weight:800;letter-spacing:0.08em;text-transform:uppercase;}'
          + '.finance-report__card strong{display:block;margin-top:0.45rem;color:var(--color-body);font-size:1.3rem;}'
          + '.finance-report__blockers{margin:0;padding-left:1.2rem;color:#8b1f1f;}'
          + '.finance-report__pill{display:inline-flex;padding:0.25rem 0.6rem;border-radius:999px;border:1px solid var(--color-line);font-size:0.75rem;font-weight:700;}'
          + '.finance-report__pill.is-ready{background:#eefaf1;color:#176936;border-color:#9ad4a6;}'
          + '.finance-report__pill.is-blocked{background:#fff3f2;color:#8b1f1f;border-color:#e6b2ad;}'
          + '.finance-report__actions{display:flex;gap:0.5rem;flex-wrap:wrap;}'
          + '@media (max-width: 900px){.finance-report__table{min-width:36rem;}}';
        document.head.appendChild(style);
      }
      async function loadReport() {
        if (reportKey === 'period-close') {
          const periodsPayload = await apiJSON('/models/accounting_period?page=1&page_size=100');
          const periods = periodsPayload.items || [];
          const chosen = filters.period_id || (periods[0] && periods[0].id) || '';
          let pack = null;
          if (chosen) {
            pack = await apiJSON('/ui/data/finance/periods/' + encodeURIComponent(chosen) + '/close-pack');
          }
          return { periods: periods, pack: pack };
        }
        if (reportKey === 'collections') {
          const query = new URLSearchParams();
          if (filters.kind) query.set('kind', filters.kind);
          if (filters.status) query.set('status', filters.status);
          return apiJSON('/ui/data/finance/collections' + (query.toString() ? '?' + query.toString() : ''));
        }
        const query = new URLSearchParams();
        if (usesAsOf) {
          if (filters.as_of_date) query.set('as_of_date', filters.as_of_date);
        } else {
          if (filters.from_date) query.set('from_date', filters.from_date);
          if (filters.to_date) query.set('to_date', filters.to_date);
        }
        if (filters.party_id && (reportKey === 'ar-aging' || reportKey === 'ar-reconciliation')) query.set('party_id', filters.party_id);
        if (filters.vendor_id && (reportKey === 'ap-aging' || reportKey === 'ap-reconciliation')) query.set('vendor_id', filters.vendor_id);
        if (filters.party_id && reportKey === 'ar-statements') query.set('party_id', filters.party_id);
        if (filters.vendor_id && reportKey === 'ap-statements') query.set('vendor_id', filters.vendor_id);
        if (filters.account_code && (reportKey === 'ar-reconciliation' || reportKey === 'ap-reconciliation' || reportKey === 'inventory-gl-reconciliation')) query.set('account_code', filters.account_code);
        if (filters.aging_bucket && (reportKey === 'ar-aging' || reportKey === 'ap-aging')) query.set('aging_bucket', filters.aging_bucket);
        if (filters.kind && reportKey === 'settlement-exceptions') query.set('kind', filters.kind);
        if (filters.warehouse_code && (reportKey === 'inventory-valuation' || reportKey === 'inventory-valuation-as-of')) query.set('warehouse_code', filters.warehouse_code);
        if (filters.status && reportKey === 'inventory-adjustment-review') query.set('status', filters.status);
        return apiJSON('/ui/data/finance/' + reportKey + (query.toString() ? '?' + query.toString() : ''));
      }
      function renderRows(payload) {
        if (reportKey === 'period-close') {
          const pack = payload.pack;
          if (!pack) {
            return '<section class="finance-report__panel"><p>' + escapeHTML(text('No accounting period selected.', 'Belum ada periode akuntansi dipilih.')) + '</p></section>';
          }
          const blockers = (pack.blockers || []).map(function(item) { return '<li>' + escapeHTML(item) + '</li>'; }).join('');
          const taskRows = (pack.tasks || []).map(function(task) {
            const actionCell = task.task_type === 'checklist' && task.status === 'pending'
              ? '<div class="finance-report__actions"><button class="finance-report__button" data-task-complete="' + escapeHTML(task.id) + '">' + escapeHTML(text('Complete', 'Selesai')) + '</button><button class="finance-report__button" data-task-waive="' + escapeHTML(task.id) + '">' + escapeHTML(text('Waive', 'Abaikan')) + '</button></div>'
              : '';
            return '<tr><td>' + escapeHTML(task.task_code) + '</td><td>' + escapeHTML(task.label) + '</td><td>' + escapeHTML(task.task_type) + '</td><td>' + escapeHTML(task.status) + '</td><td>' + (task.required ? escapeHTML(text('Yes', 'Ya')) : escapeHTML(text('No', 'Tidak'))) + '</td><td>' + actionCell + '</td></tr>';
          }).join('');
          const runRows = (pack.journal_runs || []).map(function(run) {
            const reverseButton = run.journal_kind === 'accrual' && run.posting_id && run.posting_status === 'posted' && run.reversal_status !== 'reversed'
              ? '<button class="finance-report__button" data-posting-reverse="' + escapeHTML(run.posting_id) + '">' + escapeHTML(text('Reverse', 'Reverse')) + '</button>'
              : '';
            const postingLink = run.posting_id ? '<a href="/ui/commercial/ledger/detail?id=' + encodeURIComponent(run.posting_id) + '">' + escapeHTML(run.posting_number || run.posting_id) + '</a>' : '';
            return '<tr><td>' + escapeHTML(run.template_code) + '</td><td>' + escapeHTML(run.template_name) + '</td><td>' + escapeHTML(run.journal_kind) + '</td><td>' + escapeHTML(run.posting_date) + '</td><td>' + escapeHTML(run.status) + '</td><td>' + postingLink + '</td><td>' + reverseButton + '</td></tr>';
          }).join('');
          return ''
            + '<section class="finance-report__panel"><div class="finance-report__cards"><article class="finance-report__card"><span>' + escapeHTML(text('Period', 'Periode')) + '</span><strong>' + escapeHTML(pack.period_key) + '</strong></article><article class="finance-report__card"><span>' + escapeHTML(text('Status', 'Status')) + '</span><strong>' + escapeHTML(pack.status) + '</strong></article><article class="finance-report__card"><span>' + escapeHTML(text('Readiness', 'Kesiapan')) + '</span><strong><span class="finance-report__pill ' + (pack.ready ? 'is-ready' : 'is-blocked') + '">' + escapeHTML(pack.ready ? text('Ready', 'Siap') : text('Blocked', 'Terblokir')) + '</span></strong></article></div></section>'
            + '<section class="finance-report__panel"><div class="finance-report__actions"><button class="finance-report__button finance-report__button--primary" data-generate-journals="' + escapeHTML(pack.period_id) + '">' + escapeHTML(text('Generate Journals', 'Generate Jurnal')) + '</button><button class="finance-report__button finance-report__button--primary" data-close-period="' + escapeHTML(pack.period_id) + '">' + escapeHTML(text('Close Period', 'Tutup Periode')) + '</button><button class="finance-report__button" data-reopen-period="' + escapeHTML(pack.period_id) + '">' + escapeHTML(text('Reopen Period', 'Buka Ulang')) + '</button></div></section>'
            + (blockers ? '<section class="finance-report__panel"><h3>' + escapeHTML(text('Close Blockers', 'Pemblokir Tutup')) + '</h3><ul class="finance-report__blockers">' + blockers + '</ul></section>' : '')
            + '<section class="finance-report__panel"><h3>' + escapeHTML(text('Checklist Tasks', 'Tugas Checklist')) + '</h3><div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Task', 'Tugas') + '</th><th>' + text('Label', 'Label') + '</th><th>' + text('Type', 'Tipe') + '</th><th>' + text('Status', 'Status') + '</th><th>' + text('Required', 'Wajib') + '</th><th>' + text('Action', 'Aksi') + '</th></tr></thead><tbody>' + taskRows + '</tbody></table></div></section>'
            + '<section class="finance-report__panel"><h3>' + escapeHTML(text('Journal Runs', 'Run Jurnal')) + '</h3><div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Template', 'Template') + '</th><th>' + text('Name', 'Nama') + '</th><th>' + text('Kind', 'Jenis') + '</th><th>' + text('Posting Date', 'Tanggal Posting') + '</th><th>' + text('Run Status', 'Status Run') + '</th><th>' + text('Posting', 'Posting') + '</th><th>' + text('Action', 'Aksi') + '</th></tr></thead><tbody>' + runRows + '</tbody></table></div></section>';
        }
        if (reportKey === 'inventory-valuation' || reportKey === 'inventory-valuation-as-of') {
          return '<div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Warehouse', 'Gudang') + '</th><th>' + text('Item', 'Item') + '</th><th>' + text('Account', 'Akun') + '</th><th>' + text('On Hand', 'On Hand') + '</th><th>' + text('Avg Cost', 'Biaya Rata-Rata') + '</th><th>' + text('Value', 'Nilai') + '</th></tr></thead><tbody>' + (payload.rows || []).map(function(row) {
            return '<tr><td>' + escapeHTML(row.warehouse_code) + '</td><td>' + escapeHTML((row.item_code || '') + ' ' + (row.item_name || '')) + '</td><td>' + escapeHTML(row.account_code) + '</td><td>' + money(row.quantity_on_hand) + '</td><td>' + money(row.average_unit_cost) + '</td><td>' + money(row.inventory_value) + '</td></tr>';
          }).join('') + '</tbody></table></div>';
        }
        if (reportKey === 'inventory-gl-reconciliation') {
          const accountRows = (payload.accounts || []).map(function(row) {
            const openCase = '<button class="finance-report__button" data-open-inventory-case="' + escapeHTML(row.account_code) + '" data-case-inventory="' + escapeHTML(row.inventory_value) + '" data-case-gl="' + escapeHTML(row.gl_value) + '" data-case-reason="' + escapeHTML(text('inventory and GL balances differ', 'saldo inventori dan GL berbeda')) + '">' + escapeHTML(text('Open Case', 'Buka Kasus')) + '</button>';
            return '<tr><td>' + escapeHTML(row.account_code) + '</td><td>' + escapeHTML(row.account_name) + '</td><td>' + money(row.inventory_value) + '</td><td>' + money(row.gl_value) + '</td><td>' + money(row.difference) + '</td><td>' + (row.difference !== 0 ? openCase : '') + '</td></tr>';
          }).join('');
          const mismatchRows = (payload.mismatches || []).map(function(row) {
            return '<tr><td>' + escapeHTML(row.account_code || '') + '</td><td>' + escapeHTML(row.reason || '') + '</td><td>' + money(row.inventory_value) + '</td><td>' + money(row.gl_value) + '</td><td>' + money(row.difference) + '</td></tr>';
          }).join('');
          return '<section class="finance-report__panel"><h3>' + escapeHTML(text('Account Comparison', 'Perbandingan Akun')) + '</h3><div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Account', 'Akun') + '</th><th>' + text('Name', 'Nama') + '</th><th>' + text('Inventory', 'Inventori') + '</th><th>' + text('GL', 'GL') + '</th><th>' + text('Difference', 'Selisih') + '</th><th>' + text('Action', 'Aksi') + '</th></tr></thead><tbody>' + accountRows + '</tbody></table></div></section>'
            + '<section class="finance-report__panel"><h3>' + escapeHTML(text('Mismatches', 'Selisih')) + '</h3><div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Account', 'Akun') + '</th><th>' + text('Reason', 'Alasan') + '</th><th>' + text('Inventory', 'Inventori') + '</th><th>' + text('GL', 'GL') + '</th><th>' + text('Difference', 'Selisih') + '</th></tr></thead><tbody>' + mismatchRows + '</tbody></table></div></section>';
        }
        if (reportKey === 'inventory-adjustment-review') {
          return '<div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Document', 'Dokumen') + '</th><th>' + text('Status', 'Status') + '</th><th>' + text('Warehouse', 'Gudang') + '</th><th>' + text('Lines', 'Baris') + '</th><th>' + text('Qty Delta', 'Delta Qty') + '</th><th>' + text('Value Impact', 'Dampak Nilai') + '</th><th>' + text('Created By', 'Dibuat Oleh') + '</th><th>' + text('Action', 'Aksi') + '</th></tr></thead><tbody>' + (payload.items || []).map(function(row) {
            const generate = row.count_session_id && !row.document_id ? '<button class="finance-report__button" data-generate-adjustment="' + escapeHTML(row.count_session_id) + '">' + escapeHTML(text('Generate Adjustment', 'Generate Penyesuaian')) + '</button>' : '';
            return '<tr><td>' + escapeHTML(row.document_number) + '</td><td>' + escapeHTML(row.status) + '</td><td>' + escapeHTML(row.warehouse_code) + '</td><td>' + escapeHTML(String(row.line_count || 0)) + '</td><td>' + money(row.quantity_delta_total) + '</td><td>' + money(row.estimated_value_impact) + '</td><td>' + escapeHTML(row.created_by) + '</td><td>' + generate + '</td></tr>';
          }).join('') + '</tbody></table></div>';
        }
        if (reportKey === 'ar-aging' || reportKey === 'ap-aging') {
          return '<div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Counterparty', 'Mitra') + '</th><th>' + text('Document', 'Dokumen') + '</th><th>' + text('Date', 'Tanggal') + '</th><th>' + text('Due', 'Jatuh Tempo') + '</th><th>' + text('Account', 'Akun') + '</th><th>' + text('Bucket', 'Kelompok') + '</th><th>' + text('Open', 'Terbuka') + '</th></tr></thead><tbody>' + (payload.items || []).map(function(row) {
            return '<tr><td>' + escapeHTML(row.counterparty_name) + '</td><td>' + escapeHTML(row.document_number) + '</td><td>' + escapeHTML(row.document_date) + '</td><td>' + escapeHTML(row.due_date) + '</td><td>' + escapeHTML(row.account_code) + '</td><td>' + escapeHTML(row.aging_bucket) + '</td><td>' + money(row.open_amount) + '</td></tr>';
          }).join('') + '</tbody></table></div>';
        }
        if (reportKey === 'ar-statements' || reportKey === 'ap-statements') {
          return '<div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Document', 'Dokumen') + '</th><th>' + text('Date', 'Tanggal') + '</th><th>' + text('Due', 'Jatuh Tempo') + '</th><th>' + text('Account', 'Akun') + '</th><th>' + text('Settled', 'Terselesaikan') + '</th><th>' + text('Write-off', 'Write-off') + '</th><th>' + text('Open', 'Terbuka') + '</th><th>' + text('Bucket', 'Kelompok') + '</th></tr></thead><tbody>' + (payload.rows || []).map(function(row) {
            return '<tr><td>' + escapeHTML(row.document_number) + '</td><td>' + escapeHTML(row.document_date) + '</td><td>' + escapeHTML(row.due_date) + '</td><td>' + escapeHTML(row.account_code) + '</td><td>' + money(row.settled_amount) + '</td><td>' + money(row.writeoff_amount) + '</td><td>' + money(row.open_amount) + '</td><td>' + escapeHTML(row.aging_bucket) + '</td></tr>';
          }).join('') + '</tbody></table></div>';
        }
        if (reportKey === 'collections') {
          return '<div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Counterparty', 'Mitra') + '</th><th>' + text('Kind', 'Jenis') + '</th><th>' + text('Status', 'Status') + '</th><th>' + text('Assignee', 'Penanggung Jawab') + '</th><th>' + text('Open', 'Terbuka') + '</th><th>' + text('Overdue', 'Lewat Jatuh Tempo') + '</th><th>' + text('Follow Up', 'Tindak Lanjut') + '</th><th>' + text('Action', 'Aksi') + '</th></tr></thead><tbody>' + (payload.items || []).map(function(row) {
            return '<tr><td>' + escapeHTML(row.counterparty_name) + '</td><td>' + escapeHTML(row.kind) + '</td><td>' + escapeHTML(row.status) + '</td><td>' + escapeHTML(row.assignee_user_id) + '</td><td>' + money(row.total_open_amount) + '</td><td>' + money(row.overdue_amount) + '</td><td>' + escapeHTML(row.follow_up_date) + '</td><td><button class="finance-report__button" data-case-refresh="' + escapeHTML(row.id) + '">' + escapeHTML(text('Refresh', 'Refresh')) + '</button></td></tr>';
          }).join('') + '</tbody></table></div>';
        }
        if (reportKey === 'settlement-exceptions') {
          return '<section class="finance-report__panel"><div class="finance-report__actions"><button class="finance-report__button finance-report__button--primary" data-sync-exceptions="1">' + escapeHTML(text('Refresh Snapshot', 'Refresh Snapshot')) + '</button></div></section>'
            + '<div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Counterparty', 'Mitra') + '</th><th>' + text('Type', 'Tipe') + '</th><th>' + text('Document/Payment', 'Dokumen/Pembayaran') + '</th><th>' + text('Account', 'Akun') + '</th><th>' + text('Open', 'Terbuka') + '</th><th>' + text('Unapplied', 'Belum Dialokasikan') + '</th><th>' + text('Action', 'Aksi') + '</th></tr></thead><tbody>' + (payload.items || []).map(function(row) {
            const reference = row.source_document_number || row.source_payment_number || '';
            const actionKey = row.id || '';
            const applyButton = actionKey && row.source_payment_id ? '<button class="finance-report__button" data-exception-apply="' + escapeHTML(actionKey) + '">' + escapeHTML(text('Apply', 'Alokasikan')) + '</button>' : '';
            const writeoffButton = actionKey && row.source_document_id ? '<button class="finance-report__button" data-exception-writeoff="' + escapeHTML(actionKey) + '">' + escapeHTML(text('Write-off', 'Write-off')) + '</button>' : '';
            const caseButton = actionKey ? '<button class="finance-report__button" data-exception-case="' + escapeHTML(actionKey) + '">' + escapeHTML(text('Open Case', 'Buka Kasus')) + '</button>' : '';
            const actionCell = actionKey ? '<div class="finance-report__actions">' + applyButton + writeoffButton + caseButton + '</div>' : '<span class="finance-report__pill">' + escapeHTML(text('Sync first', 'Sync dulu')) + '</span>';
            return '<tr><td>' + escapeHTML(row.counterparty_name) + '</td><td>' + escapeHTML(row.exception_type) + '</td><td>' + escapeHTML(reference) + '</td><td>' + escapeHTML(row.account_code) + '</td><td>' + money(row.open_amount) + '</td><td>' + money(row.unapplied_amount) + '</td><td>' + actionCell + '</td></tr>';
          }).join('') + '</tbody></table></div>';
        }
        if (reportKey === 'ar-reconciliation' || reportKey === 'ap-reconciliation') {
          const accountRows = (payload.accounts || []).map(function(row) {
            return '<tr><td>' + escapeHTML(row.account_code) + '</td><td>' + escapeHTML(row.account_name) + '</td><td>' + money(row.subledger_amount) + '</td><td>' + money(row.gl_amount) + '</td><td>' + money(row.difference) + '</td></tr>';
          }).join('');
          const mismatchRows = (payload.mismatches || []).map(function(row) {
            return '<tr><td>' + escapeHTML(row.account_code || row.document_number || '') + '</td><td>' + escapeHTML(row.reason || '') + '</td><td>' + money(row.subledger_amount) + '</td><td>' + money(row.gl_amount) + '</td><td>' + money(row.difference) + '</td></tr>';
          }).join('');
          return '<section class="finance-report__panel"><h3>' + escapeHTML(text('Account Comparison', 'Perbandingan Akun')) + '</h3><div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Account', 'Akun') + '</th><th>' + text('Name', 'Nama') + '</th><th>' + text('Subledger', 'Subledger') + '</th><th>' + text('GL', 'GL') + '</th><th>' + text('Difference', 'Selisih') + '</th></tr></thead><tbody>' + accountRows + '</tbody></table></div></section>'
            + '<section class="finance-report__panel"><h3>' + escapeHTML(text('Mismatches', 'Selisih')) + '</h3><div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Reference', 'Referensi') + '</th><th>' + text('Reason', 'Alasan') + '</th><th>' + text('Subledger', 'Subledger') + '</th><th>' + text('GL', 'GL') + '</th><th>' + text('Difference', 'Selisih') + '</th></tr></thead><tbody>' + mismatchRows + '</tbody></table></div></section>';
        }
        if (reportKey === 'profit-and-loss' || reportKey === 'balance-sheet') {
          return (payload.sections || []).map(function(section) {
            const rows = (section.rows || []).map(function(row) {
              return '<tr><td>' + escapeHTML(row.account_code) + '</td><td>' + escapeHTML(row.account_name) + '</td><td>' + money(row.amount) + '</td></tr>';
            }).join('');
            return '<section class="finance-report__panel"><h3>' + escapeHTML(section.label) + '</h3><div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Account', 'Akun') + '</th><th>' + text('Name', 'Nama') + '</th><th>' + text('Amount', 'Jumlah') + '</th></tr></thead><tbody>' + rows + '<tr><td colspan="2"><strong>' + text('Section Total', 'Total Bagian') + '</strong></td><td><strong>' + money(section.amount) + '</strong></td></tr></tbody></table></div></section>';
          }).join('');
        }
        const rows = payload.rows || [];
        if (reportKey === 'tax-summary') {
          return '<div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Account', 'Akun') + '</th><th>' + text('Name', 'Nama') + '</th><th>' + text('Bucket', 'Kelompok') + '</th><th>' + text('Debit', 'Debit') + '</th><th>' + text('Credit', 'Kredit') + '</th><th>' + text('Net', 'Bersih') + '</th></tr></thead><tbody>' + rows.map(function(row) {
            return '<tr><td>' + escapeHTML(row.account_code) + '</td><td>' + escapeHTML(row.account_name) + '</td><td>' + escapeHTML(row.tax_bucket) + '</td><td>' + money(row.debit) + '</td><td>' + money(row.credit) + '</td><td>' + money(row.net_amount) + '</td></tr>';
          }).join('') + '</tbody></table></div>';
        }
        if (reportKey === 'journal-ledger') {
          return '<div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Date', 'Tanggal') + '</th><th>' + text('Posting', 'Posting') + '</th><th>' + text('Source', 'Sumber') + '</th><th>' + text('Account', 'Akun') + '</th><th>' + text('Description', 'Deskripsi') + '</th><th>' + text('Debit', 'Debit') + '</th><th>' + text('Credit', 'Kredit') + '</th></tr></thead><tbody>' + rows.map(function(row) {
            return '<tr><td>' + escapeHTML(row.posting_date) + '</td><td>' + escapeHTML(row.posting_number) + '</td><td>' + escapeHTML((row.source_document_type || '') + ' ' + (row.source_document_id || '')) + '</td><td>' + escapeHTML(row.account_code) + '</td><td>' + escapeHTML(row.description) + '</td><td>' + money(row.debit) + '</td><td>' + money(row.credit) + '</td></tr>';
          }).join('') + '</tbody></table></div>';
        }
        return '<div class="finance-report__table-wrap"><table class="finance-report__table"><thead><tr><th>' + text('Account', 'Akun') + '</th><th>' + text('Name', 'Nama') + '</th><th>' + text('Type', 'Tipe') + '</th><th>' + text('Opening', 'Awal') + '</th><th>' + text('Debit', 'Debit') + '</th><th>' + text('Credit', 'Kredit') + '</th><th>' + text('Ending', 'Akhir') + '</th></tr></thead><tbody>' + rows.map(function(row) {
          return '<tr><td>' + escapeHTML(row.account_code) + '</td><td>' + escapeHTML(row.account_name) + '</td><td>' + escapeHTML(row.account_type) + '</td><td>' + money(row.opening) + '</td><td>' + money(row.debit) + '</td><td>' + money(row.credit) + '</td><td>' + money(row.ending) + '</td></tr>';
        }).join('') + '</tbody></table></div>';
      }
      function summaryCards(payload) {
        if (reportKey === 'period-close') return '';
        if (reportKey === 'profit-and-loss') {
          return '<section class="finance-report__cards"><article class="finance-report__card"><span>' + text('Gross Profit', 'Laba Kotor') + '</span><strong>' + money(payload.gross_profit) + '</strong></article><article class="finance-report__card"><span>' + text('Net Profit', 'Laba Bersih') + '</span><strong>' + money(payload.net_profit) + '</strong></article></section>';
        }
        if (reportKey === 'balance-sheet') {
          return '<section class="finance-report__cards"><article class="finance-report__card"><span>' + text('Retained Earnings', 'Laba Ditahan') + '</span><strong>' + money(payload.retained_earnings) + '</strong></article></section>';
        }
        if (reportKey === 'ar-reconciliation' || reportKey === 'ap-reconciliation') {
          return '<section class="finance-report__cards"><article class="finance-report__card"><span>' + text('Subledger', 'Subledger') + '</span><strong>' + money(payload.subledger_total) + '</strong></article><article class="finance-report__card"><span>' + text('GL', 'GL') + '</span><strong>' + money(payload.gl_total) + '</strong></article><article class="finance-report__card"><span>' + text('Difference', 'Selisih') + '</span><strong>' + money(payload.difference) + '</strong></article></section>';
        }
        if (reportKey === 'inventory-gl-reconciliation') {
          return '<section class="finance-report__cards"><article class="finance-report__card"><span>' + text('Inventory', 'Inventori') + '</span><strong>' + money(payload.inventory_total) + '</strong></article><article class="finance-report__card"><span>' + text('GL', 'GL') + '</span><strong>' + money(payload.gl_total) + '</strong></article><article class="finance-report__card"><span>' + text('Difference', 'Selisih') + '</span><strong>' + money(payload.difference) + '</strong></article></section>';
        }
        if (reportKey === 'ar-statements' || reportKey === 'ap-statements' || reportKey === 'settlement-exceptions') {
          const totals = payload.totals || {};
          return '<section class="finance-report__cards">' + Object.keys(totals).map(function(key) {
            return '<article class="finance-report__card"><span>' + escapeHTML(key.replace(/_/g, ' ')) + '</span><strong>' + money(totals[key]) + '</strong></article>';
          }).join('') + '</section>';
        }
        const totals = payload.totals || {};
        const keys = Object.keys(totals);
        if (!keys.length) return '';
        return '<section class="finance-report__cards">' + keys.map(function(key) {
          return '<article class="finance-report__card"><span>' + escapeHTML(key.replace(/_/g, ' ')) + '</span><strong>' + money(totals[key]) + '</strong></article>';
        }).join('') + '</section>';
      }
      function navItems() {
        return [
          { key: 'period-close', label: text('Period Close', 'Tutup Periode'), path: '/ui/finance/period-close' },
          { key: 'inventory-valuation', label: text('Inventory Valuation', 'Penilaian Inventori'), path: '/ui/finance/inventory-valuation' },
          { key: 'inventory-valuation-as-of', label: text('Inventory Valuation As Of', 'Penilaian Inventori Per Tanggal'), path: '/ui/finance/inventory-valuation-as-of' },
          { key: 'inventory-gl-reconciliation', label: text('Inventory GL Reconciliation', 'Rekonsiliasi GL Inventori'), path: '/ui/finance/inventory-gl-reconciliation' },
          { key: 'inventory-adjustment-review', label: text('Adjustment Review', 'Tinjauan Penyesuaian'), path: '/ui/finance/inventory-adjustment-review' },
          { key: 'ar-statements', label: text('AR Statements', 'Statement Piutang'), path: '/ui/finance/ar-statements' },
          { key: 'ap-statements', label: text('AP Statements', 'Statement Utang'), path: '/ui/finance/ap-statements' },
          { key: 'collections', label: text('Collections', 'Penagihan'), path: '/ui/finance/collections' },
          { key: 'settlement-exceptions', label: text('Settlement Exceptions', 'Pengecualian Settlement'), path: '/ui/finance/settlement-exceptions' },
          { key: 'trial-balance', label: text('Trial Balance', 'Neraca Saldo'), path: '/ui/finance/trial-balance' },
          { key: 'profit-and-loss', label: text('Profit and Loss', 'Laba Rugi'), path: '/ui/finance/profit-and-loss' },
          { key: 'balance-sheet', label: text('Balance Sheet', 'Neraca'), path: '/ui/finance/balance-sheet' },
          { key: 'tax-summary', label: text('Tax Summary', 'Ringkasan Pajak'), path: '/ui/finance/tax-summary' },
          { key: 'ar-aging', label: text('AR Aging', 'Umur Piutang'), path: '/ui/finance/ar-aging' },
          { key: 'ap-aging', label: text('AP Aging', 'Umur Utang'), path: '/ui/finance/ap-aging' },
          { key: 'ar-reconciliation', label: text('AR Reconciliation', 'Rekonsiliasi Piutang'), path: '/ui/finance/ar-reconciliation' },
          { key: 'ap-reconciliation', label: text('AP Reconciliation', 'Rekonsiliasi Utang'), path: '/ui/finance/ap-reconciliation' },
          { key: 'journal-ledger', label: text('Journal Ledger', 'Buku Jurnal'), path: '/ui/finance/journal-ledger' }
        ];
      }
      function renderFilters(payload) {
        if (reportKey === 'period-close') {
          const options = (payload.periods || []).map(function(period) {
            const selected = filters.period_id === period.id || (!filters.period_id && payload.pack && payload.pack.period_id === period.id);
            return '<option value="' + escapeHTML(period.id) + '"' + (selected ? ' selected' : '') + '>' + escapeHTML((period.values && period.values.period_key) || period.id) + '</option>';
          }).join('');
          return '<section class="finance-report__panel"><div class="finance-report__filters"><label class="finance-report__field"><span>' + text('Accounting Period', 'Periode Akuntansi') + '</span><select data-filter="period_id">' + options + '</select></label><button class="finance-report__button finance-report__button--primary" data-apply>' + escapeHTML(text('Load', 'Muat')) + '</button></div></section>';
        }
        return '<section class="finance-report__panel"><div class="finance-report__filters">'
          + (reportKey === 'collections' || reportKey === 'inventory-adjustment-review'
              ? ''
              : (usesAsOf
              ? '<label class="finance-report__field"><span>' + text('As Of', 'Per Tanggal') + '</span><input data-filter="as_of_date" type="date" value="' + escapeHTML(filters.as_of_date) + '" /></label>'
              : '<label class="finance-report__field"><span>' + text('From', 'Dari') + '</span><input data-filter="from_date" type="date" value="' + escapeHTML(filters.from_date) + '" /></label><label class="finance-report__field"><span>' + text('To', 'Sampai') + '</span><input data-filter="to_date" type="date" value="' + escapeHTML(filters.to_date) + '" /></label>'))
          + ((reportKey === 'ar-aging' || reportKey === 'ar-reconciliation') ? '<label class="finance-report__field"><span>' + text('Party', 'Pihak') + '</span><input data-filter="party_id" value="' + escapeHTML(filters.party_id) + '" /></label>' : '')
          + ((reportKey === 'ap-aging' || reportKey === 'ap-reconciliation') ? '<label class="finance-report__field"><span>' + text('Vendor', 'Vendor') + '</span><input data-filter="vendor_id" value="' + escapeHTML(filters.vendor_id) + '" /></label>' : '')
          + (reportKey === 'ar-statements' ? '<label class="finance-report__field"><span>' + text('Party', 'Pihak') + '</span><input data-filter="party_id" value="' + escapeHTML(filters.party_id) + '" /></label>' : '')
          + (reportKey === 'ap-statements' ? '<label class="finance-report__field"><span>' + text('Vendor', 'Vendor') + '</span><input data-filter="vendor_id" value="' + escapeHTML(filters.vendor_id) + '" /></label>' : '')
          + ((reportKey === 'ar-reconciliation' || reportKey === 'ap-reconciliation' || reportKey === 'inventory-gl-reconciliation') ? '<label class="finance-report__field"><span>' + text('Account', 'Akun') + '</span><input data-filter="account_code" value="' + escapeHTML(filters.account_code) + '" /></label>' : '')
          + ((reportKey === 'inventory-valuation' || reportKey === 'inventory-valuation-as-of') ? '<label class="finance-report__field"><span>' + text('Warehouse', 'Gudang') + '</span><input data-filter="warehouse_code" value="' + escapeHTML(filters.warehouse_code) + '" /></label>' : '')
          + ((reportKey === 'ar-aging' || reportKey === 'ap-aging') ? '<label class="finance-report__field"><span>' + text('Bucket', 'Kelompok') + '</span><input data-filter="aging_bucket" value="' + escapeHTML(filters.aging_bucket) + '" /></label>' : '')
          + (reportKey === 'collections' || reportKey === 'settlement-exceptions' ? '<label class="finance-report__field"><span>' + text('Kind', 'Jenis') + '</span><input data-filter="kind" value="' + escapeHTML(filters.kind) + '" /></label>' : '')
          + (reportKey === 'collections' || reportKey === 'inventory-adjustment-review' ? '<label class="finance-report__field"><span>' + text('Status', 'Status') + '</span><input data-filter="status" value="' + escapeHTML(filters.status) + '" /></label>' : '')
          + '<button class="finance-report__button finance-report__button--primary" data-apply>' + escapeHTML(text('Apply', 'Terapkan')) + '</button>'
          + ((reportKey === 'ar-statements' || reportKey === 'ap-statements') ? '<button class="finance-report__button" data-generate-statement="' + escapeHTML(reportKey) + '">' + escapeHTML(text('Generate Snapshot', 'Generate Snapshot')) + '</button>' : '')
          + '</div></section>';
      }
      ensureStyles();
      const payload = await loadReport();
      mount.innerHTML = ''
        + '<section class="finance-report">'
        +   '<section class="finance-report__hero"><div><h2>' + escapeHTML(title) + '</h2><p>' + escapeHTML(reportKey === 'period-close' ? text('Period-end journal generation, checklist readiness, and close controls.', 'Generate jurnal akhir periode, kesiapan checklist, dan kontrol tutup periode.') : ((reportKey === 'inventory-valuation' || reportKey === 'inventory-valuation-as-of' || reportKey === 'inventory-gl-reconciliation' || reportKey === 'inventory-adjustment-review') ? text('Inventory valuation, reconciliation, and adjustment governance controls.', 'Kontrol penilaian inventori, rekonsiliasi, dan tata kelola penyesuaian.') : text('Finance statements and tax visibility from posted journals.', 'Laporan keuangan dan visibilitas pajak dari jurnal yang sudah diposting.'))) + '</p></div><nav class="finance-report__nav">' + navItems().map(function(item) {
              return '<a href="' + item.path + '" class="' + (item.key === reportKey ? 'is-active' : '') + '">' + escapeHTML(item.label) + '</a>';
            }).join('') + '</nav></section>'
        +   renderFilters(payload)
        +   summaryCards(payload.pack || payload)
        +   renderRows(payload.pack ? payload.pack : payload)
        + '</section>';
      const apply = mount.querySelector('[data-apply]');
      if (apply) {
        apply.addEventListener('click', function() {
          mount.querySelectorAll('[data-filter]').forEach(function(node) {
            filters[node.getAttribute('data-filter')] = node.value || '';
          });
          const next = new URL(window.location.href);
          if (reportKey === 'period-close') {
            if (filters.period_id) next.searchParams.set('period_id', filters.period_id); else next.searchParams.delete('period_id');
          } else if (usesAsOf) {
            if (filters.as_of_date) next.searchParams.set('as_of_date', filters.as_of_date); else next.searchParams.delete('as_of_date');
            next.searchParams.delete('from_date');
            next.searchParams.delete('to_date');
          } else {
            if (filters.from_date) next.searchParams.set('from_date', filters.from_date); else next.searchParams.delete('from_date');
            if (filters.to_date) next.searchParams.set('to_date', filters.to_date); else next.searchParams.delete('to_date');
            next.searchParams.delete('as_of_date');
          }
          if (filters.party_id) next.searchParams.set('party_id', filters.party_id); else next.searchParams.delete('party_id');
          if (filters.vendor_id) next.searchParams.set('vendor_id', filters.vendor_id); else next.searchParams.delete('vendor_id');
          if (filters.account_code) next.searchParams.set('account_code', filters.account_code); else next.searchParams.delete('account_code');
          if (filters.warehouse_code) next.searchParams.set('warehouse_code', filters.warehouse_code); else next.searchParams.delete('warehouse_code');
          if (filters.aging_bucket) next.searchParams.set('aging_bucket', filters.aging_bucket); else next.searchParams.delete('aging_bucket');
          if (filters.kind) next.searchParams.set('kind', filters.kind); else next.searchParams.delete('kind');
          if (filters.status) next.searchParams.set('status', filters.status); else next.searchParams.delete('status');
          window.location.assign(next.toString());
        });
      }
      mount.querySelectorAll('[data-generate-statement]').forEach(function(node) {
        node.addEventListener('click', async function() {
          if (reportKey === 'ar-statements') {
            await postJSON('/ui/data/finance/ar-statements/generate', { party_id: filters.party_id, as_of_date: filters.as_of_date });
          } else if (reportKey === 'ap-statements') {
            await postJSON('/ui/data/finance/ap-statements/generate', { vendor_id: filters.vendor_id, as_of_date: filters.as_of_date });
          }
          window.location.reload();
        });
      });
      mount.querySelectorAll('[data-sync-exceptions]').forEach(function(node) {
        node.addEventListener('click', async function() {
          await postJSON('/ui/data/finance/settlement-exceptions/sync', { as_of_date: filters.as_of_date, kind: filters.kind });
          window.location.reload();
        });
      });
      mount.querySelectorAll('[data-exception-case]').forEach(function(node) {
        node.addEventListener('click', async function() {
          await postJSON('/ui/data/finance/settlement-exceptions/' + encodeURIComponent(node.getAttribute('data-exception-case')) + '/open-case');
          window.location.assign('/ui/finance/collection-cases');
        });
      });
      mount.querySelectorAll('[data-exception-apply]').forEach(function(node) {
        node.addEventListener('click', async function() {
          const targetDocumentID = window.prompt(text('Enter target invoice or bill ID', 'Masukkan ID invoice atau bill tujuan'));
          if (!targetDocumentID) return;
          const amountValue = window.prompt(text('Enter amount to apply', 'Masukkan jumlah alokasi'));
          await postJSON('/ui/data/finance/settlement-exceptions/' + encodeURIComponent(node.getAttribute('data-exception-apply')) + '/apply', { target_document_id: targetDocumentID, amount: Number(amountValue || 0) });
          window.location.reload();
        });
      });
      mount.querySelectorAll('[data-exception-writeoff]').forEach(function(node) {
        node.addEventListener('click', async function() {
          const postingDate = window.prompt(text('Enter posting date (YYYY-MM-DD)', 'Masukkan tanggal posting (YYYY-MM-DD)'));
          if (!postingDate) return;
          const amountValue = window.prompt(text('Enter write-off amount', 'Masukkan jumlah write-off'));
          await postJSON('/ui/data/finance/settlement-exceptions/' + encodeURIComponent(node.getAttribute('data-exception-writeoff')) + '/write-off', { posting_date: postingDate, amount: Number(amountValue || 0) });
          window.location.reload();
        });
      });
      mount.querySelectorAll('[data-case-refresh]').forEach(function(node) {
        node.addEventListener('click', async function() {
          await postJSON('/ui/data/finance/collections/' + encodeURIComponent(node.getAttribute('data-case-refresh')) + '/refresh');
          window.location.reload();
        });
      });
      mount.querySelectorAll('[data-open-inventory-case]').forEach(function(node) {
        node.addEventListener('click', async function() {
          await postJSON('/ui/data/finance/inventory-reconciliation-cases/open', {
            as_of_date: filters.as_of_date,
            account_code: node.getAttribute('data-open-inventory-case'),
            inventory_value: Number(node.getAttribute('data-case-inventory') || 0),
            gl_value: Number(node.getAttribute('data-case-gl') || 0),
            reason: node.getAttribute('data-case-reason') || ''
          });
          window.location.assign('/ui/finance/inventory-reconciliation-cases');
        });
      });
      mount.querySelectorAll('[data-generate-adjustment]').forEach(function(node) {
        node.addEventListener('click', async function() {
          await postJSON('/ui/data/inventory/count-sessions/' + encodeURIComponent(node.getAttribute('data-generate-adjustment')) + '/generate-adjustment');
          window.location.assign('/ui/inventory/adjustments');
        });
      });
      mount.querySelectorAll('[data-generate-journals]').forEach(function(node) {
        node.addEventListener('click', async function() {
          const periodID = node.getAttribute('data-generate-journals');
          await postJSON('/ui/data/finance/periods/' + encodeURIComponent(periodID) + '/generate-journals');
          window.location.reload();
        });
      });
      mount.querySelectorAll('[data-close-period]').forEach(function(node) {
        node.addEventListener('click', async function() {
          const periodID = node.getAttribute('data-close-period');
          await postJSON('/ui/data/finance/periods/' + encodeURIComponent(periodID) + '/close');
          window.location.reload();
        });
      });
      mount.querySelectorAll('[data-reopen-period]').forEach(function(node) {
        node.addEventListener('click', async function() {
          const periodID = node.getAttribute('data-reopen-period');
          await postJSON('/ui/data/finance/periods/' + encodeURIComponent(periodID) + '/reopen');
          window.location.reload();
        });
      });
      mount.querySelectorAll('[data-task-complete]').forEach(function(node) {
        node.addEventListener('click', async function() {
          await postJSON('/ui/data/finance/period-tasks/' + encodeURIComponent(node.getAttribute('data-task-complete')) + '/complete');
          window.location.reload();
        });
      });
      mount.querySelectorAll('[data-task-waive]').forEach(function(node) {
        node.addEventListener('click', async function() {
          await postJSON('/ui/data/finance/period-tasks/' + encodeURIComponent(node.getAttribute('data-task-waive')) + '/waive');
          window.location.reload();
        });
      });
      mount.querySelectorAll('[data-posting-reverse]').forEach(function(node) {
        node.addEventListener('click', async function() {
          const reversalDate = window.prompt(text('Enter reversal date (YYYY-MM-DD)', 'Masukkan tanggal reversal (YYYY-MM-DD)'));
          if (!reversalDate) return;
          await postJSON('/ui/data/finance/journals/' + encodeURIComponent(node.getAttribute('data-posting-reverse')) + '/reverse', { reversal_date: reversalDate });
          window.location.reload();
        });
      });
    }
  };
})();`
}
