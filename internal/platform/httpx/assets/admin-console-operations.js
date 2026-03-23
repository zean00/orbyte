// Module, config, org chart, and workflow operations surfaces.
    function renderModules(items) {
      const enabledCount = items.filter((item) => item.installed && item.installed.enabled).length;
      const disabledCount = items.length - enabledCount;
      const blockedCount = items.filter((item) => (item.dependency_diagnostics || []).some((dep) => !dep.compatible)).length;
      document.getElementById('modules').innerHTML = '<div class="metric-grid module-summary-grid">'
        + '<article class="metric-card"><span class="meta">Installed</span><strong>' + items.length + '</strong></article>'
        + '<article class="metric-card"><span class="meta">Enabled</span><strong>' + enabledCount + '</strong></article>'
        + '<article class="metric-card"><span class="meta">Disabled</span><strong>' + disabledCount + '</strong></article>'
        + '<article class="metric-card"><span class="meta">Needs Attention</span><strong>' + blockedCount + '</strong></article>'
        + '</div><div class="table-shell"><table class="data-table"><thead><tr><th>' + t('module_col') + '</th><th>' + t('status_col') + '</th><th>' + t('deps_col') + '</th><th></th></tr></thead><tbody>' + items.map(item => {
        const enabled = item.installed.enabled;
        const deps = (item.dependency_diagnostics || []).map(dep => dep.module_key + ':' + (dep.compatible ? 'ok' : dep.reason || 'blocked')).join(', ');
        return '<tr><td><div class="row-primary">' + pickText(item.manifest, 'name') + '</div><div class="row-secondary">' + item.manifest.key + ' · ' + item.manifest.version + '</div></td><td><span class="pill ' + (enabled ? '' : 'off') + '">' + (enabled ? t('enabled') : t('disabled')) + '</span></td><td class="row-secondary">' + (deps || t('none')) + '</td><td><button data-key="' + item.manifest.key + '" data-action="' + (enabled ? 'disable' : 'enable') + '" class="' + (enabled ? 'warn' : '') + '">' + (enabled ? t('disable') : t('enable')) + '</button></td></tr>';
      }).join('') + '</tbody></table></div>';
      enhanceAdminControlAccessibility(document.getElementById('modules'));
      document.querySelectorAll('#modules button[data-key]').forEach(btn => {
        btn.addEventListener('click', async () => {
          const csrf = getCookie('orbyte_csrf');
          await getJSON('/admin/api/modules/' + btn.dataset.key + '/actions/' + btn.dataset.action, {method:'POST', headers:{'X-CSRF-Token': csrf}});
          boot();
        });
      });
    }
    function renderDefinitions(items) {
      document.getElementById('definitions').innerHTML = items.map((item) => {
        const fields = (item.fields || []).map((field) => '<li><strong>' + pickText(field, 'label') + '</strong> <span class="muted">(' + field.key + ' · ' + field.type + ')</span></li>').join('');
        return '<article class="card"><h3>' + pickText(item, 'display_name') + '</h3><p class="muted">' + item.key + ' · ' + item.module_key + '</p>' +
          (pickText(item, 'description') ? '<p class="status">' + pickText(item, 'description') + '</p>' : '') +
          '<p><strong>' + t('default_value') + ':</strong></p><pre>' + escapeHTML(JSON.stringify(item.default_value || {}, null, 2)) + '</pre>' +
          '<p><strong>' + t('fields_label') + ':</strong></p><ul>' + fields + '</ul></article>';
      }).join('');
      document.getElementById('config-key').innerHTML = items.map(item => '<option value="' + item.key + '">' + pickText(item, 'display_name') + ' (' + item.key + ')</option>').join('');
      if (items[0]) {
        document.getElementById('config-value').value = JSON.stringify(items[0].default_value, null, 2);
      }
      document.getElementById('config-key').onchange = () => {
        const current = items.find(item => item.key === document.getElementById('config-key').value);
        if (current) document.getElementById('config-value').value = JSON.stringify(current.default_value, null, 2);
      };
      document.getElementById('load-effective').onclick = async () => {
        const key = document.getElementById('config-key').value;
        const orgID = document.getElementById('organization-id').value;
        const locationID = document.getElementById('location-id').value;
        const payload = await getJSON('/admin/api/config/effective?organization_id=' + encodeURIComponent(orgID) + '&location_id=' + encodeURIComponent(locationID));
        const match = payload.items.find(item => item.key === key);
          if (match) {
            document.getElementById('config-value').value = JSON.stringify(match.value, null, 2);
            document.getElementById('config-status').textContent = t('loaded_effective') + ' ' + match.source_scope + (match.source_scope_id ? ':' + match.source_scope_id : '');
          }
        };
      document.getElementById('save-config').onclick = async () => {
        const key = document.getElementById('config-key').value;
        const scope = document.getElementById('config-scope').value;
        const scopeID = scope === 'deployment' ? '' : (scope === 'organization' ? document.getElementById('organization-id').value : document.getElementById('location-id').value);
        const value = JSON.parse(document.getElementById('config-value').value || '{}');
        const csrf = getCookie('orbyte_csrf');
        await getJSON('/admin/api/config/entries/' + key + '/value', {
          method:'PUT',
          headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
          body: JSON.stringify({scope: scope, scope_id: scopeID, value: value})
        });
        document.getElementById('config-status').textContent = t('saved_config') + ' ' + key + ' ' + t('scope') + ' ' + scope + (scopeID ? ':' + scopeID : '');
      };
    }
    function populateUserSelect(id, allowBlank) {
      const select = document.getElementById(id);
      if (!select) return;
      const users = (adminState.users || []).slice().sort((left, right) => {
        const name = (left.username || '').localeCompare(right.username || '');
        return name || (left.id || '').localeCompare(right.id || '');
      });
      select.innerHTML = (allowBlank ? '<option value="">' + t('default_option') + '</option>' : '') + users.map((user) => '<option value="' + escapeHTML(user.id) + '">' + escapeHTML(user.username + ' (' + user.id + ')') + '</option>').join('');
    }
    function populateLocationSelect(id, allowBlank) {
      const select = document.getElementById(id);
      if (!select) return;
      const locations = (adminState.bootstrap && adminState.bootstrap.locations) || [];
      select.innerHTML = (allowBlank ? '<option value="">' + t('default_option') + '</option>' : '') + locations.map((loc) => '<option value="' + escapeHTML(loc.id) + '">' + escapeHTML(loc.name + ' (' + loc.id + ')') + '</option>').join('');
    }
    function toDateTimeLocalValue(value) {
      if (!value) return '';
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) return '';
      const offset = date.getTimezoneOffset();
      const local = new Date(date.getTime() - offset * 60000);
      return local.toISOString().slice(0, 16);
    }
    function fromDateTimeLocalValue(value) {
      return value ? new Date(value).toISOString() : '';
    }
    function hierarchyFilters() {
      return {
        organization_id: (adminState.bootstrap && adminState.bootstrap.organization && adminState.bootstrap.organization.id) || '',
        location_id: document.getElementById('org-location-filter') ? document.getElementById('org-location-filter').value : '',
        status: document.getElementById('org-status-filter') ? document.getElementById('org-status-filter').value : ''
      };
    }
    function activeHierarchyEdge(subjectUserID) {
      const edges = ((adminState.hierarchyGraph && adminState.hierarchyGraph.edges) || []).filter((item) => item.subject_user_id === subjectUserID && item.status === 'active');
      edges.sort((left, right) => {
        const rank = (item) => item.relationship_type === 'acting_manager' ? 0 : 1;
        if (rank(left) === rank(right)) return (right.priority || 0) - (left.priority || 0);
        return rank(left) - rank(right);
      });
      return edges[0] || null;
    }
    function hierarchyNode(userID) {
      return ((adminState.hierarchyGraph && adminState.hierarchyGraph.nodes) || []).find((item) => item.id === userID) || null;
    }
    function renderHierarchySummary() {
      const summary = (adminState.hierarchyGraph && adminState.hierarchyGraph.summary) || {};
      const container = document.getElementById('org-summary');
      if (!container) return;
      const cards = [
        {label: 'Users', value: summary.total_users || 0},
        {label: 'Active Lines', value: summary.active_lines || 0},
        {label: 'Without Manager', value: summary.orphan_users || 0},
        {label: 'Acting Overrides', value: summary.acting_overrides || 0}
      ];
      container.innerHTML = cards.map((item) => '<article class="metric-card"><span class="meta">' + escapeHTML(item.label) + '</span><strong>' + escapeHTML(item.value) + '</strong></article>').join('');
    }
    function layoutHierarchyGraph() {
      const nodes = (adminState.hierarchyGraph && adminState.hierarchyGraph.nodes) || [];
      const focusUserID = document.getElementById('org-focus-user') ? document.getElementById('org-focus-user').value : '';
      const parentByChild = {};
      const childrenByParent = {};
      nodes.forEach((node) => { childrenByParent[node.id] = []; });
      nodes.forEach((node) => {
        const edge = activeHierarchyEdge(node.id);
        if (edge) {
          parentByChild[node.id] = edge.manager_user_id;
          childrenByParent[edge.manager_user_id] = childrenByParent[edge.manager_user_id] || [];
          childrenByParent[edge.manager_user_id].push(node.id);
        }
      });
      Object.keys(childrenByParent).forEach((key) => childrenByParent[key].sort((left, right) => {
        const leftNode = hierarchyNode(left) || {username: left};
        const rightNode = hierarchyNode(right) || {username: right};
        return (leftNode.username || '').localeCompare(rightNode.username || '') || left.localeCompare(right);
      }));
      let roots = nodes.filter((node) => !parentByChild[node.id]).map((node) => node.id);
      if (!roots.length && nodes[0]) roots = [nodes[0].id];
      if (focusUserID) {
        const path = [];
        const seen = new Set();
        let current = focusUserID;
        while (current && !seen.has(current)) {
          seen.add(current);
          path.unshift(current);
          current = parentByChild[current];
        }
        const subtree = [];
        const queue = [focusUserID];
        const visited = new Set();
        while (queue.length) {
          const id = queue.shift();
          if (!id || visited.has(id)) continue;
          visited.add(id);
          subtree.push(id);
          (childrenByParent[id] || []).forEach((child) => queue.push(child));
        }
        roots = path.length ? [path[0]] : roots;
        const allowed = new Set(path.concat(subtree));
        const filteredNodes = nodes.filter((node) => allowed.has(node.id));
        return hierarchyLayoutFromNodes(filteredNodes, childrenByParent, roots, parentByChild);
      }
      return hierarchyLayoutFromNodes(nodes, childrenByParent, roots, parentByChild);
    }
    function hierarchyLayoutFromNodes(nodes, childrenByParent, roots, parentByChild) {
      const positions = {};
      const levelIndex = {};
      const visible = new Set(nodes.map((node) => node.id));
      function place(nodeID, depth) {
        if (!visible.has(nodeID) || positions[nodeID]) return;
        const siblings = childrenByParent[nodeID] || [];
        const index = levelIndex[depth] || 0;
        positions[nodeID] = {x: depth * 240 + 24, y: index * 120 + 24};
        levelIndex[depth] = index + 1;
        siblings.forEach((childID) => place(childID, depth + 1));
      }
      roots.forEach((rootID) => place(rootID, 0));
      nodes.forEach((node) => {
        if (!positions[node.id]) {
          const parentID = parentByChild[node.id];
          const depth = parentID && positions[parentID] ? Math.round((positions[parentID].x - 24) / 240) + 1 : 0;
          place(node.id, depth);
        }
      });
      return positions;
    }
    function renderHierarchyChart() {
      const container = document.getElementById('org-chart');
      if (!container) return;
      const nodes = (adminState.hierarchyGraph && adminState.hierarchyGraph.nodes) || [];
      if (!nodes.length) {
        container.innerHTML = '<p class="status">-</p>';
        return;
      }
      const positions = layoutHierarchyGraph();
      const visibleNodes = nodes.filter((node) => positions[node.id]);
      const width = Math.max.apply(null, visibleNodes.map((node) => positions[node.id].x)) + 240;
      const height = Math.max.apply(null, visibleNodes.map((node) => positions[node.id].y)) + 140;
      const edges = ((adminState.hierarchyGraph && adminState.hierarchyGraph.edges) || []).filter((edge) => positions[edge.subject_user_id] && positions[edge.manager_user_id]);
      container.innerHTML = '<div class="org-chart-stage" style="width:' + width + 'px;height:' + height + 'px;">'
        + '<svg class="org-chart-lines" viewBox="0 0 ' + width + ' ' + height + '" preserveAspectRatio="xMinYMin meet">'
        + edges.map((edge) => {
          const from = positions[edge.manager_user_id];
          const to = positions[edge.subject_user_id];
          const lineClass = edge.relationship_type === 'acting_manager' ? 'acting' : 'primary';
          const path = 'M ' + (from.x + 180) + ' ' + (from.y + 34) + ' C ' + (from.x + 210) + ' ' + (from.y + 34) + ', ' + (to.x - 30) + ' ' + (to.y + 34) + ', ' + to.x + ' ' + (to.y + 34);
          return '<path class="' + lineClass + '" d="' + path + '"></path>';
        }).join('')
        + '</svg>'
        + visibleNodes.map((node) => {
          const position = positions[node.id];
          const selected = adminState.hierarchySelectedUserID === node.id ? ' selected' : '';
          const edge = activeHierarchyEdge(node.id);
          const badge = edge ? edge.relationship_type.replace('_', ' ') : 'root';
          return '<button type="button" class="org-node' + selected + '" data-org-user="' + escapeHTML(node.id) + '" style="left:' + position.x + 'px;top:' + position.y + 'px;">'
            + '<strong>' + escapeHTML(node.username) + '</strong>'
            + '<span class="meta">' + escapeHTML(node.id) + '</span>'
            + '<span class="pill' + (node.status !== 'active' ? ' off' : '') + '">' + escapeHTML(badge) + '</span>'
            + '</button>';
        }).join('')
        + '</div>';
      container.querySelectorAll('[data-org-user]').forEach((node) => {
        node.onclick = () => {
          adminState.hierarchySelectedUserID = node.getAttribute('data-org-user') || '';
          renderHierarchySelection();
          void loadHierarchyChain(adminState.hierarchySelectedUserID);
          renderHierarchyChart();
        };
      });
    }
    function renderHierarchySelection() {
      const user = hierarchyNode(adminState.hierarchySelectedUserID) || ((adminState.users || []).find((item) => item.id === adminState.hierarchySelectedUserID));
      const detail = document.getElementById('org-selected-user');
      const lines = document.getElementById('org-user-lines');
      if (detail) {
        if (!user) {
          detail.innerHTML = '<p class="status">Select a user to inspect the chain.</p>';
        } else {
          const edge = activeHierarchyEdge(user.id);
          detail.innerHTML = ''
            + '<article class="detail-item"><span class="meta">User</span><strong>' + escapeHTML(user.username || user.id) + '</strong></article>'
            + '<article class="detail-item"><span class="meta">ID</span><strong>' + escapeHTML(user.id) + '</strong></article>'
            + '<article class="detail-item"><span class="meta">Current Manager</span><strong>' + escapeHTML(edge ? edge.manager_user_id : 'Unassigned') + '</strong></article>';
        }
      }
      if (lines) {
        const items = (adminState.reportingLines || []).filter((item) => item.subject_user_id === adminState.hierarchySelectedUserID);
        lines.innerHTML = items.length ? items.map((item) => '<article class="card"><strong>' + escapeHTML(item.relationship_type) + '</strong><div class="row-secondary">' + escapeHTML(item.manager_user_id + ' · ' + (item.status || '')) + '</div><div class="actions"><button type="button" class="secondary" data-org-line="' + escapeHTML(item.id) + '">Edit</button></div></article>').join('') : '<p class="status">No reporting lines for this user.</p>';
        lines.querySelectorAll('[data-org-line]').forEach((node) => {
          node.onclick = () => resetHierarchyForm(adminState.hierarchySelectedUserID, node.getAttribute('data-org-line') || '');
        });
      }
      if (!document.getElementById('org-form-subject').value && user) {
        resetHierarchyForm(user.id, '');
      }
    }
    function renderHierarchyChain() {
      const container = document.getElementById('org-chain');
      if (!container) return;
      const items = adminState.hierarchyChain || [];
      container.innerHTML = items.length ? items.map((item, index) => '<article class="card"><strong>' + escapeHTML((item.user && item.user.username) || item.username || item.user_id) + '</strong><div class="row-secondary">' + escapeHTML(item.user_id || '') + '</div>' + (index < items.length - 1 ? '<div class="status">' + escapeHTML((item.resolved_via || 'manager') + ' → ' + (item.manager_username || item.manager_user_id || '')) + '</div>' : '') + '</article>').join('') : '<p class="status">Select a user to inspect the manager chain.</p>';
    }
    async function loadHierarchyChain(userID) {
      if (!userID) {
        adminState.hierarchyChain = [];
        renderHierarchyChain();
        return;
      }
      const params = new URLSearchParams(hierarchyFilters());
      params.set('user_id', userID);
      const payload = await getJSON('/admin/api/hierarchy/chain?' + params.toString());
      adminState.hierarchyChain = payload.items || [];
      renderHierarchyChain();
    }
    async function loadHierarchyGraph() {
      const params = new URLSearchParams(hierarchyFilters());
      const payload = await getJSON('/admin/api/hierarchy/graph?' + params.toString());
      adminState.hierarchyGraph = payload || {nodes: [], edges: [], summary: {}};
      renderHierarchySummary();
      renderHierarchyChart();
      renderHierarchySelection();
    }
    function resetHierarchyForm(userID, lineID) {
      adminState.hierarchySelectedLineID = lineID || '';
      const line = (adminState.reportingLines || []).find((item) => item.id === lineID) || null;
      document.getElementById('org-form-subject').value = (line && line.subject_user_id) || userID || '';
      document.getElementById('org-form-manager').value = (line && line.manager_user_id) || '';
      document.getElementById('org-form-type').value = (line && line.relationship_type) || 'primary_manager';
      document.getElementById('org-form-status').value = (line && line.status) || 'active';
      document.getElementById('org-form-priority').value = String((line && line.priority) || 0);
      document.getElementById('org-form-location').value = (line && line.location_id) || '';
      document.getElementById('org-form-effective-from').value = toDateTimeLocalValue(line && line.effective_from);
      document.getElementById('org-form-effective-to').value = toDateTimeLocalValue(line && line.effective_to);
      document.getElementById('org-form-message').textContent = line ? 'Editing ' + line.id : '';
    }
    async function saveHierarchyLine() {
      const csrf = getCookie('orbyte_csrf');
      const body = {
        subject_user_id: document.getElementById('org-form-subject').value,
        manager_user_id: document.getElementById('org-form-manager').value,
        relationship_type: document.getElementById('org-form-type').value,
        status: document.getElementById('org-form-status').value,
        priority: parseInt(document.getElementById('org-form-priority').value || '0', 10),
        organization_id: (adminState.bootstrap && adminState.bootstrap.organization && adminState.bootstrap.organization.id) || '',
        location_id: document.getElementById('org-form-location').value,
        effective_from: fromDateTimeLocalValue(document.getElementById('org-form-effective-from').value),
        effective_to: fromDateTimeLocalValue(document.getElementById('org-form-effective-to').value)
      };
      const path = adminState.hierarchySelectedLineID ? '/admin/api/reporting-lines/' + encodeURIComponent(adminState.hierarchySelectedLineID) : '/admin/api/reporting-lines';
      const method = adminState.hierarchySelectedLineID ? 'PUT' : 'POST';
      await getJSON(path, {method: method, headers: {'Content-Type':'application/json','X-CSRF-Token':csrf}, body: JSON.stringify(body)});
      const lines = await getJSON('/admin/api/reporting-lines');
      adminState.reportingLines = lines.items || [];
      document.getElementById('org-form-message').textContent = 'Reporting line saved';
      await loadHierarchyGraph();
      if (body.subject_user_id) {
        adminState.hierarchySelectedUserID = body.subject_user_id;
        renderHierarchySelection();
        void loadHierarchyChain(body.subject_user_id);
      }
    }
    function renderHierarchyAdmin() {
      populateUserSelect('org-focus-user', true);
      populateUserSelect('org-form-subject', true);
      populateUserSelect('org-form-manager', true);
      populateLocationSelect('org-location-filter', true);
      populateLocationSelect('org-form-location', true);
      populateUserSelect('workflow-sim-requester', true);
      populateUserSelect('workflow-sim-previous-approver', true);
      populateLocationSelect('workflow-sim-location', true);
      if (!document.getElementById('org-location-filter').value && adminState.users.length) {
        document.getElementById('org-focus-user').value = adminState.hierarchySelectedUserID || '';
      }
      document.getElementById('org-refresh').onclick = () => { void loadHierarchyGraph(); };
      document.getElementById('org-new-line').onclick = () => resetHierarchyForm(adminState.hierarchySelectedUserID, '');
      document.getElementById('org-save-line').onclick = () => { void saveHierarchyLine(); };
      document.getElementById('org-reset-line').onclick = () => resetHierarchyForm(adminState.hierarchySelectedUserID, adminState.hierarchySelectedLineID);
      document.getElementById('org-focus-user').onchange = () => {
        adminState.hierarchySelectedUserID = document.getElementById('org-focus-user').value;
        renderHierarchySelection();
        void loadHierarchyChain(adminState.hierarchySelectedUserID);
        renderHierarchyChart();
      };
      document.getElementById('org-location-filter').onchange = () => { void loadHierarchyGraph(); };
      document.getElementById('org-status-filter').onchange = () => { void loadHierarchyGraph(); };
      void loadHierarchyGraph();
      if (adminState.hierarchySelectedUserID) {
        void loadHierarchyChain(adminState.hierarchySelectedUserID);
      } else {
        renderHierarchyChain();
      }
    }
    async function loadWorkflowVersions(key) {
      const payload = await getJSON('/admin/api/workflows/' + encodeURIComponent(key) + '/versions');
      adminState.workflowVersions = payload.items || [];
      const versionSelect = document.getElementById('workflow-version-select');
      versionSelect.innerHTML = adminState.workflowVersions.map((item) => '<option value="' + item.version + '">' + escapeHTML('v' + item.version + ' · ' + item.status) + '</option>').join('');
      const draft = adminState.workflowVersions.find((item) => item.status === 'draft');
      versionSelect.value = String((draft && draft.version) || (adminState.workflowVersions[0] && adminState.workflowVersions[0].version) || '');
      if (versionSelect.value) {
        await loadWorkflowVersion(key, versionSelect.value);
      }
    }
    async function loadWorkflowVersion(key, version) {
      if (!key || !version) return;
      adminState.workflowCurrent = await getJSON('/admin/api/workflows/' + encodeURIComponent(key) + '/versions/' + encodeURIComponent(version));
      renderWorkflowEditor();
    }
    function workflowActionRow(row, index) {
      function select(id, value, options) {
        return '<select data-workflow-field="' + id + '" data-index="' + index + '">' + options.map((item) => '<option value="' + escapeHTML(item) + '"' + (item === (value || '') ? ' selected' : '') + '>' + escapeHTML(item || 'default') + '</option>').join('') + '</select>';
      }
      function input(id, value, type) {
        return '<input data-workflow-field="' + id + '" data-index="' + index + '" type="' + (type || 'text') + '" value="' + escapeHTML(value || '') + '">';
      }
      return '<tr>'
        + '<td>' + input('action', row.action) + '</td>'
        + '<td>' + input('from_state', row.from_state) + '</td>'
        + '<td>' + input('to_state', row.to_state) + '</td>'
        + '<td>' + select('assignment_strategy', row.assignment_strategy, ['', 'requester_manager', 'previous_approver_manager', 'role_fallback', 'static_role']) + '</td>'
        + '<td>' + input('assignee_role_key', row.assignee_role_key) + '</td>'
        + '<td>' + input('fallback_role_key', row.fallback_role_key) + '</td>'
        + '<td>' + input('task_type', row.task_type) + '</td>'
        + '<td>' + input('approval_stage_key', row.approval_stage_key) + '</td>'
        + '<td><label><input data-workflow-field="create_approval" data-index="' + index + '" type="checkbox"' + (row.create_approval ? ' checked' : '') + '> approval</label></td>'
        + '</tr>';
    }
    function bindWorkflowEditorInputs() {
      document.querySelectorAll('[data-workflow-field]').forEach((node) => {
        const handler = () => {
          const index = parseInt(node.getAttribute('data-index') || '-1', 10);
          const field = node.getAttribute('data-workflow-field');
          if (!adminState.workflowCurrent || index < 0 || !field) return;
          const row = adminState.workflowCurrent.actions[index];
          if (!row) return;
          row[field] = node.type === 'checkbox' ? !!node.checked : node.value;
        };
        node.onchange = handler;
        node.oninput = handler;
      });
    }
    function renderWorkflowEditor() {
      const current = adminState.workflowCurrent;
      if (!current) return;
      document.getElementById('workflow-states-input').value = (current.states || []).join(', ');
      document.getElementById('workflow-summary').innerHTML = ''
        + '<article class="detail-item"><span class="meta">Key</span><strong>' + escapeHTML(current.key) + '</strong></article>'
        + '<article class="detail-item"><span class="meta">Version</span><strong>' + escapeHTML('v' + current.version + ' · ' + current.status) + '</strong></article>'
        + '<article class="detail-item"><span class="meta">Transitions</span><strong>' + escapeHTML((current.actions || []).length) + '</strong></article>';
      document.getElementById('workflow-actions-editor').innerHTML = '<table class="data-table"><thead><tr><th>Action</th><th>From</th><th>To</th><th>Strategy</th><th>Role</th><th>Fallback</th><th>Task</th><th>Stage</th><th>Artifact</th></tr></thead><tbody>' + (current.actions || []).map(workflowActionRow).join('') + '</tbody></table>';
      bindWorkflowEditorInputs();
    }
    async function saveWorkflowDraft() {
      const current = adminState.workflowCurrent;
      if (!current) return;
      current.states = document.getElementById('workflow-states-input').value.split(',').map((item) => item.trim()).filter(Boolean);
      const csrf = getCookie('orbyte_csrf');
      adminState.workflowCurrent = await getJSON('/admin/api/workflows/' + encodeURIComponent(current.key) + '/versions/' + current.version, {
        method:'PUT',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify(current)
      });
      document.getElementById('workflow-message').textContent = 'Draft saved';
      await loadWorkflowVersions(current.key);
    }
    async function createWorkflowDraft() {
      const key = document.getElementById('workflow-key-select').value;
      if (!key) return;
      const csrf = getCookie('orbyte_csrf');
      await getJSON('/admin/api/workflows/' + encodeURIComponent(key) + '/drafts', {
        method:'POST',
        headers:{'X-CSRF-Token':csrf}
      });
      document.getElementById('workflow-message').textContent = 'Draft created';
      await loadWorkflowVersions(key);
    }
    async function validateWorkflowDraft() {
      const key = document.getElementById('workflow-key-select').value;
      const version = document.getElementById('workflow-version-select').value;
      if (!key || !version) return;
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/admin/api/workflows/' + encodeURIComponent(key) + '/versions/' + encodeURIComponent(version) + '/validate', {
        method:'POST',
        headers:{'X-CSRF-Token':csrf}
      });
      document.getElementById('workflow-message').textContent = payload.valid ? 'Workflow is valid' : (payload.issues || []).join('; ');
    }
    async function publishWorkflowDraft() {
      const key = document.getElementById('workflow-key-select').value;
      const version = document.getElementById('workflow-version-select').value;
      if (!key || !version) return;
      const csrf = getCookie('orbyte_csrf');
      await getJSON('/admin/api/workflows/' + encodeURIComponent(key) + '/versions/' + encodeURIComponent(version) + '/publish', {
        method:'POST',
        headers:{'X-CSRF-Token':csrf}
      });
      document.getElementById('workflow-message').textContent = 'Draft published';
      await loadWorkflowVersions(key);
    }
    function renderWorkflowSimulation() {
      const container = document.getElementById('workflow-simulation');
      if (!container) return;
      const payload = adminState.workflowSimulation;
      if (!payload) {
        container.innerHTML = '<p class="status">Run a simulation to inspect manager-chain routing.</p>';
        return;
      }
      const simulation = payload.simulation || {};
      const preview = payload.routing_preview || {};
      container.innerHTML = ''
        + '<article class="card"><strong>Transition</strong><div class="row-secondary">' + escapeHTML((simulation.transition && (simulation.transition.from_state + ' → ' + simulation.transition.to_state + ' via ' + simulation.transition.action)) || '-') + '</div></article>'
        + '<article class="card"><strong>Resolution</strong><div class="row-secondary">' + escapeHTML(preview.resolved_via || preview.error || 'n/a') + '</div>'
        + (preview.resolved_assignee_username ? '<div class="status">Assignee: ' + escapeHTML(preview.resolved_assignee_username + ' (' + preview.resolved_assignee_user_id + ')') + '</div>' : '')
        + (preview.resolved_candidate_usernames ? '<div class="status">Candidates: ' + escapeHTML(preview.resolved_candidate_usernames.join(', ')) + '</div>' : '')
        + (preview.fallback_role_key ? '<div class="status">Fallback role: ' + escapeHTML(preview.fallback_role_key) + '</div>' : '')
        + '</article>'
        + '<pre>' + escapeHTML(JSON.stringify(payload, null, 2)) + '</pre>';
    }
    async function simulateWorkflowRouting() {
      const key = document.getElementById('workflow-key-select').value;
      const version = document.getElementById('workflow-version-select').value;
      if (!key || !version) return;
      const csrf = getCookie('orbyte_csrf');
      adminState.workflowSimulation = await getJSON('/admin/api/workflows/' + encodeURIComponent(key) + '/versions/' + encodeURIComponent(version) + '/simulate', {
        method:'POST',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          current_state: document.getElementById('workflow-sim-state').value,
          action: document.getElementById('workflow-sim-action').value,
          actor_id: document.getElementById('workflow-sim-requester').value,
          organization_id: (adminState.bootstrap && adminState.bootstrap.organization && adminState.bootstrap.organization.id) || '',
          location_id: document.getElementById('workflow-sim-location').value,
          additional_input: {
            requester_user_id: document.getElementById('workflow-sim-requester').value,
            previous_approver_id: document.getElementById('workflow-sim-previous-approver').value
          }
        })
      });
      renderWorkflowSimulation();
    }
    async function renderWorkflowAdmin() {
      const select = document.getElementById('workflow-key-select');
      if (!select) return;
      const items = adminState.workflows || [];
      select.innerHTML = items.map((item) => '<option value="' + escapeHTML(item.key) + '">' + escapeHTML(item.key) + '</option>').join('');
      if (!select.value && items[0]) {
        select.value = items[0].key;
      }
      if (select.value) {
        await loadWorkflowVersions(select.value);
      }
      select.onchange = () => { void loadWorkflowVersions(select.value); };
      document.getElementById('workflow-version-select').onchange = () => { void loadWorkflowVersion(select.value, document.getElementById('workflow-version-select').value); };
      document.getElementById('workflow-create-draft').onclick = () => { void createWorkflowDraft(); };
      document.getElementById('workflow-validate').onclick = () => { void validateWorkflowDraft(); };
      document.getElementById('workflow-publish').onclick = () => { void publishWorkflowDraft(); };
      document.getElementById('workflow-save-draft').onclick = () => { void saveWorkflowDraft(); };
      document.getElementById('workflow-add-action').onclick = () => {
        if (!adminState.workflowCurrent) return;
        adminState.workflowCurrent.actions = adminState.workflowCurrent.actions || [];
        adminState.workflowCurrent.actions.push({action: '', from_state: '', to_state: '', assignment_strategy: '', assignee_role_key: '', fallback_role_key: '', task_type: '', approval_stage_key: '', create_approval: false});
        renderWorkflowEditor();
      };
      document.getElementById('workflow-simulate').onclick = () => { void simulateWorkflowRouting(); };
      renderWorkflowSimulation();
    }
