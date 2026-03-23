// Admin shell chrome, locale strings, and navigation scaffolding.
    const adminMessages = {
      en: {
        admin_title: 'Platform Admin',
        admin_subtitle: 'Modules, scoped configuration, and effective runtime settings.',
        language: 'Language',
        workspace_link: 'Workspace',
        logout: 'Log out',
        modules: 'Modules',
        admin_eyebrow: 'Operations Console',
        admin_search_placeholder: 'Search admin sections or jump to a route',
        admin_search_go: 'Go',
        admin_user_chip: 'Signed in',
        auth_settings: 'Authentication Settings',
        config_editor: 'Config Editor',
        org_chart: 'Org Chart',
        org_chart_subtitle: 'Browse reporting lines, spot gaps, and update managers from one place.',
        hierarchy_explorer: 'Hierarchy Explorer',
        hierarchy_explorer_status: 'Select a user card to inspect the chain and edit reporting lines.',
        reporting_line_editor: 'Reporting Line Editor',
        reporting_line_editor_status: 'Choose a user or line, then save changes.',
        workflow_routing: 'Workflow Routing',
        workflow_routing_subtitle: 'Edit draft transitions and inspect assignment resolution before publishing.',
        workflow_draft_editor: 'Draft Editor',
        workflow_draft_editor_status: 'Update states and assignment rules in a structured table, then save the draft.',
        workflow_routing_inspector: 'Routing Inspector',
        workflow_routing_inspector_status: 'Simulate manager-chain and fallback routing for the selected transition.',
        focus_user: 'Focus User',
        line_status: 'Line Status',
        subject_user: 'Subject User',
        manager_user: 'Manager User',
        relationship_type: 'Relationship Type',
        effective_from: 'Effective From',
        effective_to: 'Effective To',
        workflow_key: 'Workflow',
        workflow_version: 'Version',
        workflow_states: 'States',
        action: 'Action',
        current_state: 'Current State',
        requester: 'Requester',
        previous_approver: 'Previous Approver',
        refresh: 'Refresh',
        new_reporting_line: 'New Reporting Line',
        save_reporting_line: 'Save Reporting Line',
        reset: 'Reset',
        create_draft: 'Create Draft',
        validate: 'Validate',
        publish_draft: 'Publish Draft',
        save_draft: 'Save Draft',
        add_action: 'Add Action',
        simulate_routing: 'Simulate Routing',
        templates: 'Templates',
        template_library: 'Template Library',
        template_definition: 'Template',
        template_binding_scope: 'Binding Scope',
        template_binding_scope_id: 'Binding Scope ID',
        template_purpose: 'Purpose',
        template_channel: 'Channel',
        template_binding_flags: 'Binding Flags',
        template_binding_default: 'Default',
        template_binding_official: 'Official',
        template_paper_preset: 'Paper Preset',
        template_body: 'Template Body',
        template_style: 'Template Style',
        preview_target_id: 'Preview Target ID',
        preview_target_key: 'Preview Target Key',
        preview_mode: 'Preview Mode',
        load_template: 'Load Template',
        save_template_draft: 'Save Draft',
        publish_template_version: 'Publish Current Draft',
        save_template_binding: 'Save Binding',
        preview_template_render: 'Preview',
        template_preview: 'Template Preview',
        template_versions: 'Versions',
        template_bindings: 'Bindings',
        template_palette: 'Block Palette',
        template_canvas: 'Designer Canvas',
        template_canvas_help: 'Drag blocks into header, body, or footer rows.',
        template_inspector: 'Inspector',
        template_expert: 'Expert Source',
        template_add_row: 'Add Row',
        template_add_column: 'Add Column',
        template_section_header: 'Header',
        template_section_body: 'Body',
        template_section_footer: 'Footer',
        template_block_text: 'Text',
        template_block_field: 'Field',
        template_block_table: 'Table',
        template_block_totals: 'Totals',
        template_block_divider: 'Divider',
        template_block_image: 'Image',
        template_block_barcode: 'Barcode',
        template_block_signature: 'Signature',
        template_inspector_empty: 'Select a block to edit it.',
        template_block_label: 'Label',
        template_block_text_prop: 'Text',
        template_block_path: 'Field Path',
        template_block_rows_path: 'Rows Path',
        template_block_span: 'Column Span',
        template_block_align: 'Align',
        template_block_size: 'Font Size',
        template_block_emphasis: 'Emphasis',
        template_block_visible_if: 'Visible If',
        template_block_columns: 'Table Columns',
        template_delete_block: 'Delete Block',
        template_duplicate_block: 'Duplicate Block',
        template_move_up: 'Move Up',
        template_move_down: 'Move Down',
        template_delete_row: 'Delete Row',
        template_add_column_action: 'Add Column',
        template_remove_column: 'Remove Column',
        template_block_value: 'Value',
        template_block_image_url: 'Image URL',
        template_block_alt: 'Image Alt',
        template_block_format: 'Format',
        template_add_column_definition: 'Add Column',
        template_remove_column_definition: 'Remove Column',
        template_no_columns: 'No table columns yet.',
        template_column_label: 'Column Label',
        template_column_path: 'Column Path',
        template_module_default: 'Module Default',
        template_module_default_help: 'No scoped binding is active. Module default resolution will be used.',
        template_binding_effective: 'Effective binding',
        template_binding_overrides_broader: 'Overrides broader scopes',
        template_binding_overrides_deployment: 'Overrides deployment default',
        template_binding_fallback: 'Fallback binding',
        template_preview_sample: 'sample',
        template_preview_live: 'live',
        loaded_template: 'Loaded template',
        saved_template_draft: 'Saved template draft',
        published_template_version: 'Published template version',
        saved_template_binding: 'Saved template binding',
        updated_template_binding: 'Updated template binding',
        definitions: 'Definitions',
        role_templates: 'Role Templates',
        navigation_defaults: 'Navigation Defaults',
        navigation_defaults_help: 'Set landing pages by role, user override, and role-binding priority.',
        users: 'Users',
        roles_label: 'Roles',
        role_bindings: 'Role Bindings',
        selected_user: 'Selected User',
        selected_role: 'Selected Role',
        selected_binding: 'Selected Binding',
        preferred_user_route: 'Preferred Workspace Route',
        preferred_admin_route: 'Preferred Admin Route',
        default_user_route: 'Default Workspace Route',
        default_admin_route: 'Default Admin Route',
        binding_priority: 'Binding Priority',
        save_user_preferences: 'Save User Preferences',
        save_role_defaults: 'Save Role Defaults',
        save_binding_priority: 'Save Binding Priority',
        manage_users_required: 'User navigation settings require manage users permission.',
        no_bindings: 'No bindings for selected user.',
        no_routes: 'No routes available',
        loaded_navigation_settings: 'Loaded navigation settings',
        saved_user_preferences: 'Saved user navigation preferences',
        saved_role_defaults: 'Saved role navigation defaults',
        saved_binding_priority: 'Saved role binding priority',
        policy_hooks: 'Policy Hooks',
        observability: 'Observability Contracts',
        config_key: 'Config Key',
        scope: 'Scope',
        organization: 'Organization',
        location: 'Location',
        value_json: 'Value JSON',
        password_login: 'Password Login',
        google_login: 'Google Login',
        login_title: 'Login Title',
        google_button_label: 'Google Button Label',
        login_subtitle: 'Login Subtitle',
        google_client_id: 'Google Client ID',
        google_client_secret: 'Google Client Secret',
        google_redirect_url: 'Google Redirect URL',
        google_hosted_domain: 'Google Hosted Domain',
        google_auth_url: 'Google Auth URL',
        google_token_url: 'Google Token URL',
        google_jwks_url: 'Google JWKS URL',
        google_issuer: 'Google Issuer',
        google_timeout_seconds: 'Google Timeout Seconds',
        provision_new_users: 'Provision New Users',
        provision_role: 'Provision Role',
        provision_default_location: 'Provision Default Location',
        provision_scope_type: 'Provision Scope Type',
        provision_scope_id: 'Provision Scope ID',
        provision_allowed_domains: 'Provision Allowed Domains',
        load_effective: 'Load Effective',
        save_entry: 'Save Entry',
        load_auth_settings: 'Load Auth Settings',
        save_auth_settings: 'Save Auth Settings',
        default_value: 'Default Value',
        fields_label: 'Fields',
        description_label: 'Description',
        scopes_label: 'Scopes',
        permissions_label: 'Permissions',
        module_label: 'Module',
        target_label: 'Target',
        dashboards_label: 'Dashboards',
        metrics_label: 'Metrics',
        reports_label: 'Reports',
        hooks_label: 'Hooks',
        module_col: 'Module',
        status_col: 'Status',
        deps_col: 'Dependencies',
        none: 'none',
        enabled: 'enabled',
        disabled: 'disabled',
        enable: 'Enable',
        disable: 'Disable',
        default_option: 'default',
        select_role: 'Select role',
        default_location: 'Default location',
        select_organization: 'Select organization',
        select_location: 'Select location',
        deployment_default: 'Deployment default',
        auth_validation_clear: 'No authentication validation issues.',
        loaded_auth_settings: 'Loaded authentication settings from',
        saved_auth_settings: 'Saved authentication settings at',
        loaded_effective: 'Loaded effective value from',
        saved_config: 'Saved'
      },
      id: {
        admin_title: 'Admin Platform',
        admin_subtitle: 'Modul, konfigurasi berscope, dan pengaturan runtime efektif.',
        language: 'Bahasa',
        workspace_link: 'Workspace',
        logout: 'Keluar',
        modules: 'Modul',
        admin_eyebrow: 'Konsol Operasi',
        admin_search_placeholder: 'Cari bagian admin atau lompat ke rute',
        admin_search_go: 'Buka',
        admin_user_chip: 'Masuk',
        auth_settings: 'Pengaturan Autentikasi',
        config_editor: 'Editor Konfigurasi',
        org_chart: 'Bagan Organisasi',
        org_chart_subtitle: 'Telusuri reporting line, temukan gap, dan perbarui atasan dari satu tempat.',
        hierarchy_explorer: 'Penjelajah Hierarki',
        hierarchy_explorer_status: 'Pilih kartu pengguna untuk melihat rantai dan mengedit reporting line.',
        reporting_line_editor: 'Editor Reporting Line',
        reporting_line_editor_status: 'Pilih pengguna atau line, lalu simpan perubahan.',
        workflow_routing: 'Routing Workflow',
        workflow_routing_subtitle: 'Edit transisi draf dan inspeksi resolusi assignment sebelum publikasi.',
        workflow_draft_editor: 'Editor Draf',
        workflow_draft_editor_status: 'Perbarui state dan aturan assignment dalam tabel terstruktur, lalu simpan draf.',
        workflow_routing_inspector: 'Inspector Routing',
        workflow_routing_inspector_status: 'Simulasikan routing manager-chain dan fallback untuk transisi terpilih.',
        focus_user: 'Fokus Pengguna',
        line_status: 'Status Line',
        subject_user: 'Pengguna Subjek',
        manager_user: 'Pengguna Atasan',
        relationship_type: 'Tipe Relasi',
        effective_from: 'Berlaku Dari',
        effective_to: 'Berlaku Sampai',
        workflow_key: 'Workflow',
        workflow_version: 'Versi',
        workflow_states: 'State',
        action: 'Aksi',
        current_state: 'State Saat Ini',
        requester: 'Peminta',
        previous_approver: 'Penyetuju Sebelumnya',
        refresh: 'Muat Ulang',
        new_reporting_line: 'Line Baru',
        save_reporting_line: 'Simpan Reporting Line',
        reset: 'Reset',
        create_draft: 'Buat Draf',
        validate: 'Validasi',
        publish_draft: 'Publikasikan Draf',
        save_draft: 'Simpan Draf',
        add_action: 'Tambah Aksi',
        simulate_routing: 'Simulasikan Routing',
        templates: 'Template',
        template_library: 'Pustaka Template',
        template_definition: 'Template',
        template_binding_scope: 'Scope Binding',
        template_binding_scope_id: 'ID Scope Binding',
        template_purpose: 'Tujuan',
        template_channel: 'Channel',
        template_binding_flags: 'Flag Binding',
        template_binding_default: 'Default',
        template_binding_official: 'Resmi',
        template_paper_preset: 'Preset Kertas',
        template_body: 'Isi Template',
        template_style: 'Gaya Template',
        preview_target_id: 'ID Target Preview',
        preview_target_key: 'Kunci Target Preview',
        preview_mode: 'Mode Pratinjau',
        load_template: 'Muat Template',
        save_template_draft: 'Simpan Draf',
        publish_template_version: 'Publikasikan Draf Saat Ini',
        save_template_binding: 'Simpan Binding',
        preview_template_render: 'Pratinjau',
        template_preview: 'Pratinjau Template',
        template_versions: 'Versi',
        template_bindings: 'Binding',
        template_palette: 'Palet Blok',
        template_canvas: 'Kanvas Desainer',
        template_canvas_help: 'Seret blok ke baris header, body, atau footer.',
        template_inspector: 'Inspector',
        template_expert: 'Sumber Ahli',
        template_add_row: 'Tambah Baris',
        template_add_column: 'Tambah Kolom',
        template_section_header: 'Header',
        template_section_body: 'Body',
        template_section_footer: 'Footer',
        template_block_text: 'Teks',
        template_block_field: 'Field',
        template_block_table: 'Tabel',
        template_block_totals: 'Total',
        template_block_divider: 'Pemisah',
        template_block_image: 'Gambar',
        template_block_barcode: 'Barcode',
        template_block_signature: 'Tanda Tangan',
        template_inspector_empty: 'Pilih blok untuk mengeditnya.',
        template_block_label: 'Label',
        template_block_text_prop: 'Teks',
        template_block_path: 'Path Field',
        template_block_rows_path: 'Path Rows',
        template_block_span: 'Span Kolom',
        template_block_align: 'Perataan',
        template_block_size: 'Ukuran Font',
        template_block_emphasis: 'Penekanan',
        template_block_visible_if: 'Visible If',
        template_block_columns: 'Kolom Tabel',
        template_delete_block: 'Hapus Blok',
        template_duplicate_block: 'Duplikasi Blok',
        template_move_up: 'Naikkan',
        template_move_down: 'Turunkan',
        template_delete_row: 'Hapus Baris',
        template_add_column_action: 'Tambah Kolom',
        template_remove_column: 'Hapus Kolom',
        template_block_value: 'Nilai',
        template_block_image_url: 'URL Gambar',
        template_block_alt: 'Alt Gambar',
        template_block_format: 'Format',
        template_add_column_definition: 'Tambah Kolom',
        template_remove_column_definition: 'Hapus Definisi Kolom',
        template_no_columns: 'Belum ada kolom tabel.',
        template_column_label: 'Label Kolom',
        template_column_path: 'Path Kolom',
        template_module_default: 'Default Modul',
        template_module_default_help: 'Tidak ada binding scope yang aktif. Resolusi akan memakai default modul.',
        template_binding_effective: 'Binding efektif',
        template_binding_overrides_broader: 'Menimpa scope yang lebih luas',
        template_binding_overrides_deployment: 'Menimpa default deployment',
        template_binding_fallback: 'Binding fallback',
        template_preview_sample: 'sample',
        template_preview_live: 'live',
        loaded_template: 'Memuat template',
        saved_template_draft: 'Menyimpan draf template',
        published_template_version: 'Mempublikasikan versi template',
        saved_template_binding: 'Menyimpan binding template',
        updated_template_binding: 'Memperbarui binding template',
        definitions: 'Definisi',
        role_templates: 'Template Peran',
        navigation_defaults: 'Default Navigasi',
        navigation_defaults_help: 'Atur landing page berdasarkan peran, override pengguna, dan prioritas binding peran.',
        users: 'Pengguna',
        roles_label: 'Peran',
        role_bindings: 'Binding Peran',
        selected_user: 'Pengguna Terpilih',
        selected_role: 'Peran Terpilih',
        selected_binding: 'Binding Terpilih',
        preferred_user_route: 'Route Workspace Pilihan',
        preferred_admin_route: 'Route Admin Pilihan',
        default_user_route: 'Route Workspace Default',
        default_admin_route: 'Route Admin Default',
        binding_priority: 'Prioritas Binding',
        save_user_preferences: 'Simpan Preferensi Pengguna',
        save_role_defaults: 'Simpan Default Peran',
        save_binding_priority: 'Simpan Prioritas Binding',
        manage_users_required: 'Pengaturan navigasi pengguna memerlukan izin kelola pengguna.',
        no_bindings: 'Tidak ada binding untuk pengguna terpilih.',
        no_routes: 'Tidak ada route tersedia',
        loaded_navigation_settings: 'Memuat pengaturan navigasi',
        saved_user_preferences: 'Menyimpan preferensi navigasi pengguna',
        saved_role_defaults: 'Menyimpan default navigasi peran',
        saved_binding_priority: 'Menyimpan prioritas binding peran',
        policy_hooks: 'Policy Hook',
        observability: 'Kontrak Observabilitas',
        config_key: 'Kunci Konfigurasi',
        scope: 'Cakupan',
        organization: 'Organisasi',
        location: 'Lokasi',
        value_json: 'JSON Nilai',
        password_login: 'Login Kata Sandi',
        google_login: 'Login Google',
        login_title: 'Judul Login',
        google_button_label: 'Label Tombol Google',
        login_subtitle: 'Subjudul Login',
        google_client_id: 'Google Client ID',
        google_client_secret: 'Google Client Secret',
        google_redirect_url: 'URL Redirect Google',
        google_hosted_domain: 'Domain Hosted Google',
        google_auth_url: 'URL Auth Google',
        google_token_url: 'URL Token Google',
        google_jwks_url: 'URL JWKS Google',
        google_issuer: 'Issuer Google',
        google_timeout_seconds: 'Detik Timeout Google',
        provision_new_users: 'Provision Pengguna Baru',
        provision_role: 'Peran Provision',
        provision_default_location: 'Lokasi Default Provision',
        provision_scope_type: 'Tipe Scope Provision',
        provision_scope_id: 'ID Scope Provision',
        provision_allowed_domains: 'Domain Provision yang Diizinkan',
        load_effective: 'Muat Efektif',
        save_entry: 'Simpan Entri',
        load_auth_settings: 'Muat Pengaturan Auth',
        save_auth_settings: 'Simpan Pengaturan Auth',
        default_value: 'Nilai Default',
        fields_label: 'Field',
        description_label: 'Deskripsi',
        scopes_label: 'Scope',
        permissions_label: 'Izin',
        module_label: 'Modul',
        target_label: 'Target',
        dashboards_label: 'Dashboard',
        metrics_label: 'Metrik',
        reports_label: 'Laporan',
        hooks_label: 'Hook',
        module_col: 'Modul',
        status_col: 'Status',
        deps_col: 'Dependensi',
        none: 'tidak ada',
        enabled: 'aktif',
        disabled: 'nonaktif',
        enable: 'Aktifkan',
        disable: 'Nonaktifkan',
        default_option: 'default',
        select_role: 'Pilih peran',
        default_location: 'Lokasi default',
        select_organization: 'Pilih organisasi',
        select_location: 'Pilih lokasi',
        deployment_default: 'Default deployment',
        auth_validation_clear: 'Tidak ada isu validasi autentikasi.',
        loaded_auth_settings: 'Memuat pengaturan autentikasi dari',
        saved_auth_settings: 'Menyimpan pengaturan autentikasi pada',
        loaded_effective: 'Memuat nilai efektif dari',
        saved_config: 'Menyimpan'
      }
    };
    const adminState = { bootstrap: null, locale: 'en', supportedLocales: ['en', 'id'], navCollapsed: false, users: [], bindings: [], navigationManageAllowed: false, reportingLines: [], hierarchyGraph: {nodes: [], edges: [], summary: {}}, hierarchyChain: [], hierarchySelectedUserID: '', hierarchySelectedLineID: '', workflows: [], workflowVersions: [], workflowCurrent: null, workflowSimulation: null, templateDefinitions: [], templateBindings: [], templateVersions: [], templateFixtures: [], templatePreview: null, templateDesigner: { layout: null, sectionID: 'body', selectedBlockID: '' }, agent: { open: false, providers: [], sessions: [], currentSessionId: '', attachContext: true, stream: null } };
    function normalizeLocale(locale) {
      return orbyteNormalizeLocale(locale);
    }
    function t(key) {
      return (adminMessages[adminState.locale] && adminMessages[adminState.locale][key]) || adminMessages.en[key] || key;
    }
    function pickText(item, baseField) {
      if (!item) return '';
      const localized = item[baseField + '_i18n'];
      if (localized && typeof localized === 'object') {
        return localized[adminState.locale] || localized.en || localized.id || item[baseField] || '';
      }
      return item[baseField] || '';
    }
    function enhanceAdminControlAccessibility(root) {
      const scope = root || document;
      scope.querySelectorAll('input, select, textarea').forEach((field, index) => {
        if (!field.name) {
          field.name = field.id || field.dataset.workflowField || field.dataset.templateColumnLabel || field.dataset.templateColumnPath || ('admin_field_' + index);
        }
        if (field.getAttribute('aria-label')) return;
        const wrappingLabel = field.closest('label');
        let labelText = '';
        if (wrappingLabel) labelText = wrappingLabel.textContent || '';
        if (!labelText && field.id) {
          const explicitLabel = scope.querySelector('label[for="' + field.id + '"]');
          if (explicitLabel) labelText = explicitLabel.textContent || '';
        }
        if (!labelText && field.placeholder) labelText = field.placeholder;
        if (!labelText && field.dataset.workflowField) labelText = field.dataset.workflowField.replace(/_/g, ' ');
        if (!labelText && field.dataset.templateColumnLabel != null) labelText = 'template column label';
        if (!labelText && field.dataset.templateColumnPath != null) labelText = 'template column path';
        if (labelText) field.setAttribute('aria-label', labelText.replace(/\s+/g, ' ').trim());
        if (!field.getAttribute('autocomplete') && (field.tagName === 'INPUT' || field.tagName === 'TEXTAREA')) {
          const marker = ((field.id || '') + ' ' + (field.name || '') + ' ' + (field.placeholder || '')).toLowerCase();
          if (field.type === 'password') {
            field.setAttribute('autocomplete', 'current-password');
          } else if (marker.indexOf('user') >= 0 || marker.indexOf('email') >= 0 || marker.indexOf('login') >= 0) {
            field.setAttribute('autocomplete', 'username');
          } else {
            field.setAttribute('autocomplete', 'off');
          }
        }
      });
    }
    function adminCurrentPath() {
      const raw = window.location.hash.replace(/^#/, '').trim();
      return raw || ((adminState.bootstrap && adminState.bootstrap.default_path) || '/admin/modules');
    }
    function loadAdminShellPrefs() {
      try {
        const stored = window.localStorage.getItem('orbyte.admin.navCollapsed');
        adminState.navCollapsed = stored == null ? window.innerWidth < 1024 : stored === '1';
      } catch (_) {
        adminState.navCollapsed = window.innerWidth < 1024;
      }
    }
    function persistAdminShellPrefs() {
      try {
        window.localStorage.setItem('orbyte.admin.navCollapsed', adminState.navCollapsed ? '1' : '0');
      } catch (_) {}
    }
    function applyAdminShellState() {
      const root = document.getElementById('admin-shell-root');
      const sidebar = document.getElementById('admin-sidebar');
      const toggle = document.getElementById('admin-nav-toggle');
      const backdrop = document.getElementById('admin-shell-backdrop');
      if (root) root.classList.toggle('nav-collapsed', !!adminState.navCollapsed);
      if (sidebar) sidebar.classList.toggle('open', !adminState.navCollapsed);
      if (toggle) toggle.setAttribute('aria-expanded', String(!adminState.navCollapsed));
      if (backdrop) backdrop.hidden = adminState.navCollapsed || window.innerWidth >= 1024;
    }
    function bindAdminShellControls() {
      const toggle = document.getElementById('admin-nav-toggle');
      const backdrop = document.getElementById('admin-shell-backdrop');
      const commandForm = document.getElementById('admin-command-form');
      const commandInput = document.getElementById('admin-command-input');
      if (toggle && !toggle.dataset.bound) {
        toggle.dataset.bound = '1';
        toggle.addEventListener('click', () => {
          adminState.navCollapsed = !adminState.navCollapsed;
          persistAdminShellPrefs();
          applyAdminShellState();
        });
      }
      if (backdrop && !backdrop.dataset.bound) {
        backdrop.dataset.bound = '1';
        backdrop.addEventListener('click', () => {
          adminState.navCollapsed = true;
          persistAdminShellPrefs();
          applyAdminShellState();
        });
      }
      if (commandForm && !commandForm.dataset.bound) {
        commandForm.dataset.bound = '1';
        commandForm.addEventListener('submit', (event) => {
          event.preventDefault();
          const raw = (commandInput && commandInput.value || '').trim();
          if (!raw) return;
          const actions = (adminState.bootstrap && adminState.bootstrap.actions) || [];
          const staticMenus = [
            {label: 'Communications', route_path: '/admin/communications'},
            {label: t('org_chart'), route_path: '/admin/org'},
            {label: t('workflow_routing'), route_path: '/admin/workflows'}
          ];
          if (adminState.bootstrap && adminState.bootstrap.acp && adminState.bootstrap.acp.enabled) staticMenus.push({label: 'Agent', route_path: '/admin/agent'});
          const match = actions.find((item) => item.route_path === raw || pickText(item, 'label').toLowerCase() === raw.toLowerCase()) ||
            staticMenus.find((item) => item.route_path === raw || item.label.toLowerCase() === raw.toLowerCase());
          const target = match ? match.route_path : raw;
          window.location.hash = target.startsWith('#') ? target : ('#' + target);
          if (window.innerWidth < 1024) {
            adminState.navCollapsed = true;
            persistAdminShellPrefs();
            applyAdminShellState();
          }
        });
      }
      window.addEventListener('resize', applyAdminShellState, {passive: true});
    }
    function renderAdminRouteOptions() {
      const datalist = document.getElementById('admin-route-options');
      if (!datalist) return;
      const actions = (adminState.bootstrap && adminState.bootstrap.actions) || [];
      const staticMenus = [
        {label: 'Communications', route_path: '/admin/communications'},
        {label: t('org_chart'), route_path: '/admin/org'},
        {label: t('workflow_routing'), route_path: '/admin/workflows'}
      ];
      if (adminState.bootstrap && adminState.bootstrap.acp && adminState.bootstrap.acp.enabled) staticMenus.push({label: 'Agent', route_path: '/admin/agent'});
      datalist.innerHTML = actions.map((item) => '<option value="' + escapeHTML(item.route_path) + '">' + escapeHTML(pickText(item, 'label')) + '</option>').join('') + staticMenus.map((item) => '<option value="' + escapeHTML(item.route_path) + '">' + escapeHTML(item.label) + '</option>').join('');
    }
    function adminNavGroups(items) {
      const groups = [
        {key: 'runtime', label: 'Runtime', routes: ['/admin/modules', '/admin/observability']},
        {key: 'communications', label: 'Communications', routes: ['/admin/communications']},
        {key: 'security', label: 'Security', routes: ['/admin/auth', '/admin/security']},
        {key: 'configuration', label: 'Configuration', routes: ['/admin/config', '/admin/definitions']},
        {key: 'workflow', label: 'Workflow', routes: ['/admin/workflows', '/admin/templates']},
        {key: 'organization', label: 'Organization', routes: ['/admin/org']},
        {key: 'agent', label: 'Agent', routes: ['/admin/agent']}
      ];
      const routeToGroup = (routePath, label) => {
        if (groups.some((group) => group.routes.includes(routePath))) return groups.find((group) => group.routes.includes(routePath)).key;
        const value = ((label || '') + ' ' + (routePath || '')).toLowerCase();
        if (value.indexOf('auth') >= 0 || value.indexOf('security') >= 0) return 'security';
        if (value.indexOf('communication') >= 0 || value.indexOf('notification') >= 0) return 'communications';
        if (value.indexOf('workflow') >= 0 || value.indexOf('template') >= 0) return 'workflow';
        if (value.indexOf('config') >= 0 || value.indexOf('definition') >= 0) return 'configuration';
        if (value.indexOf('org') >= 0) return 'organization';
        if (value.indexOf('agent') >= 0) return 'agent';
        return 'runtime';
      };
      const grouped = {};
      groups.forEach((group) => { grouped[group.key] = []; });
      items.forEach((item) => {
        const key = routeToGroup(item.route_path, item.label);
        grouped[key].push(item);
      });
      return groups.map((group) => ({label: group.label, items: grouped[group.key]})).filter((group) => group.items.length);
    }
    function renderAdminMenus() {
      const container = document.getElementById('admin-nav');
      if (!container) return;
      const menus = (adminState.bootstrap && adminState.bootstrap.menus) || [];
      const actions = (adminState.bootstrap && adminState.bootstrap.actions) || [];
      const path = adminCurrentPath();
      const staticMenus = [
        {label: 'Communications', route_path: '/admin/communications'},
        {label: t('org_chart'), route_path: '/admin/org'},
        {label: t('workflow_routing'), route_path: '/admin/workflows'}
      ];
      if (adminState.bootstrap && adminState.bootstrap.acp && adminState.bootstrap.acp.enabled) {
        staticMenus.push({label: 'Agent', route_path: '/admin/agent'});
      }
      const dynamicMenus = menus.map((menu) => {
        const action = actions.find((item) => item.key === menu.action_key);
        if (!action) return null;
        return {label: pickText(menu, 'label'), route_path: action.route_path};
      }).filter(Boolean).concat(staticMenus);
      container.innerHTML = adminNavGroups(dynamicMenus).map((group) => {
        return '<section class="nav-group"><div class="nav-group-title">' + escapeHTML(group.label) + '</div><div class="nav-group-items">' + group.items.map((item) => {
          const selected = item.route_path === path ? 'true' : 'false';
          const classes = item.route_path === path ? 'admin-tab active' : 'admin-tab';
          return '<a class="' + classes + '" aria-current="' + (item.route_path === path ? 'page' : 'false') + '" href="#' + item.route_path + '">' + escapeHTML(item.label) + '</a>';
        }).join('') + '</div></section>';
      }).join('');
    }
    async function renderAdminCommunications() {
      const root = document.getElementById('admin-communications');
      if (!root) return;
      const payload = await getJSON('/admin/api/notifications?category=' + encodeURIComponent('workflow_approval'));
      const items = payload.items || [];
      root.innerHTML = '<div class="table-shell"><table class="data-table"><thead><tr><th>Message</th><th>User</th><th>Status</th><th>Created</th><th></th></tr></thead><tbody>' + (items.map((item) => {
        const meta = item.metadata || {};
        const approvalID = meta.approval_id || item.target_id || '';
        const recipient = meta.recipient || meta.recipient_user_id || item.user_id || '';
        return '<tr><td><div class="row-primary">' + escapeHTML(item.title || approvalID || item.id) + '</div><div class="row-secondary">' + escapeHTML(item.body || '') + '</div></td><td>' + escapeHTML(recipient) + '</td><td>' + escapeHTML(item.status || 'unread') + '</td><td>' + escapeHTML(item.created_at || '') + '</td><td><div class="toolbar-row">' + (approvalID ? '<button type="button" class="secondary" data-comm-reissue="' + escapeHTML(approvalID) + '">Reissue</button><button type="button" class="secondary" data-comm-revoke="' + escapeHTML(approvalID) + '">Revoke</button><button type="button" class="secondary" data-comm-dispatch="' + escapeHTML(approvalID) + '" data-comm-recipient="' + escapeHTML(recipient) + '">Dispatch Email</button>' : '') + '</div></td></tr>';
      }).join('')) + (items.length ? '' : '<tr><td colspan="5"><div class="empty-state-inline">No workflow communications yet.</div></td></tr>') + '</tbody></table></div>';
      root.querySelectorAll('[data-comm-reissue]').forEach((button) => {
        button.onclick = async () => {
          await getJSON('/ops/workflow/approvals/' + encodeURIComponent(button.dataset.commReissue || '') + '/communication/actions/reissue', {method: 'POST', headers: {'X-CSRF-Token': getCookie('orbyte_csrf')}});
          await renderAdminCommunications();
        };
      });
      root.querySelectorAll('[data-comm-revoke]').forEach((button) => {
        button.onclick = async () => {
          await getJSON('/ops/workflow/approvals/' + encodeURIComponent(button.dataset.commRevoke || '') + '/communication/actions/revoke', {method: 'POST', headers: {'X-CSRF-Token': getCookie('orbyte_csrf')}});
          await renderAdminCommunications();
        };
      });
      root.querySelectorAll('[data-comm-dispatch]').forEach((button) => {
        button.onclick = async () => {
          const recipient = button.dataset.commRecipient || '';
          const suffix = recipient ? ('?recipient=' + encodeURIComponent(recipient)) : '';
          await getJSON('/ops/workflow/approvals/' + encodeURIComponent(button.dataset.commDispatch || '') + '/communication/actions/dispatch-email' + suffix, {method: 'POST', headers: {'X-CSRF-Token': getCookie('orbyte_csrf')}});
          await renderAdminCommunications();
        };
      });
    }
    function applyAdminRoute() {
      const path = adminCurrentPath();
      document.querySelectorAll('[data-admin-route]').forEach((node) => {
        node.style.display = node.dataset.adminRoute === path ? '' : 'none';
      });
      renderAdminMenus();
      if (path === '/admin/communications') void renderAdminCommunications();
      if (path === '/admin/agent') renderAdminAgentWorkspace();
      renderAdminAgentPanel();
      enhanceAdminControlAccessibility(document.querySelector('[data-admin-route="' + path + '"]') || document);
    }
    async function persistLocale(locale) {
      try {
        const payload = await getJSON('/locale?locale=' + encodeURIComponent(locale));
        adminState.locale = normalizeLocale(payload.locale || locale);
        adminState.supportedLocales = payload.supported_locales || adminState.supportedLocales || ['en', 'id'];
      } catch (_) {
        adminState.locale = normalizeLocale(locale);
      }
    }
    async function logoutAdmin() {
      const csrf = getCookie('orbyte_csrf');
      try {
        await fetch('/auth/logout', {
          method: 'POST',
          credentials: 'same-origin',
          headers: csrf ? {'X-CSRF-Token': csrf} : {}
        });
      } catch (_) {}
      resetAdminAgentState();
      window.location.assign('/ui');
    }
    function renderLocaleSwitcher() {
      const select = document.getElementById('admin-locale-switcher');
      if (!select) return;
      select.innerHTML = (adminState.supportedLocales || ['en', 'id']).map((locale) => '<option value="' + locale + '">' + (locale === 'id' ? 'Bahasa Indonesia' : 'English') + '</option>').join('');
      select.value = adminState.locale;
      select.onchange = async () => {
        await persistLocale(select.value);
        renderAdminChrome();
        if (adminState.bootstrap) boot();
      };
    }
    function renderAdminChrome() {
      document.documentElement.lang = adminState.locale;
      document.title = t('admin_title');
      const eyebrow = document.getElementById('admin-eyebrow');
      const commandInput = document.getElementById('admin-command-input');
      const commandSubmit = document.getElementById('admin-command-submit');
      const userChip = document.getElementById('admin-user-chip');
      if (eyebrow) eyebrow.textContent = t('admin_eyebrow');
      if (commandInput) commandInput.placeholder = t('admin_search_placeholder');
      if (commandSubmit) commandSubmit.textContent = t('admin_search_go');
      if (userChip) userChip.textContent = t('admin_user_chip') + ' · ' + (((adminState.bootstrap || {}).current_user_id) || 'admin');
      const pairs = {
        'admin-title': 'admin_title',
        'admin-subtitle': 'admin_subtitle',
        'admin-locale-label': 'language',
        'admin-ui-link': 'workspace_link',
        'admin-logout-button': 'logout',
        'modules-heading': 'modules',
        'auth-heading': 'auth_settings',
        'config-heading': 'config_editor',
        'org-heading': 'org_chart',
        'org-subtitle': 'org_chart_subtitle',
        'org-location-label': 'location',
        'org-status-label': 'line_status',
        'org-user-label': 'focus_user',
        'org-refresh': 'refresh',
        'org-new-line': 'new_reporting_line',
        'org-chart-heading': 'hierarchy_explorer',
        'org-chart-status': 'hierarchy_explorer_status',
        'org-editor-heading': 'reporting_line_editor',
        'org-editor-status': 'reporting_line_editor_status',
        'org-form-subject-label': 'subject_user',
        'org-form-manager-label': 'manager_user',
        'org-form-type-label': 'relationship_type',
        'org-form-status-label': 'status_col',
        'org-form-priority-label': 'binding_priority',
        'org-form-location-label': 'location',
        'org-form-effective-from-label': 'effective_from',
        'org-form-effective-to-label': 'effective_to',
        'org-save-line': 'save_reporting_line',
        'org-reset-line': 'reset',
        'workflow-heading': 'workflow_routing',
        'workflow-subtitle': 'workflow_routing_subtitle',
        'workflow-key-label': 'workflow_key',
        'workflow-version-label': 'workflow_version',
        'workflow-create-draft': 'create_draft',
        'workflow-validate': 'validate',
        'workflow-publish': 'publish_draft',
        'workflow-editor-heading': 'workflow_draft_editor',
        'workflow-editor-status': 'workflow_draft_editor_status',
        'workflow-states-label': 'workflow_states',
        'workflow-add-action': 'add_action',
        'workflow-save-draft': 'save_draft',
        'workflow-inspector-heading': 'workflow_routing_inspector',
        'workflow-inspector-status': 'workflow_routing_inspector_status',
        'workflow-sim-state-label': 'current_state',
        'workflow-sim-action-label': 'action',
        'workflow-sim-requester-label': 'requester',
        'workflow-sim-previous-approver-label': 'previous_approver',
        'workflow-sim-location-label': 'location',
        'workflow-simulate': 'simulate_routing',
        'templates-heading': 'template_library',
        'template-definition-label': 'template_definition',
        'template-binding-scope-label': 'template_binding_scope',
        'template-binding-scope-id-label': 'template_binding_scope_id',
        'template-purpose-label': 'template_purpose',
        'template-channel-label': 'template_channel',
        'template-binding-flags-label': 'template_binding_flags',
        'template-binding-default-label': 'template_binding_default',
        'template-binding-official-label': 'template_binding_official',
        'template-paper-preset-label': 'template_paper_preset',
        'template-body-label': 'template_body',
        'template-style-label': 'template_style',
        'template-render-target-label': 'preview_target_id',
        'template-report-key-label': 'preview_target_key',
        'template-render-mode-label': 'preview_mode',
        'load-template-definition': 'load_template',
        'save-template-draft': 'save_template_draft',
        'publish-template-version': 'publish_template_version',
        'save-template-binding': 'save_template_binding',
        'preview-template-render': 'preview_template_render',
        'template-preview-heading': 'template_preview',
        'template-versions-heading': 'template_versions',
        'template-bindings-heading': 'template_bindings',
        'template-palette-heading': 'template_palette',
        'template-canvas-heading': 'template_canvas',
        'template-canvas-status': 'template_canvas_help',
        'template-inspector-heading': 'template_inspector',
        'template-expert-heading': 'template_expert',
        'template-add-row': 'template_add_row',
        'template-add-column': 'template_add_column',
        'definitions-heading': 'definitions',
        'navigation-heading': 'navigation_defaults',
        'role-templates-heading': 'role_templates',
        'policy-hooks-heading': 'policy_hooks',
        'observability-heading': 'observability',
        'config-key-label': 'config_key',
        'config-scope-label': 'scope',
        'organization-label': 'organization',
        'location-label': 'location',
        'config-value-label': 'value_json',
        'label-auth-password-enabled': 'password_login',
        'label-auth-google-enabled': 'google_login',
        'label-auth-login-title': 'login_title',
        'label-auth-google-button-label': 'google_button_label',
        'label-auth-login-subtitle': 'login_subtitle',
        'label-auth-google-client-id': 'google_client_id',
        'label-auth-google-client-secret': 'google_client_secret',
        'label-auth-google-redirect-url': 'google_redirect_url',
        'label-auth-google-hosted-domain': 'google_hosted_domain',
        'label-auth-google-auth-url': 'google_auth_url',
        'label-auth-google-token-url': 'google_token_url',
        'label-auth-google-jwks-url': 'google_jwks_url',
        'label-auth-google-issuer': 'google_issuer',
        'label-auth-google-timeout-seconds': 'google_timeout_seconds',
        'label-auth-google-auto-provision-enabled': 'provision_new_users',
        'label-auth-google-auto-provision-role-id': 'provision_role',
        'label-auth-google-auto-provision-default-location-id': 'provision_default_location',
        'label-auth-google-auto-provision-scope-type': 'provision_scope_type',
        'label-auth-google-auto-provision-scope-id': 'provision_scope_id',
        'label-auth-google-auto-provision-allowed-domains': 'provision_allowed_domains',
        'load-effective': 'load_effective',
        'save-config': 'save_entry',
        'load-auth-settings': 'load_auth_settings',
        'save-auth-settings': 'save_auth_settings',
        'admin-locale-label': 'language'
      };
      Object.keys(pairs).forEach((id) => {
        const node = document.getElementById(id);
        if (node) node.textContent = t(pairs[id]);
      });
      const boolSelects = ['auth-password-enabled', 'auth-google-enabled', 'auth-google-auto-provision-enabled'];
      boolSelects.forEach((id) => {
        const select = document.getElementById(id);
        if (!select || !select.options || select.options.length < 2) return;
        select.options[0].textContent = t('enabled');
        select.options[1].textContent = t('disabled');
      });
      const scopeType = document.getElementById('auth-google-auto-provision-scope-type');
      if (scopeType && scopeType.options.length >= 3) {
        scopeType.options[0].textContent = 'deployment';
        scopeType.options[1].textContent = t('organization');
        scopeType.options[2].textContent = t('location');
      }
      const configScope = document.getElementById('config-scope');
      if (configScope && configScope.options.length >= 3) {
        configScope.options[0].textContent = 'deployment';
        configScope.options[1].textContent = t('organization');
        configScope.options[2].textContent = t('location');
      }
    }
