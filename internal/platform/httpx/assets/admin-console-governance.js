// Templates, navigation defaults, policy, observability, and boot.
    function renderRoleTemplates(items) {
      document.getElementById('role-templates').innerHTML = items.map((item) => {
        const template = item.template || {};
        return '<article class="card"><h3>' + pickText(template, 'name') + '</h3><p class="muted">' + template.key + ' · ' + (item.module_key || '') + '</p>' +
          (pickText(template, 'description') ? '<p class="status">' + pickText(template, 'description') + '</p>' : '') +
          '<p><strong>' + t('scopes_label') + ':</strong> ' + escapeHTML((template.allowed_scopes || []).join(', ') || '-')
          + '</p><p><strong>' + t('permissions_label') + ':</strong> ' + escapeHTML((template.permission_keys || []).join(', ') || '-')
          + '</p></article>';
      }).join('');
    }
    async function loadTemplateVersions(templateKey) {
      const payload = await getJSON('/admin/api/templates/versions?template_key=' + encodeURIComponent(templateKey));
      adminState.templateVersions = payload.items || [];
    }
    async function loadTemplateBindings(templateKey) {
      const payload = await getJSON('/admin/api/template-bindings?template_key=' + encodeURIComponent(templateKey));
      adminState.templateBindings = payload.items || [];
    }
    async function loadTemplateFixtures(templateKey, targetKind) {
      const payload = await getJSON('/admin/api/template-fixtures?template_key=' + encodeURIComponent(templateKey) + '&target_kind=' + encodeURIComponent(targetKind || ''));
      adminState.templateFixtures = payload.items || [];
    }
    function renderTemplateFixtureOptions() {
      const select = document.getElementById('template-fixture-key');
      if (!select) return;
      const currentValue = select.value;
      const items = adminState.templateFixtures || [];
      select.innerHTML = '<option value="">none</option>' + items.map((item) => '<option value="' + escapeHTML(item.fixture_key) + '">' + escapeHTML((item.name || item.fixture_key) + ' (' + item.source_type + ')') + '</option>').join('');
      if (currentValue && items.some((item) => item.fixture_key === currentValue)) {
        select.value = currentValue;
      }
    }
    function renderTemplatePreviewDiagnostics() {
      const diagnostics = document.getElementById('template-preview-diagnostics');
      const bindingDebug = document.getElementById('template-binding-debug');
      const preview = adminState.templatePreview;
      if (diagnostics) {
        if (!preview) {
          diagnostics.innerHTML = '<p class="status">-</p>';
        } else {
          const warnings = preview.warnings || [];
          const issues = preview.issues || [];
          diagnostics.innerHTML = ''
            + '<article class="card"><strong>Render</strong><div class="muted">' + escapeHTML((preview.mode || '-') + ' · ' + (preview.data_source || '-') + ' · ' + (preview.render_id || '-')) + '</div></article>'
            + '<article class="card"><strong>Warnings</strong><div class="status">' + escapeHTML(warnings.map((item) => item.message || item.code).join('; ') || '-') + '</div></article>'
            + '<article class="card"><strong>Issues</strong><div class="status">' + escapeHTML(issues.map((item) => item.message || item.code).join('; ') || '-') + '</div></article>';
        }
      }
      if (bindingDebug) {
        const debug = preview && preview.binding_resolution;
        if (!debug) {
          bindingDebug.innerHTML = '<p class="status">-</p>';
        } else {
          const scopePath = (debug.scope_path || []).map((item) => item.scope_type + (item.scope_id ? ':' + item.scope_id : '')).join(' → ');
          const matched = debug.matched_binding ? (debug.matched_binding.template_key + ' @ ' + debug.matched_binding.scope_type + (debug.matched_binding.scope_id ? ':' + debug.matched_binding.scope_id : '')) : 'module default';
          bindingDebug.innerHTML = '<article class="card"><strong>' + escapeHTML(debug.definition_key || '-') + '</strong><div class="muted">' + escapeHTML('v' + (debug.version || '-')) + '</div><div class="status">' + escapeHTML('Path: ' + (scopePath || '-')) + '</div><div class="status">' + escapeHTML('Matched: ' + matched) + '</div></article>';
        }
      }
    }
    function renderTemplateBindingScopeOptions(scopeType, selectedValue) {
      const select = document.getElementById('template-binding-scope-id');
      if (!select) return;
      if (scopeType === 'organization') {
        const org = adminState.bootstrap && adminState.bootstrap.organization;
        select.disabled = false;
        select.innerHTML = '<option value="">' + t('select_organization') + '</option>' + (org ? '<option value="' + org.id + '">' + org.name + ' (' + org.id + ')</option>' : '');
        select.value = selectedValue || '';
        return;
      }
      if (scopeType === 'location') {
        const locations = (adminState.bootstrap && adminState.bootstrap.locations) || [];
        select.disabled = false;
        select.innerHTML = '<option value="">' + t('select_location') + '</option>' + locations.map((loc) => '<option value="' + loc.id + '">' + loc.name + ' (' + loc.id + ')</option>').join('');
        select.value = selectedValue || '';
        return;
      }
      select.disabled = true;
      select.innerHTML = '<option value="">' + t('deployment_default') + '</option>';
      select.value = '';
    }
    function templatePaletteItems() {
      return [
        {type: 'text', label: t('template_block_text')},
        {type: 'field', label: t('template_block_field')},
        {type: 'table', label: t('template_block_table')},
        {type: 'totals', label: t('template_block_totals')},
        {type: 'divider', label: t('template_block_divider')},
        {type: 'image', label: t('template_block_image')},
        {type: 'barcode', label: t('template_block_barcode')},
        {type: 'signature', label: t('template_block_signature')}
      ];
    }
    function templateDefaultLayout(current) {
      const title = pickText(current, 'title') || current.key || 'Template';
      const bodyBlock = current.target_kind === 'report'
        ? {id: 'body-main', type: 'table', rows_path: 'report.rows', columns: [{label: 'Label', path: 'label'}, {label: 'Total', path: 'total'}]}
        : {id: 'body-main', type: 'field', label: 'Document Number', path: 'document.header.number'};
      return {
        schema_version: 'visual-grid/v1',
        title: title,
        settings: {paper_preset: 'a4', orientation: 'portrait', density: 'comfortable'},
        sections: [
          {id: 'header', title: t('template_section_header'), kind: 'header', rows: [{id: 'header-row-1', columns: [{id: 'header-row-1-cell-1', span: 12, blocks: [{id: 'header-title', type: 'text', text: title, font_size: 'xl', emphasis: 'strong'}]}]}]},
          {id: 'body', title: t('template_section_body'), kind: 'body', rows: [{id: 'body-row-1', columns: [{id: 'body-row-1-cell-1', span: 12, blocks: [bodyBlock]}]}]},
          {id: 'footer', title: t('template_section_footer'), kind: 'footer', rows: [{id: 'footer-row-1', columns: [{id: 'footer-row-1-cell-1', span: 12, blocks: [{id: 'footer-note', type: 'text', text: 'Prepared by Orbyte', align: 'right', emphasis: 'muted'}]}]}]}
        ]
      };
    }
    function normalizeTemplateLayout(layout, current) {
      const base = layout && typeof layout === 'object' ? layout : templateDefaultLayout(current);
      const sections = Array.isArray(base.sections) && base.sections.length ? base.sections : templateDefaultLayout(current).sections;
      return {
        schema_version: base.schema_version || 'visual-grid/v1',
        title: base.title || pickText(current, 'title') || current.key || 'Template',
        settings: Object.assign({paper_preset: 'a4', orientation: 'portrait', density: 'comfortable'}, base.settings || {}),
        sections: sections.map((section, sectionIndex) => ({
          id: section.id || ['header', 'body', 'footer'][sectionIndex] || ('section-' + (sectionIndex + 1)),
          title: section.title || ['Header', 'Body', 'Footer'][sectionIndex] || ('Section ' + (sectionIndex + 1)),
          kind: section.kind || section.id || 'body',
          rows: (Array.isArray(section.rows) && section.rows.length ? section.rows : [{columns: [{span: 12, blocks: []}]}]).map((row, rowIndex) => ({
            id: row.id || ((section.id || 'section') + '-row-' + (rowIndex + 1)),
            columns: (Array.isArray(row.columns) && row.columns.length ? row.columns : [{span: 12, blocks: []}]).map((column, columnIndex) => ({
              id: column.id || ((row.id || 'row') + '-cell-' + (columnIndex + 1)),
              span: Math.min(12, Math.max(1, parseInt(column.span || 12, 10) || 12)),
              blocks: (Array.isArray(column.blocks) ? column.blocks : []).map((block, blockIndex) => Object.assign({
                id: block.id || ((column.id || 'cell') + '-block-' + (blockIndex + 1)),
                label: '',
                text: '',
                path: '',
                rows_path: '',
                columns: [],
                align: '',
                font_size: '',
                emphasis: '',
                visible_if: ''
              }, block))
            }))
          }))
        }))
      };
    }
    function parseTemplateDesignerBody(current, body) {
      if ((current.renderer_kind || '').toLowerCase() !== 'visual') return null;
      try {
        return normalizeTemplateLayout(JSON.parse(body || '{}'), current);
      } catch (_) {
        return templateDefaultLayout(current);
      }
    }
    function selectedTemplateDefinition() {
      return (adminState.templateDefinitions || []).find((item) => item.key === document.getElementById('template-definition').value) || null;
    }
    function selectedTemplateDraft() {
      return (adminState.templateVersions || []).find((item) => item.status === 'draft') || (adminState.templateVersions || []).slice(-1)[0] || null;
    }
    function templateSectionName(section) {
      if (!section) return t('template_section_body');
      if (section.id === 'header') return t('template_section_header');
      if (section.id === 'footer') return t('template_section_footer');
      return t('template_section_body');
    }
    function findTemplateBlock(blockID) {
      const layout = adminState.templateDesigner.layout;
      if (!layout || !blockID) return null;
      for (const section of layout.sections || []) {
        for (const row of section.rows || []) {
          for (const column of row.columns || []) {
            for (const block of column.blocks || []) {
              if (block.id === blockID) return {section, row, column, block};
            }
          }
        }
      }
      return null;
    }
    function nextDesignerID(prefix) {
      return prefix + '-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 7);
    }
    function createTemplateBlock(type) {
      const id = nextDesignerID('block');
      switch (type) {
      case 'field':
        return {id, type, label: 'Field', path: 'document.header.number'};
      case 'table':
        return {id, type, rows_path: 'document.lines', columns: [{label: 'Label', path: 'payload.name'}, {label: 'Amount', path: 'amount'}]};
      case 'totals':
        return {id, type, label: 'Total', rows_path: 'document.lines', path: 'amount'};
      case 'divider':
        return {id, type};
      case 'image':
        return {id, type, label: 'Logo', image_url: ''};
      case 'barcode':
        return {id, type, label: 'Barcode', path: 'document.header.number'};
      case 'signature':
        return {id, type, label: 'Authorized Signature'};
      default:
        return {id, type: 'text', text: 'New text block'};
      }
    }
    function removeTemplateBlock(blockID) {
      const layout = adminState.templateDesigner.layout;
      if (!layout) return null;
      for (const section of layout.sections || []) {
        for (const row of section.rows || []) {
          for (const column of row.columns || []) {
            const index = (column.blocks || []).findIndex((item) => item.id === blockID);
            if (index >= 0) {
              return column.blocks.splice(index, 1)[0];
            }
          }
        }
      }
      return null;
    }
    function moveTemplateBlock(blockID, targetCellID, beforeBlockID) {
      const layout = adminState.templateDesigner.layout;
      if (!layout) return;
      const block = removeTemplateBlock(blockID);
      if (!block) return;
      for (const section of layout.sections || []) {
        for (const row of section.rows || []) {
          for (const column of row.columns || []) {
            if (column.id !== targetCellID) continue;
            column.blocks = column.blocks || [];
            if (beforeBlockID) {
              const index = column.blocks.findIndex((item) => item.id === beforeBlockID);
              if (index >= 0) {
                column.blocks.splice(index, 0, block);
                return;
              }
            }
            column.blocks.push(block);
            return;
          }
        }
      }
    }
    function moveRow(sectionID, rowID, direction) {
      const section = (adminState.templateDesigner.layout && adminState.templateDesigner.layout.sections || []).find((item) => item.id === sectionID);
      if (!section || !section.rows) return;
      const index = section.rows.findIndex((item) => item.id === rowID);
      const nextIndex = index + direction;
      if (index < 0 || nextIndex < 0 || nextIndex >= section.rows.length) return;
      const row = section.rows.splice(index, 1)[0];
      section.rows.splice(nextIndex, 0, row);
    }
    function addColumnToActiveSection() {
      const section = (adminState.templateDesigner.layout && adminState.templateDesigner.layout.sections || []).find((item) => item.id === adminState.templateDesigner.sectionID);
      if (!section || !section.rows || !section.rows.length) return;
      const row = section.rows[section.rows.length - 1];
      row.columns = row.columns || [];
      const nextCount = row.columns.length + 1;
      const nextSpan = Math.max(2, Math.floor(12 / nextCount));
      row.columns = row.columns.map((column) => Object.assign({}, column, {span: nextSpan}));
      row.columns.push({id: nextDesignerID('cell'), span: nextSpan, blocks: []});
    }
    function removeColumn(cellID) {
      const layout = adminState.templateDesigner.layout;
      if (!layout) return;
      for (const section of layout.sections || []) {
        for (const row of section.rows || []) {
          const index = (row.columns || []).findIndex((column) => column.id === cellID);
          if (index >= 0) {
            if ((row.columns || []).length === 1) return;
            row.columns.splice(index, 1);
            const nextSpan = Math.max(2, Math.floor(12 / row.columns.length));
            row.columns = row.columns.map((column) => Object.assign({}, column, {span: nextSpan}));
            return;
          }
        }
      }
    }
    function blockTypeFields(type) {
      switch ((type || '').toLowerCase()) {
      case 'text':
        return ['text', 'align', 'font_size', 'emphasis', 'visible_if'];
      case 'field':
        return ['label', 'path', 'format', 'align', 'font_size', 'emphasis', 'visible_if'];
      case 'table':
        return ['label', 'rows_path', 'columns', 'visible_if'];
      case 'totals':
        return ['label', 'rows_path', 'path', 'format', 'visible_if'];
      case 'image':
        return ['label', 'image_url', 'alt', 'align', 'visible_if'];
      case 'barcode':
        return ['label', 'path', 'value', 'format', 'visible_if'];
      case 'signature':
        return ['label', 'align', 'visible_if'];
      case 'divider':
        return ['visible_if'];
      default:
        return ['label', 'text', 'path', 'rows_path', 'columns', 'align', 'font_size', 'emphasis', 'visible_if'];
      }
    }
    function renderTemplateBindings() {
      const container = document.getElementById('template-bindings');
      if (!container) return;
      const current = selectedTemplateDefinition();
      const bindings = (adminState.templateBindings || []).slice().sort((left, right) => {
        const weight = function(scopeType) {
          if (scopeType === 'location') return 3;
          if (scopeType === 'organization') return 2;
          return 1;
        };
        return weight(right.scope_type) - weight(left.scope_type);
      });
      if (!bindings.length) {
        container.innerHTML = current
          ? '<article class="card"><strong>' + escapeHTML(t('template_module_default')) + '</strong><div class="muted">' + escapeHTML(current.target_kind + ' · ' + current.target_key + ' · ' + (current.purpose || '-') + ' · ' + (current.channel || '-')) + '</div><div class="status">' + escapeHTML(t('template_module_default_help')) + '</div></article>'
          : '<p class="status">-</p>';
        return;
      }
      container.innerHTML = bindings.map((item, index) => {
        const flags = [item.is_default ? t('template_binding_default') : '', item.is_official ? t('template_binding_official') : ''].filter(Boolean);
        const priority = index === 0 ? t('template_binding_effective') : (item.scope_type === 'location' ? t('template_binding_overrides_broader') : item.scope_type === 'organization' ? t('template_binding_overrides_deployment') : t('template_binding_fallback'));
        return '<article class="card"><strong>' + escapeHTML(item.scope_type + (item.scope_id ? ':' + item.scope_id : '')) + '</strong><div class="muted">' + escapeHTML(item.target_kind + ' · ' + item.target_key + ' · ' + (item.purpose || '-') + ' · ' + (item.channel || '-')) + '</div><div class="status">' + escapeHTML([priority].concat(flags).join(' · ')) + '</div></article>';
      }).join('');
    }
    function syncTemplateDesignerBody() {
      const current = selectedTemplateDefinition();
      if (!current) return;
      if ((current.renderer_kind || '').toLowerCase() === 'visual' && adminState.templateDesigner.layout) {
        document.getElementById('template-body').value = JSON.stringify(adminState.templateDesigner.layout, null, 2);
        const settings = adminState.templateDesigner.layout.settings || {};
        let preset = settings.paper_preset || 'a4';
        if (preset === 'a4' && settings.orientation === 'landscape') preset = 'a4-landscape';
        document.getElementById('template-paper-preset').value = preset;
      }
    }
    function renderTemplateSectionTabs() {
      const container = document.getElementById('template-section-tabs');
      const layout = adminState.templateDesigner.layout;
      if (!container || !layout) return;
      container.innerHTML = (layout.sections || []).map((section) => {
        const active = adminState.templateDesigner.sectionID === section.id ? 'template-section-tab active' : 'template-section-tab';
        return '<button type="button" class="' + active + '" data-template-section="' + escapeHTML(section.id) + '">' + escapeHTML(templateSectionName(section)) + '</button>';
      }).join('');
      container.querySelectorAll('[data-template-section]').forEach((node) => {
        node.onclick = () => {
          adminState.templateDesigner.sectionID = node.getAttribute('data-template-section') || 'body';
          renderTemplateDesigner();
        };
      });
    }
    function renderTemplatePalette() {
      const palette = document.getElementById('template-block-palette');
      if (!palette) return;
      palette.innerHTML = templatePaletteItems().map((item) => '<button type="button" class="secondary" draggable="true" data-template-palette="' + item.type + '">' + escapeHTML(item.label) + '</button>').join('');
      palette.querySelectorAll('[data-template-palette]').forEach((node) => {
        node.addEventListener('dragstart', (event) => {
          event.dataTransfer.setData('text/plain', JSON.stringify({kind: 'palette', type: node.getAttribute('data-template-palette')}));
        });
        node.onclick = () => {
          const section = (adminState.templateDesigner.layout.sections || []).find((item) => item.id === adminState.templateDesigner.sectionID);
          if (!section || !section.rows || !section.rows[0] || !section.rows[0].columns || !section.rows[0].columns[0]) return;
          section.rows[0].columns[0].blocks.push(createTemplateBlock(node.getAttribute('data-template-palette') || 'text'));
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      });
    }
    function renderTemplateCanvas() {
      const canvas = document.getElementById('template-canvas');
      const layout = adminState.templateDesigner.layout;
      if (!canvas || !layout) return;
      const section = (layout.sections || []).find((item) => item.id === adminState.templateDesigner.sectionID) || (layout.sections || [])[0];
      if (!section) {
        canvas.innerHTML = '<p class="status">-</p>';
        return;
      }
      document.getElementById('template-active-section').textContent = templateSectionName(section);
      const preset = (() => {
        const settings = layout.settings || {};
        if (settings.paper_preset === 'receipt-80' || settings.paper_preset === 'receipt-58') return settings.paper_preset;
        if (settings.paper_preset === 'a4' && settings.orientation === 'landscape') return 'a4-landscape';
        return settings.paper_preset || 'a4';
      })();
      canvas.innerHTML = '<div class="template-paper ' + escapeHTML('paper-' + preset + ' density-' + ((layout.settings && layout.settings.density) || 'comfortable')) + '">' +
        '<div class="template-designer-section">' +
        (section.rows || []).map((row, rowIndex) => '<div class="template-designer-row-wrap"><div class="page-actions compact template-row-actions"><span class="status">Row ' + (rowIndex + 1) + '</span><button type="button" class="secondary" data-template-row-move="' + escapeHTML(section.id + ':up:' + row.id) + '">' + escapeHTML(t('template_move_up')) + '</button><button type="button" class="secondary" data-template-row-move="' + escapeHTML(section.id + ':down:' + row.id) + '">' + escapeHTML(t('template_move_down')) + '</button><button type="button" class="secondary" data-template-row-delete="' + escapeHTML(section.id + ':' + row.id) + '">' + escapeHTML(t('template_delete_row')) + '</button></div><div class="template-designer-row">' + (row.columns || []).map((column) => {
          const span = Math.min(12, Math.max(1, parseInt(column.span || 12, 10) || 12));
          return '<div class="template-cell-drop" data-template-cell="' + escapeHTML(column.id) + '" style="grid-column: span ' + span + ' / span ' + span + ';">' +
            '<div class="template-cell-toolbar"><span class="muted">Span ' + span + '/12</span>' + ((row.columns || []).length > 1 ? '<button type="button" class="secondary" data-template-remove-column="' + escapeHTML(column.id) + '">' + escapeHTML(t('template_remove_column')) + '</button>' : '') + '</div>' +
            ((column.blocks || []).map((block) => '<div class="template-designer-block' + (adminState.templateDesigner.selectedBlockID === block.id ? ' is-selected' : '') + '" draggable="true" data-template-block="' + escapeHTML(block.id) + '">' +
            '<div class="template-block-title">' + escapeHTML(block.label || block.text || block.type) + '</div>' +
            '<div class="template-block-meta">' + escapeHTML(block.type + (block.path ? ' · ' + block.path : block.rows_path ? ' · ' + block.rows_path : '')) + '</div>' +
            '</div>').join('') || '<div class="template-block-meta">Drop block here</div>') +
            '</div>';
        }).join('') + '</div></div>').join('') +
        '</div></div>';
      canvas.querySelectorAll('[data-template-row-move]').forEach((node) => {
        node.onclick = () => {
          const parts = (node.getAttribute('data-template-row-move') || '').split(':');
          if (parts.length !== 3) return;
          moveRow(parts[0], parts[2], parts[1] === 'up' ? -1 : 1);
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      });
      canvas.querySelectorAll('[data-template-row-delete]').forEach((node) => {
        node.onclick = () => {
          const parts = (node.getAttribute('data-template-row-delete') || '').split(':');
          if (parts.length !== 2) return;
          const targetSection = (layout.sections || []).find((item) => item.id === parts[0]);
          if (!targetSection || !targetSection.rows || targetSection.rows.length <= 1) return;
          targetSection.rows = targetSection.rows.filter((item) => item.id !== parts[1]);
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      });
      canvas.querySelectorAll('[data-template-remove-column]').forEach((node) => {
        node.onclick = () => {
          removeColumn(node.getAttribute('data-template-remove-column') || '');
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      });
      canvas.querySelectorAll('[data-template-cell]').forEach((node) => {
        node.addEventListener('dragover', (event) => {
          event.preventDefault();
          node.classList.add('dragover');
        });
        node.addEventListener('dragleave', () => node.classList.remove('dragover'));
        node.addEventListener('drop', (event) => {
          event.preventDefault();
          node.classList.remove('dragover');
          let payload = null;
          try {
            payload = JSON.parse(event.dataTransfer.getData('text/plain') || '{}');
          } catch (_) {}
          if (!payload) return;
          const cellID = node.getAttribute('data-template-cell');
          let targetCell = null;
          for (const sectionItem of layout.sections || []) {
            for (const row of sectionItem.rows || []) {
              for (const column of row.columns || []) {
                if (column.id === cellID) targetCell = column;
              }
            }
          }
          if (!targetCell) return;
          const beforeNode = event.target && event.target.closest ? event.target.closest('[data-template-block]') : null;
          const beforeBlockID = beforeNode ? beforeNode.getAttribute('data-template-block') : '';
          if (payload.kind === 'palette') {
            const block = createTemplateBlock(payload.type || 'text');
            if (beforeBlockID) {
              const index = targetCell.blocks.findIndex((item) => item.id === beforeBlockID);
              if (index >= 0) targetCell.blocks.splice(index, 0, block);
              else targetCell.blocks.push(block);
            } else {
              targetCell.blocks.push(block);
            }
          }
          if (payload.kind === 'block' && payload.block_id) {
            moveTemplateBlock(payload.block_id, cellID, beforeBlockID || '');
          }
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        });
      });
      canvas.querySelectorAll('[data-template-block]').forEach((node) => {
        node.addEventListener('dragstart', (event) => {
          event.dataTransfer.setData('text/plain', JSON.stringify({kind: 'block', block_id: node.getAttribute('data-template-block')}));
        });
        node.onclick = () => {
          adminState.templateDesigner.selectedBlockID = node.getAttribute('data-template-block') || '';
          renderTemplateDesigner();
        };
      });
    }
    function renderTemplateInspector() {
      const inspector = document.getElementById('template-inspector');
      const layout = adminState.templateDesigner.layout;
      if (!inspector || !layout) return;
      const current = selectedTemplateDefinition();
      const selected = findTemplateBlock(adminState.templateDesigner.selectedBlockID);
      if ((current && current.renderer_kind || '').toLowerCase() !== 'visual') {
        inspector.innerHTML = '<p class="status">' + escapeHTML(t('template_inspector_empty')) + '</p>';
        return;
      }
      if (!selected) {
        inspector.innerHTML = '<label class="field"><span>' + escapeHTML(t('template_paper_preset')) + '</span><select id="template-inspector-paper-preset"><option value="a4">A4 Portrait</option><option value="a4-landscape">A4 Landscape</option><option value="receipt-80">Receipt 80mm</option><option value="receipt-58">Receipt 58mm</option></select></label>' +
          '<label class="field"><span>Density</span><select id="template-inspector-density"><option value="comfortable">comfortable</option><option value="compact">compact</option></select></label>' +
          '<p class="status">' + escapeHTML(t('template_inspector_empty')) + '</p>';
        const presetNode = document.getElementById('template-inspector-paper-preset');
        const densityNode = document.getElementById('template-inspector-density');
        let preset = (layout.settings && layout.settings.paper_preset) || 'a4';
        if (preset === 'a4' && (layout.settings && layout.settings.orientation) === 'landscape') preset = 'a4-landscape';
        presetNode.value = preset;
        densityNode.value = (layout.settings && layout.settings.density) || 'comfortable';
        presetNode.onchange = () => {
          if (!layout.settings) layout.settings = {};
          if (presetNode.value === 'a4-landscape') {
            layout.settings.paper_preset = 'a4';
            layout.settings.orientation = 'landscape';
          } else {
            layout.settings.paper_preset = presetNode.value;
            layout.settings.orientation = 'portrait';
          }
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
        densityNode.onchange = () => {
          if (!layout.settings) layout.settings = {};
          layout.settings.density = densityNode.value;
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
        return;
      }
      const block = selected.block;
      const supportedFields = blockTypeFields(block.type);
      let content = '<label class="field"><span>' + escapeHTML(t('template_block_label')) + '</span><input id="template-inspector-label" value="' + escapeHTML(block.label || '') + '"></label>';
      if (supportedFields.includes('text')) content += '<label class="field"><span>' + escapeHTML(t('template_block_text_prop')) + '</span><textarea id="template-inspector-text">' + escapeHTML(block.text || '') + '</textarea></label>';
      if (supportedFields.includes('path')) content += '<label class="field"><span>' + escapeHTML(t('template_block_path')) + '</span><input id="template-inspector-path" value="' + escapeHTML(block.path || '') + '"></label>';
      if (supportedFields.includes('rows_path')) content += '<label class="field"><span>' + escapeHTML(t('template_block_rows_path')) + '</span><input id="template-inspector-rows-path" value="' + escapeHTML(block.rows_path || '') + '"></label>';
      if (supportedFields.includes('value')) content += '<label class="field"><span>' + escapeHTML(t('template_block_value')) + '</span><input id="template-inspector-value" value="' + escapeHTML(block.value || '') + '"></label>';
      if (supportedFields.includes('image_url')) content += '<label class="field"><span>' + escapeHTML(t('template_block_image_url')) + '</span><input id="template-inspector-image-url" value="' + escapeHTML(block.image_url || '') + '"></label>';
      if (supportedFields.includes('alt')) content += '<label class="field"><span>' + escapeHTML(t('template_block_alt')) + '</span><input id="template-inspector-alt" value="' + escapeHTML(block.alt || '') + '"></label>';
      if (supportedFields.includes('format')) content += '<label class="field"><span>' + escapeHTML(t('template_block_format')) + '</span><input id="template-inspector-format" value="' + escapeHTML(block.format || '') + '"></label>';
      if (supportedFields.includes('columns')) {
        const columns = Array.isArray(block.columns) ? block.columns : [];
        content += '<div class="field"><span>' + escapeHTML(t('template_block_columns')) + '</span><div id="template-column-editor">' + (columns.map((column, index) => '<div class="form-grid compact"><label class="field"><span>' + escapeHTML(t('template_column_label')) + '</span><input data-template-column-label="' + index + '" value="' + escapeHTML(column.label || '') + '"></label><label class="field"><span>' + escapeHTML(t('template_column_path')) + '</span><input data-template-column-path="' + index + '" value="' + escapeHTML(column.path || '') + '"></label><div class="actions compact"><button type="button" class="secondary" data-template-column-remove="' + index + '">' + escapeHTML(t('template_remove_column_definition')) + '</button></div></div>').join('') || '<p class="status">' + escapeHTML(t('template_no_columns')) + '</p>') + '</div><button type="button" id="template-inspector-add-column" class="secondary">' + escapeHTML(t('template_add_column_definition')) + '</button></div>';
      }
      content +=
        '<label class="field"><span>' + escapeHTML(t('template_block_span')) + '</span><input id="template-inspector-span" type="number" min="1" max="12" value="' + escapeHTML(String(selected.column.span || 12)) + '"></label>' +
        (supportedFields.includes('align') ? '<label class="field"><span>' + escapeHTML(t('template_block_align')) + '</span><select id="template-inspector-align"><option value=\"\">default</option><option value=\"left\">left</option><option value=\"center\">center</option><option value=\"right\">right</option></select></label>' : '') +
        (supportedFields.includes('font_size') ? '<label class="field"><span>' + escapeHTML(t('template_block_size')) + '</span><select id="template-inspector-size"><option value=\"\">default</option><option value=\"sm\">sm</option><option value=\"lg\">lg</option><option value=\"xl\">xl</option></select></label>' : '') +
        (supportedFields.includes('emphasis') ? '<label class="field"><span>' + escapeHTML(t('template_block_emphasis')) + '</span><select id="template-inspector-emphasis"><option value=\"\">default</option><option value=\"strong\">strong</option><option value=\"muted\">muted</option></select></label>' : '') +
        (supportedFields.includes('visible_if') ? '<label class="field"><span>' + escapeHTML(t('template_block_visible_if')) + '</span><input id="template-inspector-visible-if" value="' + escapeHTML(block.visible_if || '') + '"></label>' : '') +
        '<div class="actions"><button id="template-duplicate-block" class="secondary">' + escapeHTML(t('template_duplicate_block')) + '</button><button id="template-delete-block" class="warn">' + escapeHTML(t('template_delete_block')) + '</button></div>';
      inspector.innerHTML = content;
      const bind = (id, key) => {
        const node = document.getElementById(id);
        if (!node) return;
        node.value = key === 'align' || key === 'font_size' || key === 'emphasis' ? (block[key] || '') : node.value;
        node.oninput = () => {
          if (key === 'columns') {
            try {
              block.columns = JSON.parse(node.value || '[]');
            } catch (_) {}
          } else if (key === 'span') {
            selected.column.span = Math.min(12, Math.max(1, parseInt(node.value || '12', 10) || 12));
          } else {
            block[key] = node.value;
          }
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
        node.onchange = node.oninput;
      };
      bind('template-inspector-label', 'label');
      bind('template-inspector-text', 'text');
      bind('template-inspector-path', 'path');
      bind('template-inspector-rows-path', 'rows_path');
      bind('template-inspector-value', 'value');
      bind('template-inspector-image-url', 'image_url');
      bind('template-inspector-alt', 'alt');
      bind('template-inspector-format', 'format');
      bind('template-inspector-span', 'span');
      bind('template-inspector-align', 'align');
      bind('template-inspector-size', 'font_size');
      bind('template-inspector-emphasis', 'emphasis');
      bind('template-inspector-visible-if', 'visible_if');
      const addColumnButton = document.getElementById('template-inspector-add-column');
      if (addColumnButton) {
        addColumnButton.onclick = () => {
          block.columns = Array.isArray(block.columns) ? block.columns : [];
          block.columns.push({label: 'Column', path: ''});
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      }
      inspector.querySelectorAll('[data-template-column-label]').forEach((node) => {
        node.oninput = () => {
          const index = parseInt(node.getAttribute('data-template-column-label') || '-1', 10);
          if (index < 0) return;
          block.columns = Array.isArray(block.columns) ? block.columns : [];
          block.columns[index] = Object.assign({}, block.columns[index] || {}, {label: node.value});
          syncTemplateDesignerBody();
        };
      });
      inspector.querySelectorAll('[data-template-column-path]').forEach((node) => {
        node.oninput = () => {
          const index = parseInt(node.getAttribute('data-template-column-path') || '-1', 10);
          if (index < 0) return;
          block.columns = Array.isArray(block.columns) ? block.columns : [];
          block.columns[index] = Object.assign({}, block.columns[index] || {}, {path: node.value});
          syncTemplateDesignerBody();
        };
      });
      inspector.querySelectorAll('[data-template-column-remove]').forEach((node) => {
        node.onclick = () => {
          const index = parseInt(node.getAttribute('data-template-column-remove') || '-1', 10);
          if (index < 0 || !Array.isArray(block.columns)) return;
          block.columns.splice(index, 1);
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      });
      document.getElementById('template-duplicate-block').onclick = () => {
        selected.column.blocks.push(Object.assign({}, JSON.parse(JSON.stringify(block)), {id: nextDesignerID('block')}));
        syncTemplateDesignerBody();
        renderTemplateDesigner();
      };
      document.getElementById('template-delete-block').onclick = () => {
        removeTemplateBlock(block.id);
        adminState.templateDesigner.selectedBlockID = '';
        syncTemplateDesignerBody();
        renderTemplateDesigner();
      };
    }
    function renderTemplateDesigner() {
      renderTemplatePalette();
      renderTemplateSectionTabs();
      renderTemplateCanvas();
      renderTemplateInspector();
      syncTemplateDesignerBody();
    }
    async function renderTemplates(loadVersions) {
      const defs = adminState.templateDefinitions || [];
      const select = document.getElementById('template-definition');
      if (!select) return;
      select.innerHTML = defs.map((item) => '<option value="' + item.key + '">' + escapeHTML(pickText(item, 'title') || item.key) + ' (' + escapeHTML(item.key) + ')</option>').join('');
      if (!defs.length) {
        document.getElementById('template-preview').innerHTML = '<p class="status">-</p>';
        document.getElementById('template-versions').innerHTML = '<p class="status">-</p>';
        document.getElementById('template-bindings').innerHTML = '<p class="status">-</p>';
        return;
      }
      if (!select.value) {
        select.value = defs[0].key;
      }
      const current = defs.find((item) => item.key === select.value) || defs[0];
      if (loadVersions !== false) {
        await loadTemplateVersions(current.key);
      }
      await loadTemplateBindings(current.key);
      await loadTemplateFixtures(current.key, current.target_kind);
      const currentBinding = (adminState.templateBindings || []).find((item) => item.template_key === current.key) || null;
      const draft = (adminState.templateVersions || []).find((item) => item.status === 'draft') || (adminState.templateVersions || []).slice(-1)[0];
      document.getElementById('template-body').value = (draft && draft.body) || current.default_body || '';
      document.getElementById('template-style').value = (draft && draft.style) || current.default_style || '';
      document.getElementById('template-purpose').value = (currentBinding && currentBinding.purpose) || current.purpose || '';
      document.getElementById('template-channel').value = (currentBinding && currentBinding.channel) || current.channel || '';
      document.getElementById('template-binding-scope').value = (currentBinding && currentBinding.scope_type) || 'deployment';
      document.getElementById('template-binding-default').checked = currentBinding ? !!currentBinding.is_default : true;
      document.getElementById('template-binding-official').checked = currentBinding ? !!currentBinding.is_official : (current.purpose || '') === 'official';
      document.getElementById('template-render-target-key').value = current.target_key || '';
      document.getElementById('template-render-target-id').value = '';
      document.getElementById('template-render-mode').value = 'sample';
      renderTemplateFixtureOptions();
      document.getElementById('template-status').textContent = t('loaded_template') + ' · ' + current.key;
      document.getElementById('template-versions').innerHTML = (adminState.templateVersions || []).map((item) => '<article class="card"><strong>v' + item.version + '</strong><div class="muted">' + escapeHTML(item.status) + ' · ' + escapeHTML(item.renderer_kind) + '</div><div class="status">' + escapeHTML((item.change_note || '-') + (item.last_render_status ? ' · ' + item.last_render_status : '')) + '</div></article>').join('');
      renderTemplateBindingScopeOptions(document.getElementById('template-binding-scope').value, currentBinding && currentBinding.scope_id);
      renderTemplateBindings();
      renderTemplatePreviewDiagnostics();
      adminState.templateDesigner.layout = parseTemplateDesignerBody(current, document.getElementById('template-body').value);
      adminState.templateDesigner.sectionID = 'body';
      adminState.templateDesigner.selectedBlockID = '';
      const designerGrid = document.querySelector('[data-admin-route="/admin/templates"] .template-admin-grid');
      if (designerGrid) designerGrid.style.display = ((current.renderer_kind || '').toLowerCase() === 'visual') ? '' : 'none';
      document.getElementById('load-template-definition').onclick = () => { void renderTemplates(true); };
      document.getElementById('template-definition').onchange = () => { void renderTemplates(true); };
      document.getElementById('save-template-draft').onclick = saveTemplateDraft;
      document.getElementById('publish-template-version').onclick = publishTemplateDraft;
      document.getElementById('duplicate-template-draft').onclick = duplicateTemplateDraft;
      document.getElementById('reset-template-draft').onclick = resetTemplateDraft;
      document.getElementById('compare-template-version').onclick = compareTemplateDraft;
      document.getElementById('save-template-binding').onclick = saveTemplateBinding;
      document.getElementById('preview-template-render').onclick = previewTemplateRender;
      document.getElementById('template-binding-scope').onchange = () => {
        renderTemplateBindingScopeOptions(document.getElementById('template-binding-scope').value, '');
      };
      document.getElementById('template-add-row').onclick = () => {
        const layout = adminState.templateDesigner.layout;
        const section = layout && (layout.sections || []).find((item) => item.id === adminState.templateDesigner.sectionID);
        if (!section) return;
        const rowID = nextDesignerID((section.id || 'section') + '-row');
        section.rows.push({id: rowID, columns: [{id: rowID + '-cell-1', span: 12, blocks: []}]});
        syncTemplateDesignerBody();
        renderTemplateDesigner();
      };
      document.getElementById('template-paper-preset').onchange = () => {
        const layout = adminState.templateDesigner.layout;
        if (!layout) return;
        if (!layout.settings) layout.settings = {};
        const preset = document.getElementById('template-paper-preset').value;
        if (preset === 'a4-landscape') {
          layout.settings.paper_preset = 'a4';
          layout.settings.orientation = 'landscape';
        } else {
          layout.settings.paper_preset = preset;
          layout.settings.orientation = 'portrait';
        }
        syncTemplateDesignerBody();
        renderTemplateDesigner();
      };
      document.getElementById('template-add-column').onclick = () => {
        addColumnToActiveSection();
        syncTemplateDesignerBody();
        renderTemplateDesigner();
      };
      renderTemplateDesigner();
    }
    async function saveTemplateDraft() {
      const current = selectedTemplateDefinition();
      if (!current) return;
      const key = current.key;
      if ((current.renderer_kind || '').toLowerCase() === 'visual') syncTemplateDesignerBody();
      const csrf = getCookie('orbyte_csrf');
      await getJSON('/admin/api/templates/' + encodeURIComponent(key) + '/actions/draft', {
        method:'PUT',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({body: document.getElementById('template-body').value, style: document.getElementById('template-style').value})
      });
      document.getElementById('template-status').textContent = t('saved_template_draft') + ' · ' + key;
      await renderTemplates(true);
    }
    async function duplicateTemplateDraft() {
      const current = selectedTemplateDefinition();
      if (!current) return;
      const source = selectedTemplateDraft() || ((adminState.templateVersions || []).find((item) => item.status === 'published')) || null;
      const csrf = getCookie('orbyte_csrf');
      await getJSON('/admin/api/templates/' + encodeURIComponent(current.key) + '/actions/duplicate-draft', {
        method:'POST',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({from_version: source ? source.version : 0})
      });
      document.getElementById('template-status').textContent = 'Draft duplicated · ' + current.key;
      await renderTemplates(true);
    }
    async function resetTemplateDraft() {
      const current = selectedTemplateDefinition();
      if (!current) return;
      const csrf = getCookie('orbyte_csrf');
      await getJSON('/admin/api/templates/' + encodeURIComponent(current.key) + '/actions/reset-draft', {
        method:'POST',
        headers:{'X-CSRF-Token':csrf}
      });
      document.getElementById('template-status').textContent = 'Draft reset · ' + current.key;
      await renderTemplates(true);
    }
    async function compareTemplateDraft() {
      const current = selectedTemplateDefinition();
      if (!current) return;
      const draft = (adminState.templateVersions || []).find((item) => item.status === 'draft');
      const published = (adminState.templateVersions || []).find((item) => item.status === 'published');
      if (!draft || !published) {
        document.getElementById('template-status').textContent = 'Draft and published versions are required for compare';
        return;
      }
      const payload = await getJSON('/admin/api/templates/compare?template_key=' + encodeURIComponent(current.key) + '&left=' + encodeURIComponent(String(published.version)) + '&right=' + encodeURIComponent(String(draft.version)));
      const comparison = payload.comparison || {};
      document.getElementById('template-status').textContent = 'Compare · ' + current.key + ' · ' + ((comparison.changed_fields || []).join(', ') || 'no differences');
    }
    async function publishTemplateDraft() {
      const key = document.getElementById('template-definition').value;
      const draft = (adminState.templateVersions || []).find((item) => item.status === 'draft');
      if (!draft) return;
      const csrf = getCookie('orbyte_csrf');
      await getJSON('/admin/api/templates/' + encodeURIComponent(key) + '/versions/' + draft.version + '/publish', {
        method:'POST',
        headers:{'X-CSRF-Token':csrf}
      });
      document.getElementById('template-status').textContent = t('published_template_version') + ' · ' + key + ' v' + draft.version;
      await renderTemplates(true);
    }
    async function saveTemplateBinding() {
      const current = (adminState.templateDefinitions || []).find((item) => item.key === document.getElementById('template-definition').value);
      if (!current) return;
      const csrf = getCookie('orbyte_csrf');
      const previous = (adminState.templateBindings || []).find((item) =>
        item.scope_type === document.getElementById('template-binding-scope').value &&
        (item.scope_id || '') === document.getElementById('template-binding-scope-id').value &&
        item.target_kind === current.target_kind &&
        item.target_key === current.target_key &&
        (item.purpose || '') === document.getElementById('template-purpose').value &&
        (item.channel || '') === document.getElementById('template-channel').value
      );
      const payload = await getJSON('/admin/api/template-bindings', {
        method:'PUT',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          template_key: current.key,
          scope_type: document.getElementById('template-binding-scope').value,
          scope_id: document.getElementById('template-binding-scope-id').value,
          target_kind: current.target_kind,
          target_key: current.target_key,
          purpose: document.getElementById('template-purpose').value,
          channel: document.getElementById('template-channel').value,
          is_default: !!document.getElementById('template-binding-default').checked,
          is_official: !!document.getElementById('template-binding-official').checked
        })
      });
      adminState.templateBindings = [payload.binding].concat((adminState.templateBindings || []).filter((item) => item.id !== payload.binding.id));
      document.getElementById('template-status').textContent = (previous ? t('updated_template_binding') : t('saved_template_binding')) + ' · ' + current.key;
      await renderTemplates(false);
    }
    async function previewTemplateRender() {
      const current = selectedTemplateDefinition();
      if (!current) return;
      if ((current.renderer_kind || '').toLowerCase() === 'visual') syncTemplateDesignerBody();
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/admin/api/templates/preview', {
        method:'POST',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          template_key: current.key,
          target_kind: current.target_kind,
          target_key: document.getElementById('template-render-target-key').value || current.target_key,
          target_id: document.getElementById('template-render-target-id').value,
          sample: document.getElementById('template-render-mode').value === 'sample',
          fixture_key: document.getElementById('template-fixture-key').value,
          draft: true,
          purpose: document.getElementById('template-purpose').value,
          channel: document.getElementById('template-channel').value,
          body: document.getElementById('template-body').value,
          style: document.getElementById('template-style').value,
          renderer_kind: current.renderer_kind
        })
      });
      adminState.templatePreview = payload.preview || null;
      const htmlPreview = ((adminState.templatePreview && adminState.templatePreview.outputs) || []).find((item) => item.format === 'html') || {};
      document.getElementById('template-preview').innerHTML = htmlPreview.html || '';
      renderTemplatePreviewDiagnostics();
    }
    function routeOptions(surface) {
      const actions = surface === 'admin'
        ? ((adminState.bootstrap && adminState.bootstrap.actions) || [])
        : ((adminState.bootstrap && adminState.bootstrap.user_actions) || []);
      const seen = new Set();
      const items = actions.filter((action) => {
        const path = action && action.route_path;
        if (!path || seen.has(path)) return false;
        seen.add(path);
        return true;
      }).map((action) => ({path: action.route_path, label: pickText(action, 'label') || action.route_path}));
      if (surface === 'admin' && adminState.bootstrap && adminState.bootstrap.acp && adminState.bootstrap.acp.enabled && !seen.has('/admin/agent')) {
        items.push({path: '/admin/agent', label: 'Agent'});
      }
      return items;
    }
    function renderRouteOptionsDatalist(id, surface) {
      const options = routeOptions(surface);
      const node = document.getElementById(id);
      if (!node) return;
      node.innerHTML = options.map((item) => '<option value="' + escapeHTML(item.path) + '">' + escapeHTML(item.label) + '</option>').join('');
    }
    function selectedUser() {
      const select = document.getElementById('navigation-user-id');
      if (!select) return null;
      const value = select.value;
      return adminState.users.find((item) => item.id === value) || null;
    }
    function selectedRole() {
      const select = document.getElementById('navigation-role-id');
      if (!select || !adminState.bootstrap) return null;
      const value = select.value;
      return ((adminState.bootstrap.roles) || []).find((item) => item.id === value) || null;
    }
    function bindingsForSelectedUser() {
      const user = selectedUser();
      if (!user) return [];
      return adminState.bindings.filter((item) => item.user_id === user.id);
    }
    function renderBindingOptions() {
      const select = document.getElementById('navigation-binding-id');
      const status = document.getElementById('navigation-settings-status');
      if (!select) return;
      const bindings = bindingsForSelectedUser();
      if (!bindings.length) {
        select.innerHTML = '<option value="">' + t('no_bindings') + '</option>';
        select.value = '';
        document.getElementById('navigation-binding-priority').value = '0';
        if (status) status.textContent = t('no_bindings');
        return;
      }
      select.innerHTML = bindings.map((binding) => {
        const role = ((adminState.bootstrap && adminState.bootstrap.roles) || []).find((item) => item.id === binding.role_id);
        const label = (role ? role.name : binding.role_id) + ' · ' + binding.scope_type + (binding.scope_id ? ':' + binding.scope_id : '');
        return '<option value="' + binding.id + '">' + escapeHTML(label) + '</option>';
      }).join('');
      const current = bindings.find((binding) => binding.id === select.value) || bindings[0];
      select.value = current.id;
      document.getElementById('navigation-binding-priority').value = String(current.priority || 0);
      if (status) status.textContent = t('loaded_navigation_settings');
    }
    function syncNavigationForms() {
      const user = selectedUser();
      const role = selectedRole();
      document.getElementById('navigation-preferred-user-route').value = (user && user.preferred_user_route) || '';
      document.getElementById('navigation-preferred-admin-route').value = (user && user.preferred_admin_route) || '';
      document.getElementById('navigation-default-user-route').value = (role && role.default_user_route) || '';
      document.getElementById('navigation-default-admin-route').value = (role && role.default_admin_route) || '';
      renderBindingOptions();
    }
    function renderNavigationSettings() {
      const container = document.getElementById('navigation-settings');
      if (!container) return;
      if (!adminState.navigationManageAllowed) {
        container.innerHTML = '<p class="status">' + escapeHTML(t('manage_users_required')) + '</p>';
        return;
      }
      const users = adminState.users || [];
      const roles = (adminState.bootstrap && adminState.bootstrap.roles) || [];
      const currentUserID = (adminState.bootstrap && adminState.bootstrap.current_user_id) || (users[0] && users[0].id) || '';
      const currentRoleID = (roles[0] && roles[0].id) || '';
      container.innerHTML = ''
        + '<p class="status">' + escapeHTML(t('navigation_defaults_help')) + '</p>'
        + '<div class="admin-grid">'
        +   '<section class="card">'
        +     '<h3>' + escapeHTML(t('users')) + '</h3>'
        +     '<label class="field"><span>' + escapeHTML(t('selected_user')) + '</span><select id="navigation-user-id">'
        +       users.map((user) => '<option value="' + user.id + '"' + (user.id === currentUserID ? ' selected' : '') + '>' + escapeHTML(user.username + ' (' + user.id + ')') + '</option>').join('')
        +     '</select></label>'
        +     '<label class="field"><span>' + escapeHTML(t('preferred_user_route')) + '</span><input id="navigation-preferred-user-route" list="user-route-options" placeholder="/documents"></label>'
        +     '<label class="field"><span>' + escapeHTML(t('preferred_admin_route')) + '</span><input id="navigation-preferred-admin-route" list="admin-route-options" placeholder="/admin/modules"></label>'
        +     '<button id="save-user-navigation">' + escapeHTML(t('save_user_preferences')) + '</button>'
        +   '</section>'
        +   '<section class="card">'
        +     '<h3>' + escapeHTML(t('roles_label')) + '</h3>'
        +     '<label class="field"><span>' + escapeHTML(t('selected_role')) + '</span><select id="navigation-role-id">'
        +       roles.map((role) => '<option value="' + role.id + '"' + (role.id === currentRoleID ? ' selected' : '') + '>' + escapeHTML(role.name + ' (' + role.id + ')') + '</option>').join('')
        +     '</select></label>'
        +     '<label class="field"><span>' + escapeHTML(t('default_user_route')) + '</span><input id="navigation-default-user-route" list="user-route-options" placeholder="/documents"></label>'
        +     '<label class="field"><span>' + escapeHTML(t('default_admin_route')) + '</span><input id="navigation-default-admin-route" list="admin-route-options" placeholder="/admin/modules"></label>'
        +     '<button id="save-role-navigation">' + escapeHTML(t('save_role_defaults')) + '</button>'
        +   '</section>'
        +   '<section class="card">'
        +     '<h3>' + escapeHTML(t('role_bindings')) + '</h3>'
        +     '<label class="field"><span>' + escapeHTML(t('selected_binding')) + '</span><select id="navigation-binding-id"></select></label>'
        +     '<label class="field"><span>' + escapeHTML(t('binding_priority')) + '</span><input id="navigation-binding-priority" type="number" min="0" step="1"></label>'
        +     '<button id="save-binding-priority">' + escapeHTML(t('save_binding_priority')) + '</button>'
        +   '</section>'
        + '</div>'
        + '<datalist id="user-route-options"></datalist>'
        + '<datalist id="admin-route-options"></datalist>';
      renderRouteOptionsDatalist('user-route-options', 'user');
      renderRouteOptionsDatalist('admin-route-options', 'admin');
      syncNavigationForms();
      document.getElementById('navigation-user-id').addEventListener('change', syncNavigationForms);
      document.getElementById('navigation-role-id').addEventListener('change', syncNavigationForms);
      document.getElementById('navigation-binding-id').addEventListener('change', () => {
        const current = bindingsForSelectedUser().find((binding) => binding.id === document.getElementById('navigation-binding-id').value);
        document.getElementById('navigation-binding-priority').value = String((current && current.priority) || 0);
      });
      document.getElementById('save-user-navigation').addEventListener('click', saveUserNavigationPreferences);
      document.getElementById('save-role-navigation').addEventListener('click', saveRoleNavigationDefaults);
      document.getElementById('save-binding-priority').addEventListener('click', saveBindingPriority);
    }
    async function saveUserNavigationPreferences() {
      const user = selectedUser();
      if (!user) return;
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/users/' + encodeURIComponent(user.id) + '/preferences/navigation', {
        method: 'PUT',
        headers: {'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          preferred_user_route: document.getElementById('navigation-preferred-user-route').value,
          preferred_admin_route: document.getElementById('navigation-preferred-admin-route').value
        })
      });
      adminState.users = adminState.users.map((item) => item.id === payload.user.id ? payload.user : item);
      document.getElementById('navigation-settings-status').textContent = t('saved_user_preferences') + ' · ' + user.username;
      syncNavigationForms();
    }
    async function saveRoleNavigationDefaults() {
      const role = selectedRole();
      if (!role) return;
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/roles/' + encodeURIComponent(role.id) + '/defaults/navigation', {
        method: 'PUT',
        headers: {'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          default_user_route: document.getElementById('navigation-default-user-route').value,
          default_admin_route: document.getElementById('navigation-default-admin-route').value
        })
      });
      adminState.bootstrap.roles = ((adminState.bootstrap && adminState.bootstrap.roles) || []).map((item) => item.id === payload.role.id ? payload.role : item);
      document.getElementById('navigation-settings-status').textContent = t('saved_role_defaults') + ' · ' + role.name;
      syncNavigationForms();
    }
    async function saveBindingPriority() {
      const bindingID = document.getElementById('navigation-binding-id').value;
      if (!bindingID) return;
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/role-bindings/' + encodeURIComponent(bindingID) + '/priority', {
        method: 'PUT',
        headers: {'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          priority: parseInt(document.getElementById('navigation-binding-priority').value || '0', 10)
        })
      });
      adminState.bindings = adminState.bindings.map((item) => item.id === payload.binding.id ? payload.binding : item);
      document.getElementById('navigation-settings-status').textContent = t('saved_binding_priority') + ' · ' + bindingID;
      syncNavigationForms();
    }
    function renderPolicyHooks(items) {
      document.getElementById('policy-hooks').innerHTML = items.map((item) => {
        return '<article class="card"><h3>' + escapeHTML(item.key || '') + '</h3><p class="muted">' + escapeHTML(item.kind || '') + ' · ' + escapeHTML(item.target || '') + '</p>' +
          (pickText(item, 'description') ? '<p class="status">' + pickText(item, 'description') + '</p>' : '') +
          '<p><strong>' + t('module_label') + ':</strong> ' + escapeHTML(item.module_key || '-')
          + '</p><p><strong>' + t('target_label') + ':</strong> ' + escapeHTML(item.target || '-')
          + '</p></article>';
      }).join('');
    }
    function renderObservability(payload) {
      payload = payload || {};
      const renderList = (items, kind, textKey) => {
        if (!items || !items.length) return '<p class="status">-</p>';
        return items.map((item) => '<article class="card"><h3>' + escapeHTML(pickText(item, 'title') || item.key || item.type || '') + '</h3><p class="muted">' + escapeHTML(kind) + ' · ' + escapeHTML(item.key || item.type || '') + '</p>' + (pickText(item, 'description') ? '<p class="status">' + escapeHTML(pickText(item, 'description')) + '</p>' : '') + '</article>').join('');
      };
      document.getElementById('observability-contracts').innerHTML =
        '<section class="list"><h3>' + t('dashboards_label') + '</h3>' + renderList(payload.dashboards, 'dashboard', 'dashboards_label') + '</section>' +
        '<section class="list"><h3>' + t('reports_label') + '</h3>' + renderList(payload.reports, 'report', 'reports_label') + '</section>' +
        '<section class="list"><h3>' + t('metrics_label') + '</h3>' + renderList(payload.metrics, 'metric', 'metrics_label') + '</section>' +
        '<section class="list"><h3>' + t('hooks_label') + '</h3>' + renderList(payload.domain_events, 'domain_event', 'hooks_label') + '</section>';
    }
    function isAuthFailureMessage(message) {
      const value = String(message || '').toLowerCase();
      return value.includes('authentication required') ||
        value.includes('session not found') ||
        value.includes('session not active') ||
        value.includes('session revoked') ||
        value.includes('session expired') ||
        value.includes('invalid token signature') ||
        value.includes('invalid session token');
    }
    function getCookie(name) {
      return orbyteGetCookie(name);
    }
    function escapeHTML(value) {
      return orbyteEscapeHTML(value);
    }
    loadAdminShellPrefs();
    bindAdminShellControls();
    window.addEventListener('hashchange', applyAdminRoute);
    document.getElementById('admin-logout-button').addEventListener('click', () => { void logoutAdmin(); });
    boot().catch(err => {
      if (isAuthFailureMessage(err && err.message)) {
        window.location.assign('/ui');
        return;
      }
      document.getElementById('definitions').textContent = String(err);
    });
