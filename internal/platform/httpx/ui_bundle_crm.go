package httpx

func CRMCustomer360Bundle() string {
	return `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  window.ClinicModuleBundles["crm-customer-360"] = {
    render: async function(ctx) {
      const text = function(en, id) { return ctx.locale === "id" ? id : en; };
      const escapeHTML = function(value) {
        return String(value == null ? '' : value).replace(/[&<>"]/g, function(char) {
          return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[char];
        });
      };
      const formatNumber = function(value) {
        return new Intl.NumberFormat(ctx.locale === 'id' ? 'id-ID' : 'en-US').format(Number(value || 0));
      };
      const formatCurrency = function(value) {
        return new Intl.NumberFormat(ctx.locale === 'id' ? 'id-ID' : 'en-US', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 }).format(Number(value || 0));
      };
      const ensureStyles = function() {
        if (document.getElementById('crm-customer-360-styles')) return;
        const style = document.createElement('style');
        style.id = 'crm-customer-360-styles';
        style.textContent = ''
          + '.crm360 { display:grid; gap:1.25rem; }'
          + '.crm360__hero, .crm360__panel, .crm360__customer-button { border:1px solid var(--color-line); background:var(--color-surface); box-shadow:var(--shadow-panel); }'
          + '.crm360__hero { border-radius:1.2rem; padding:1.25rem; background:linear-gradient(135deg, color-mix(in srgb, var(--color-accent-soft) 52%, var(--color-surface) 48%), var(--color-surface)); }'
          + '.crm360__eyebrow { display:inline-flex; padding:0.28rem 0.55rem; border-radius:999px; background:color-mix(in srgb, var(--color-accent) 10%, var(--color-surface)); color:var(--color-accent-dark); font-size:0.72rem; font-weight:800; letter-spacing:0.14em; text-transform:uppercase; }'
          + '.crm360__hero h2 { margin:0.7rem 0 0.35rem; font-size:clamp(1.6rem, 3vw, 2.3rem); line-height:1.08; }'
          + '.crm360__hero p { margin:0; color:var(--color-muted); line-height:1.55; }'
          + '.crm360__metrics { display:grid; grid-template-columns:repeat(auto-fit, minmax(12rem,1fr)); gap:1rem; }'
          + '.crm360__metric { border:1px solid var(--color-line); border-radius:1rem; padding:1rem; background:color-mix(in srgb, var(--color-shell) 35%, var(--color-surface)); }'
          + '.crm360__metric span { display:block; color:var(--color-muted); font-size:0.76rem; font-weight:800; letter-spacing:0.12em; text-transform:uppercase; }'
          + '.crm360__metric strong { display:block; margin-top:0.4rem; font-size:1.55rem; }'
          + '.crm360__grid { display:grid; gap:1rem; grid-template-columns:repeat(auto-fit, minmax(22rem,1fr)); }'
          + '.crm360__panel { border-radius:1rem; overflow:hidden; }'
          + '.crm360__panel-head { padding:0.95rem 1rem; border-bottom:1px solid var(--color-line); background:color-mix(in srgb, var(--color-accent-soft) 26%, var(--color-surface)); }'
          + '.crm360__panel-head h3 { margin:0; font-size:1rem; }'
          + '.crm360__panel-head p { margin:0.2rem 0 0; font-size:0.84rem; color:var(--color-muted); }'
          + '.crm360__panel-body { padding:1rem; }'
          + '.crm360__list { display:grid; gap:0.65rem; }'
          + '.crm360__card { border:1px solid var(--color-line); border-radius:0.9rem; padding:0.82rem 0.9rem; background:color-mix(in srgb, var(--color-shell) 28%, var(--color-surface)); }'
          + '.crm360__card-head { display:flex; justify-content:space-between; gap:0.75rem; align-items:flex-start; }'
          + '.crm360__card-title { font-weight:800; }'
          + '.crm360__meta { color:var(--color-muted); font-size:0.84rem; margin-top:0.2rem; }'
          + '.crm360__pill { display:inline-flex; align-items:center; gap:0.28rem; border-radius:999px; padding:0.2rem 0.45rem; background:color-mix(in srgb, var(--color-accent-soft) 48%, var(--color-surface)); color:var(--color-accent-dark); font-size:0.7rem; font-weight:800; text-transform:uppercase; letter-spacing:0.08em; }'
          + '.crm360__search { display:flex; flex-wrap:wrap; gap:0.75rem; align-items:end; }'
          + '.crm360__search label { display:grid; gap:0.35rem; min-width:min(28rem,100%); flex:1; }'
          + '.crm360__search input { min-height:2.8rem; border:1px solid var(--color-line); border-radius:0.9rem; background:var(--color-surface); color:var(--color-body); padding:0.7rem 0.85rem; font:inherit; }'
          + '.crm360__button, .crm360__customer-button { appearance:none; cursor:pointer; font:inherit; }'
          + '.crm360__button { border-radius:0.9rem; padding:0.8rem 1rem; background:var(--color-accent); border:1px solid var(--color-accent); color:#fff; font-weight:800; }'
          + '.crm360__button--soft { background:var(--color-surface); border:1px solid var(--color-line); color:var(--color-body); }'
          + '.crm360__customer-grid { display:grid; gap:0.75rem; grid-template-columns:repeat(auto-fit, minmax(15rem,1fr)); }'
          + '.crm360__customer-button { border-radius:1rem; padding:1rem; text-align:left; }'
          + '.crm360__empty { color:var(--color-muted); text-align:center; padding:1rem; border:1px dashed var(--color-line); border-radius:1rem; }';
        document.head.appendChild(style);
      };
      ensureStyles();
      const state = {
        partyID: (ctx.params.party_id || '').trim(),
        health: null,
        summary: null
      };
      const title = text('Customer 360', 'Customer 360');
      async function loadHealth() {
        state.health = await ctx.api('/ui/data/crm/customers/health');
      }
      async function loadSummary(partyID) {
        state.summary = null;
        if (!partyID) return;
        state.summary = await ctx.api('/ui/data/crm/customers/360/' + encodeURIComponent(partyID));
      }
      function navigate(partyID) {
        const url = new URL(window.location.href);
        if (partyID) {
          url.searchParams.set('party_id', partyID);
        } else {
          url.searchParams.delete('party_id');
        }
        window.location.assign(url.pathname + url.search);
      }
      function renderAtRiskCustomers() {
        const items = ((state.health || {}).items || []).slice(0, 8);
        if (!items.length) {
          return '<div class="crm360__empty">' + text('No at-risk customers found yet.', 'Belum ada pelanggan berisiko.') + '</div>';
        }
        return '<div class="crm360__customer-grid">' + items.map(function(item) {
          return '<button type="button" class="crm360__customer-button" data-party-id="' + escapeHTML(item.party_id) + '">'
            + '<div class="crm360__card-title">' + escapeHTML(item.party_name) + '</div>'
            + '<div class="crm360__meta">' + text('Open tickets', 'Tiket terbuka') + ': ' + formatNumber(item.open_tickets) + ' · ' + text('Overdue', 'Lewat SLA') + ': ' + formatNumber(item.overdue_tickets) + '</div>'
            + '<div class="crm360__meta">' + text('Open opportunity value', 'Nilai peluang terbuka') + ': ' + formatCurrency(item.open_opportunity_value) + '</div>'
          + '</button>';
        }).join('') + '</div>';
      }
      function renderSummary() {
        if (!state.summary) {
          return '<div class="crm360__empty">' + text('Choose a customer from the at-risk list or open this page with ?party_id=...', 'Pilih pelanggan dari daftar berisiko atau buka halaman ini dengan ?party_id=...') + '</div>';
        }
        const overview = state.summary.overview || {};
        const party = state.summary.party || {};
        const partyValues = party.values || {};
        const profile = (state.summary.customer_profile || {}).values || {};
        const tickets = state.summary.tickets || [];
        const opportunities = state.summary.opportunities || [];
        const activities = state.summary.activities || [];
        return ''
          + '<section class="crm360__hero">'
          +   '<span class="crm360__eyebrow">' + text('Customer Workspace', 'Workspace Pelanggan') + '</span>'
          +   '<h2>' + escapeHTML(partyValues.name || profile.customer_name || text('Customer', 'Pelanggan')) + '</h2>'
          +   '<p>' + text('Unified service and sales view built from shared party, customer profile, ticket, and opportunity records.', 'Tampilan layanan dan penjualan terpadu dari data pihak, profil pelanggan, tiket, dan peluang.') + '</p>'
          + '</section>'
          + '<section class="crm360__metrics">'
          +   '<article class="crm360__metric"><span>' + text('Open Tickets', 'Tiket Terbuka') + '</span><strong>' + formatNumber(overview.open_tickets) + '</strong></article>'
          +   '<article class="crm360__metric"><span>' + text('Overdue Tickets', 'Tiket Lewat SLA') + '</span><strong>' + formatNumber(overview.overdue_tickets) + '</strong></article>'
          +   '<article class="crm360__metric"><span>' + text('Open Opportunity Value', 'Nilai Peluang Terbuka') + '</span><strong>' + formatCurrency(overview.open_opportunity_value) + '</strong></article>'
          +   '<article class="crm360__metric"><span>' + text('Member Tier', 'Tier Member') + '</span><strong>' + escapeHTML(overview.member_tier || '-') + '</strong></article>'
          + '</section>'
          + '<section class="crm360__grid">'
          +   '<section class="crm360__panel"><div class="crm360__panel-head"><h3>' + text('Recent Tickets', 'Tiket Terbaru') + '</h3><p>' + text('Current service workload for this customer.', 'Beban layanan saat ini untuk pelanggan ini.') + '</p></div><div class="crm360__panel-body">' + (tickets.length ? '<div class="crm360__list">' + tickets.slice(0, 6).map(function(item) {
                const values = item.values || {};
                return '<article class="crm360__card"><div class="crm360__card-head"><div><div class="crm360__card-title">' + escapeHTML(values.ticket_number || values.title || '') + '</div><div class="crm360__meta">' + escapeHTML(values.title || '') + '</div></div><span class="crm360__pill">' + escapeHTML(values.status || '-') + '</span></div><div class="crm360__meta">' + escapeHTML(values.queue_code || '-') + ' · ' + escapeHTML(values.priority || '-') + '</div></article>';
              }).join('') + '</div>' : '<div class="crm360__empty">' + text('No tickets for this customer.', 'Tidak ada tiket untuk pelanggan ini.') + '</div>') + '</div></section>'
          +   '<section class="crm360__panel"><div class="crm360__panel-head"><h3>' + text('Opportunities', 'Peluang') + '</h3><p>' + text('Open and recent pipeline items tied to this customer.', 'Item pipeline terbuka dan terbaru yang terkait dengan pelanggan ini.') + '</p></div><div class="crm360__panel-body">' + (opportunities.length ? '<div class="crm360__list">' + opportunities.slice(0, 6).map(function(item) {
                const values = item.values || {};
                return '<article class="crm360__card"><div class="crm360__card-head"><div><div class="crm360__card-title">' + escapeHTML(values.opportunity_number || values.title || '') + '</div><div class="crm360__meta">' + escapeHTML(values.title || '') + '</div></div><span class="crm360__pill">' + escapeHTML(values.stage || '-') + '</span></div><div class="crm360__meta">' + formatCurrency(values.estimated_value) + ' · ' + escapeHTML(values.owner_user_id || '-') + '</div></article>';
              }).join('') + '</div>' : '<div class="crm360__empty">' + text('No opportunities for this customer.', 'Tidak ada peluang untuk pelanggan ini.') + '</div>') + '</div></section>'
          +   '<section class="crm360__panel"><div class="crm360__panel-head"><h3>' + text('Recent Activities', 'Aktivitas Terbaru') + '</h3><p>' + text('CRM follow-up activity across service and sales.', 'Aktivitas tindak lanjut CRM lintas layanan dan penjualan.') + '</p></div><div class="crm360__panel-body">' + (activities.length ? '<div class="crm360__list">' + activities.slice(0, 8).map(function(item) {
                const values = item.values || {};
                return '<article class="crm360__card"><div class="crm360__card-head"><div><div class="crm360__card-title">' + escapeHTML(values.subject || values.activity_number || '') + '</div><div class="crm360__meta">' + escapeHTML(values.activity_type || '-') + '</div></div><span class="crm360__pill">' + escapeHTML(values.status || '-') + '</span></div><div class="crm360__meta">' + escapeHTML(values.owner_user_id || '-') + ' · ' + escapeHTML(values.completed_at || values.due_at || '-') + '</div></article>';
              }).join('') + '</div>' : '<div class="crm360__empty">' + text('No activities for this customer.', 'Tidak ada aktivitas untuk pelanggan ini.') + '</div>') + '</div></section>'
          + '</section>';
      }
      function render() {
        ctx.mount.innerHTML = ''
          + '<section class="crm360">'
          +   '<section class="crm360__panel"><div class="crm360__panel-head"><h3>' + title + '</h3><p>' + text('Search or choose an at-risk customer to inspect service and sales context.', 'Cari atau pilih pelanggan berisiko untuk meninjau konteks layanan dan penjualan.') + '</p></div><div class="crm360__panel-body">'
          +     '<div class="crm360__search">'
          +       '<label><span>' + text('Customer Party ID', 'ID Pihak Pelanggan') + '</span><input id="crm360-party-id" value="' + escapeHTML(state.partyID || '') + '" placeholder="party_..." /></label>'
          +       '<button type="button" class="crm360__button" id="crm360-load">' + text('Load Customer', 'Muat Pelanggan') + '</button>'
          +       '<button type="button" class="crm360__button crm360__button--soft" id="crm360-reset">' + text('Clear', 'Reset') + '</button>'
          +     '</div>'
          +   '</div></section>'
          +   '<section class="crm360__panel"><div class="crm360__panel-head"><h3>' + text('At-Risk Customers', 'Pelanggan Berisiko') + '</h3><p>' + text('Customers with open service issues and active pipeline exposure.', 'Pelanggan dengan isu layanan terbuka dan eksposur pipeline aktif.') + '</p></div><div class="crm360__panel-body">' + renderAtRiskCustomers() + '</div></section>'
          +   renderSummary()
          + '</section>';
        const loadButton = ctx.mount.querySelector('#crm360-load');
        if (loadButton) {
          loadButton.addEventListener('click', function() {
            const input = ctx.mount.querySelector('#crm360-party-id');
            navigate(input ? input.value.trim() : '');
          });
        }
        const resetButton = ctx.mount.querySelector('#crm360-reset');
        if (resetButton) {
          resetButton.addEventListener('click', function() { navigate(''); });
        }
        ctx.mount.querySelectorAll('[data-party-id]').forEach(function(node) {
          node.addEventListener('click', function() {
            navigate(node.getAttribute('data-party-id') || '');
          });
        });
      }
      await loadHealth();
      if (state.partyID) {
        await loadSummary(state.partyID);
      }
      render();
    }
  };
})();`
}
