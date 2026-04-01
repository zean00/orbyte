package httpx

func LeaveWorkspaceBundle() string {
	return `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  function text(ctx, en, id) { return ctx.locale === 'id' ? id : en; }
  function escapeHTML(value) {
    return String(value == null ? '' : value).replace(/[&<>"]/g, function(char) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[char];
    });
  }
  function csrfToken() {
    const hit = document.cookie.split('; ').find(function(item) { return item.indexOf('orbyte_csrf=') === 0; });
    return hit ? decodeURIComponent(hit.split('=').slice(1).join('=')) : '';
  }
  async function apiJSON(url, options) {
    const response = await fetch(url, Object.assign({ credentials: 'include' }, options || {}));
    const payload = await response.json().catch(function() { return {}; });
    if (!response.ok) {
      const message = payload && payload.error && payload.error.message ? payload.error.message : (response.status + ' ' + response.statusText);
      throw new Error(message);
    }
    return payload;
  }
  async function postJSON(url, body, method) {
    return apiJSON(url, {
      method: method || 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken()
      },
      body: JSON.stringify(body || {})
    });
  }
  function ensureStyles() {
    if (document.getElementById('leave-workspace-styles')) return;
    const style = document.createElement('style');
    style.id = 'leave-workspace-styles';
    style.textContent = ''
      + '.leave-workspace{display:grid;gap:1rem;}'
      + '.leave-grid{display:grid;grid-template-columns:minmax(18rem,28rem) minmax(0,1fr);gap:1rem;}'
      + '.leave-panel{border:1px solid var(--color-line);border-radius:1rem;background:var(--color-surface);box-shadow:var(--shadow-panel);padding:1rem;}'
      + '.leave-header{display:flex;justify-content:space-between;gap:1rem;align-items:flex-start;flex-wrap:wrap;}'
      + '.leave-title{margin:0;font-size:1.4rem;color:var(--color-body);}'
      + '.leave-subtitle{margin:0.35rem 0 0;color:var(--color-muted);}'
      + '.leave-actions,.leave-tabs,.leave-row-actions{display:flex;gap:0.5rem;flex-wrap:wrap;}'
      + '.leave-button{appearance:none;border:1px solid var(--color-line);border-radius:0.8rem;background:var(--color-surface);color:var(--color-body);padding:0.7rem 0.9rem;font:inherit;font-weight:700;cursor:pointer;}'
      + '.leave-button--primary{background:var(--color-accent);border-color:var(--color-accent);color:#fff;}'
      + '.leave-button--danger{border-color:#d59;color:#8a2433;background:#fff6f8;}'
      + '.leave-tab.is-active{background:var(--color-accent);border-color:var(--color-accent);color:#fff;}'
      + '.leave-cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(12rem,1fr));gap:0.75rem;}'
      + '.leave-card{border:1px solid var(--color-line);border-radius:0.9rem;padding:0.9rem;background:color-mix(in srgb,var(--color-shell) 30%,var(--color-surface));}'
      + '.leave-card span{display:block;color:var(--color-muted);font-size:0.75rem;font-weight:800;letter-spacing:0.08em;text-transform:uppercase;}'
      + '.leave-card strong{display:block;margin-top:0.45rem;color:var(--color-body);font-size:1.2rem;}'
      + '.leave-list{display:grid;gap:0.6rem;}'
      + '.leave-row{border:1px solid var(--color-line);border-radius:0.9rem;padding:0.8rem;background:var(--color-surface);cursor:pointer;}'
      + '.leave-row.is-selected{border-color:var(--color-accent);box-shadow:0 0 0 1px color-mix(in srgb,var(--color-accent) 55%, transparent);}'
      + '.leave-row h4{margin:0;color:var(--color-body);font-size:1rem;}'
      + '.leave-meta{display:flex;gap:0.5rem;flex-wrap:wrap;margin-top:0.45rem;color:var(--color-muted);font-size:0.9rem;}'
      + '.leave-pill{display:inline-flex;align-items:center;padding:0.24rem 0.6rem;border-radius:999px;border:1px solid var(--color-line);font-size:0.74rem;font-weight:700;}'
      + '.leave-pill--approved{background:#eefaf1;color:#176936;border-color:#9ad4a6;}'
      + '.leave-pill--submitted{background:#fff7e8;color:#8a6112;border-color:#e7cf98;}'
      + '.leave-pill--rejected,.leave-pill--cancelled{background:#fff4f3;color:#8a2433;border-color:#e4b3b6;}'
      + '.leave-pill--draft{background:#eef3fb;color:#2f4f85;border-color:#bed0ec;}'
      + '.leave-form{display:grid;gap:0.75rem;}'
      + '.leave-field{display:grid;gap:0.35rem;}'
      + '.leave-field span{font-size:0.75rem;font-weight:800;letter-spacing:0.08em;text-transform:uppercase;color:var(--color-muted);}'
      + '.leave-field input,.leave-field select,.leave-field textarea{border:1px solid var(--color-line);border-radius:0.8rem;background:var(--color-surface);color:var(--color-body);padding:0.75rem;font:inherit;}'
      + '.leave-form-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(10rem,1fr));gap:0.75rem;}'
      + '.leave-kv{display:grid;grid-template-columns:repeat(auto-fit,minmax(12rem,1fr));gap:0.75rem;}'
      + '.leave-kv article{border:1px solid var(--color-line);border-radius:0.9rem;padding:0.8rem;background:color-mix(in srgb,var(--color-shell) 28%,var(--color-surface));}'
      + '.leave-kv span{display:block;color:var(--color-muted);font-size:0.74rem;font-weight:800;letter-spacing:0.08em;text-transform:uppercase;}'
      + '.leave-kv strong{display:block;margin-top:0.45rem;color:var(--color-body);}'
      + '.leave-table-wrap{overflow:auto;border:1px solid var(--color-line);border-radius:0.9rem;}'
      + '.leave-table{width:100%;border-collapse:collapse;min-width:38rem;}'
      + '.leave-table th,.leave-table td{padding:0.75rem;border-top:1px solid var(--color-line);text-align:left;vertical-align:top;}'
      + '.leave-table th{font-size:0.74rem;font-weight:800;letter-spacing:0.08em;text-transform:uppercase;color:var(--color-muted);}'
      + '.leave-empty{color:var(--color-muted);padding:1rem 0;}'
      + '@media (max-width: 980px){.leave-grid{grid-template-columns:1fr;}}';
    document.head.appendChild(style);
  }

  window.ClinicModuleBundles['leave-self-service-workspace'] = { render: renderWorkspace };
  window.ClinicModuleBundles['attendance-leave-workspace'] = { render: renderWorkspace };

  async function renderWorkspace(ctx) {
    ensureStyles();
    const mount = ctx.mount;
    mount.innerHTML = '';
    const path = window.location.pathname || '';
    const state = {
      selfService: !path.includes('/attendance/leave-approvals'),
      tab: 'actionable',
      requests: [],
      balances: [],
      entries: [],
      selectedRequestID: '',
      selectedRequest: null,
      selectedAccountID: '',
      draftRequestID: '',
      loading: false
    };

    function requestPill(status) {
      const value = String(status || '').toLowerCase();
      const cls = value === 'approved' ? 'leave-pill--approved'
        : value === 'submitted' ? 'leave-pill--submitted'
        : value === 'draft' ? 'leave-pill--draft'
        : 'leave-pill--rejected';
      return '<span class="leave-pill ' + cls + '">' + escapeHTML(status || '') + '</span>';
    }

    async function loadSelfService() {
      const balances = await apiJSON('/ui/self-service/leave/balances');
      const requests = await apiJSON('/ui/self-service/leave/requests');
      state.balances = balances.items || [];
      state.requests = requests.items || [];
      if (!state.selectedRequestID && state.requests.length) {
        state.selectedRequestID = state.requests[0].id || '';
      }
      if (!state.selectedAccountID && state.balances.length) {
        state.selectedAccountID = state.balances[0].id || '';
      }
      if (state.selectedRequestID) {
        const detail = await apiJSON('/ui/self-service/leave/requests/' + encodeURIComponent(state.selectedRequestID));
        state.selectedRequest = detail.record || null;
        state.draftRequestID = state.selectedRequest && String(state.selectedRequest.approval_status || '').toLowerCase() === 'draft'
          ? (state.selectedRequest.id || '')
          : '';
      } else {
        state.selectedRequest = null;
        state.draftRequestID = '';
      }
      if (state.selectedAccountID) {
        const entryPayload = await apiJSON('/ui/self-service/leave/balances/' + encodeURIComponent(state.selectedAccountID) + '/entries');
        state.entries = entryPayload.items || [];
      }
    }

    async function loadInbox() {
      const inbox = await apiJSON('/ui/attendance/leave-requests/inbox?bucket=' + encodeURIComponent(state.tab));
      state.requests = inbox.items || [];
      if (!state.selectedRequestID && state.requests.length) {
        state.selectedRequestID = state.requests[0].id || '';
      }
      if (state.selectedRequestID) {
        const detail = await apiJSON('/ui/attendance/leave-requests/' + encodeURIComponent(state.selectedRequestID));
        state.selectedRequest = detail.record || null;
      } else {
        state.selectedRequest = null;
      }
    }

    async function refresh() {
      state.loading = true;
      render();
      try {
        if (state.selfService) {
          await loadSelfService();
        } else {
          await loadInbox();
        }
      } catch (err) {
        alert(err instanceof Error ? err.message : 'Leave workspace failed to load');
      } finally {
        state.loading = false;
        render();
      }
    }

    async function submitSelfService() {
      const requestID = state.draftRequestID;
      if (!requestID) return;
      await postJSON('/ui/self-service/leave/requests/' + encodeURIComponent(requestID) + '/submit', {});
      state.selectedRequestID = requestID;
      state.draftRequestID = '';
      await refresh();
    }

    async function saveDraft(form) {
      const payload = {
        leave_policy_id: form.querySelector('[name="leave_policy_id"]').value,
        start_date: form.querySelector('[name="start_date"]').value,
        end_date: form.querySelector('[name="end_date"]').value,
        request_unit: form.querySelector('[name="request_unit"]').value,
        half_day_session: form.querySelector('[name="half_day_session"]').value,
        notes: form.querySelector('[name="notes"]').value
      };
      if (state.draftRequestID) {
        await postJSON('/ui/self-service/leave/requests/' + encodeURIComponent(state.draftRequestID), payload, 'PUT');
      } else {
        const response = await postJSON('/ui/self-service/leave/requests', payload);
        state.draftRequestID = response.record && response.record.id ? response.record.id : '';
        state.selectedRequestID = state.draftRequestID;
      }
      await refresh();
    }

    async function act(url, body) {
      await postJSON(url, body || {});
      await refresh();
    }

    function employeeForm() {
      const selected = state.selectedRequest || {};
      return ''
        + '<form class="leave-form" data-leave-draft-form>'
        + '<div class="leave-form-grid">'
        + field(text(ctx, 'Leave Policy ID', 'ID Kebijakan Cuti'), 'leave_policy_id', selected.leave_policy_id || '')
        + field(text(ctx, 'Start Date', 'Tanggal Mulai'), 'start_date', selected.start_date || '', 'date')
        + field(text(ctx, 'End Date', 'Tanggal Selesai'), 'end_date', selected.end_date || '', 'date')
        + '</div>'
        + '<div class="leave-form-grid">'
        + selectField(text(ctx, 'Request Unit', 'Unit Permintaan'), 'request_unit', selected.request_unit || 'day', [{value:'day',label:text(ctx,'Day','Hari')},{value:'half_day',label:text(ctx,'Half Day','Setengah Hari')}])
        + selectField(text(ctx, 'Half Day Session', 'Sesi Setengah Hari'), 'half_day_session', selected.half_day_session || '', [{value:'',label:text(ctx,'None','Tidak Ada')},{value:'morning',label:text(ctx,'Morning','Pagi')},{value:'afternoon',label:text(ctx,'Afternoon','Sore')}])
        + '</div>'
        + '<label class="leave-field"><span>' + escapeHTML(text(ctx, 'Notes', 'Catatan')) + '</span><textarea name="notes" rows="3">' + escapeHTML(selected.notes || '') + '</textarea></label>'
        + '<div class="leave-actions">'
        + '<button type="button" class="leave-button" data-save-draft>' + escapeHTML(text(ctx, 'Save Draft', 'Simpan Draft')) + '</button>'
        + (state.draftRequestID ? '<button type="button" class="leave-button leave-button--primary" data-submit-draft>' + escapeHTML(text(ctx, 'Submit', 'Ajukan')) + '</button>' : '')
        + '</div>'
        + '</form>';
    }

    function field(label, name, value, type) {
      return '<label class="leave-field"><span>' + escapeHTML(label) + '</span><input name="' + escapeHTML(name) + '" type="' + escapeHTML(type || 'text') + '" value="' + escapeHTML(value || '') + '"/></label>';
    }
    function selectField(label, name, value, options) {
      return '<label class="leave-field"><span>' + escapeHTML(label) + '</span><select name="' + escapeHTML(name) + '">' + options.map(function(option) {
        return '<option value="' + escapeHTML(option.value) + '"' + (String(option.value) === String(value) ? ' selected' : '') + '>' + escapeHTML(option.label) + '</option>';
      }).join('') + '</select></label>';
    }

    function render() {
      const selected = state.selectedRequest;
      const balancesHTML = state.selfService ? '<section class="leave-panel"><div class="leave-header"><div><h3 class="leave-title">' + escapeHTML(text(ctx, 'Balance Accounts', 'Akun Saldo')) + '</h3><p class="leave-subtitle">' + escapeHTML(text(ctx, 'Available, reserved, carry-forward, and expiry.', 'Tersedia, reservasi, carry-forward, dan kedaluwarsa.')) + '</p></div></div><div class="leave-cards">' + state.balances.map(function(item) {
        return '<article class="leave-card"><span>' + escapeHTML(item.leave_policy_name || item.leave_policy_id || '') + '</span><strong>' + escapeHTML(String(item.available_days || 0)) + '</strong><div class="leave-meta"><span>' + escapeHTML(text(ctx, 'Reserved', 'Reservasi') + ': ' + (item.reserved_days || 0)) + '</span><span>' + escapeHTML(text(ctx, 'Carry Forward', 'Carry Forward') + ': ' + (item.carry_forward_balance_days || 0)) + '</span><span>' + escapeHTML(text(ctx, 'Expiry', 'Kedaluwarsa') + ': ' + (item.carry_forward_expiry_date || '-')) + '</span><button class="leave-button" data-account-select="' + escapeHTML(item.id) + '">' + escapeHTML(text(ctx, 'View Entries', 'Lihat Entri')) + '</button></div></article>';
      }).join('') + '</div></section>' : '';

      const requestRows = state.requests.map(function(item) {
        const title = state.selfService ? (item.leave_policy_name || item.leave_policy_id || item.id) : ((item.employee_code || item.employee_id || '') + ' • ' + (item.leave_policy_name || item.leave_policy_id || ''));
        return '<article class="leave-row' + (item.id === state.selectedRequestID ? ' is-selected' : '') + '" data-request-select="' + escapeHTML(item.id) + '"><div class="leave-header"><h4>' + escapeHTML(title) + '</h4>' + requestPill(item.approval_status) + '</div><div class="leave-meta"><span>' + escapeHTML((item.start_date || '') + ' → ' + (item.end_date || '')) + '</span><span>' + escapeHTML(text(ctx, 'Days', 'Hari') + ': ' + (item.requested_days || 0)) + '</span><span>' + escapeHTML(item.stage_progress_label || '') + '</span></div></article>';
      }).join('');

      const detailHTML = selected ? ''
        + '<section class="leave-panel">'
        + '<div class="leave-header"><div><h3 class="leave-title">' + escapeHTML(selected.leave_policy_name || selected.leave_policy_id || selected.id) + '</h3><p class="leave-subtitle">' + escapeHTML(selected.status_label || '') + '</p></div><div class="leave-row-actions">'
        + (selected.allowed_actions || []).map(function(action) {
          const danger = action === 'reject' || action === 'cancel';
          return '<button class="leave-button' + (action === 'approve' || action === 'submit' ? ' leave-button--primary' : '') + (danger ? ' leave-button--danger' : '') + '" data-request-action="' + escapeHTML(action) + '">' + escapeHTML(action.replace(/_/g, ' ')) + '</button>';
        }).join('')
        + '</div></div>'
        + '<div class="leave-kv">'
        + kv(text(ctx, 'Employee', 'Karyawan'), selected.employee_code || selected.employee_id || '-')
        + kv(text(ctx, 'Dates', 'Tanggal'), (selected.start_date || '') + ' → ' + (selected.end_date || ''))
        + kv(text(ctx, 'Requested Days', 'Hari Diminta'), selected.requested_days || 0)
        + kv(text(ctx, 'Stage', 'Tahap'), selected.stage_progress_label || '-')
        + kv(text(ctx, 'Approvals', 'Persetujuan'), (selected.recorded_approver_count || 0) + ' / ' + (selected.required_approver_count || 0))
        + kv(text(ctx, 'Notes', 'Catatan'), selected.notes || '-')
        + '</div>'
        + (selected.balance_snapshot ? '<div class="leave-kv" style="margin-top:0.75rem;">'
            + kv(text(ctx, 'Available', 'Tersedia'), selected.balance_snapshot.available_days || 0)
            + kv(text(ctx, 'Reserved', 'Reservasi'), selected.balance_snapshot.reserved_days || 0)
            + kv(text(ctx, 'Carry Forward', 'Carry Forward'), selected.balance_snapshot.carry_forward_balance_days || 0)
            + kv(text(ctx, 'Expiry', 'Kedaluwarsa'), selected.balance_snapshot.carry_forward_expiry_date || '-')
            + '</div>' : '')
        + '</section>' : '<section class="leave-panel"><p class="leave-empty">' + escapeHTML(text(ctx, 'No request selected.', 'Belum ada permintaan dipilih.')) + '</p></section>';

      const entriesHTML = state.selfService ? '<section class="leave-panel"><div class="leave-header"><div><h3 class="leave-title">' + escapeHTML(text(ctx, 'Balance Entries', 'Entri Saldo')) + '</h3></div></div><div class="leave-table-wrap"><table class="leave-table"><thead><tr><th>' + escapeHTML(text(ctx, 'Type', 'Tipe')) + '</th><th>' + escapeHTML(text(ctx, 'Days', 'Hari')) + '</th><th>' + escapeHTML(text(ctx, 'Carry Forward', 'Carry Forward')) + '</th><th>' + escapeHTML(text(ctx, 'Date', 'Tanggal')) + '</th></tr></thead><tbody>' + state.entries.map(function(item) {
        return '<tr><td>' + escapeHTML(item.entry_type || '') + '</td><td>' + escapeHTML(String(item.days || 0)) + '</td><td>' + escapeHTML(String(item.carry_forward_days_delta || 0)) + '</td><td>' + escapeHTML(item.effective_date || '') + '</td></tr>';
      }).join('') + '</tbody></table></div></section>' : '';

      mount.innerHTML = ''
        + '<div class="leave-workspace">'
        + '<section class="leave-panel">'
        + '<div class="leave-header"><div><h2 class="leave-title">' + escapeHTML(state.selfService ? text(ctx, 'My Leave', 'Cuti Saya') : text(ctx, 'Leave Approvals', 'Persetujuan Cuti')) + '</h2><p class="leave-subtitle">' + escapeHTML(state.selfService ? text(ctx, 'Track requests, balances, and approval progress.', 'Pantau permintaan, saldo, dan progres persetujuan.') : text(ctx, 'Review actionable and approved leave requests.', 'Tinjau permintaan cuti yang dapat ditindaklanjuti dan yang sudah disetujui.')) + '</p></div><div class="leave-actions">' + (state.selfService ? '<button class="leave-button" data-new-request>' + escapeHTML(text(ctx, 'New Request', 'Permintaan Baru')) + '</button>' : '<div class="leave-tabs"><button class="leave-button leave-tab' + (state.tab === 'actionable' ? ' is-active' : '') + '" data-inbox-tab="actionable">' + escapeHTML(text(ctx, 'Actionable', 'Perlu Tindakan')) + '</button><button class="leave-button leave-tab' + (state.tab === 'approved' ? ' is-active' : '') + '" data-inbox-tab="approved">' + escapeHTML(text(ctx, 'Approved', 'Disetujui')) + '</button></div>') + '<button class="leave-button" data-refresh>' + escapeHTML(text(ctx, 'Refresh', 'Muat Ulang')) + '</button></div></div>'
        + '</section>'
        + (state.selfService ? employeeForm() : '')
        + balancesHTML
        + '<div class="leave-grid"><section class="leave-panel"><div class="leave-list">' + (requestRows || '<p class="leave-empty">' + escapeHTML(text(ctx, 'No leave requests.', 'Belum ada permintaan cuti.')) + '</p>') + '</div></section>' + detailHTML + '</div>'
        + entriesHTML
        + '</div>';

      bindEvents();
    }

    function kv(label, value) {
      return '<article><span>' + escapeHTML(label) + '</span><strong>' + escapeHTML(String(value == null ? '' : value)) + '</strong></article>';
    }

    function bindEvents() {
      mount.querySelectorAll('[data-refresh]').forEach(function(node) { node.onclick = function() { refresh(); }; });
      mount.querySelectorAll('[data-inbox-tab]').forEach(function(node) {
        node.onclick = function() { state.tab = node.getAttribute('data-inbox-tab') || 'actionable'; state.selectedRequestID = ''; refresh(); };
      });
      mount.querySelectorAll('[data-request-select]').forEach(function(node) {
        node.onclick = async function() {
          state.selectedRequestID = node.getAttribute('data-request-select') || '';
          if (state.selfService) {
            const payload = await apiJSON('/ui/self-service/leave/requests/' + encodeURIComponent(state.selectedRequestID));
            state.selectedRequest = payload.record || null;
          } else {
            const payload = await apiJSON('/ui/attendance/leave-requests/' + encodeURIComponent(state.selectedRequestID));
            state.selectedRequest = payload.record || null;
          }
          render();
        };
      });
      mount.querySelectorAll('[data-account-select]').forEach(function(node) {
        node.onclick = async function() {
          state.selectedAccountID = node.getAttribute('data-account-select') || '';
          const payload = await apiJSON('/ui/self-service/leave/balances/' + encodeURIComponent(state.selectedAccountID) + '/entries');
          state.entries = payload.items || [];
          render();
        };
      });
      const form = mount.querySelector('[data-leave-draft-form]');
      if (form) {
        const save = mount.querySelector('[data-save-draft]');
        if (save) save.onclick = function() { saveDraft(form); };
        const submit = mount.querySelector('[data-submit-draft]');
        if (submit) submit.onclick = function() { submitSelfService(); };
        const create = mount.querySelector('[data-new-request]');
        if (create) create.onclick = function() {
          state.selectedRequest = { request_unit: 'day', half_day_session: '', notes: '' };
          state.draftRequestID = '';
          render();
        };
      }
      mount.querySelectorAll('[data-request-action]').forEach(function(node) {
        node.onclick = async function() {
          const action = node.getAttribute('data-request-action') || '';
          if (!state.selectedRequestID) return;
          if (state.selfService) {
            if (action === 'cancel') await act('/ui/self-service/leave/requests/' + encodeURIComponent(state.selectedRequestID) + '/cancel');
            if (action === 'submit') await act('/ui/self-service/leave/requests/' + encodeURIComponent(state.selectedRequestID) + '/submit');
            return;
          }
          if (action === 'approve') await act('/ui/attendance/leave-requests/' + encodeURIComponent(state.selectedRequestID) + '/approve');
          if (action === 'reject') await act('/ui/attendance/leave-requests/' + encodeURIComponent(state.selectedRequestID) + '/reject', { note: window.prompt('Reject note') || '' });
          if (action === 'cancel') await act('/ui/attendance/leave-requests/' + encodeURIComponent(state.selectedRequestID) + '/cancel', { note: window.prompt('Cancel note') || '' });
        };
      });
    }

    await refresh();
  }
})();`
}
