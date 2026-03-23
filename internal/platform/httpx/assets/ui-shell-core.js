// Core shell state, locale handling, and chrome controls.
    const offlineDBName = 'orbyte_ui_offline_v1';
    const offlineDBVersion = 1;
    const defaultSupportedLocales = ['en', 'id'];
    const uiMessages = {
      en: {
        shell_brand: 'Orbyte Platform UI',
        shell_subtitle: 'Manifest-driven shell with generic pages and module custom entries.',
        shell_eyebrow: 'Workspace',
        shell_search_placeholder: 'Search pages or jump to a route',
        shell_search_go: 'Go',
        shell_user_chip: 'Signed in',
        navigation: 'Navigation',
        surfaces: 'Surfaces',
        locale_label: 'Language',
        surface_backoffice: 'Backoffice',
        surface_worklist: 'Worklist',
        surface_self_service: 'Self-Service',
        surface_pos: 'POS',
        admin_link: 'Admin',
        notifications: 'Notifications',
        notifications_status: 'Workflow messages and communication history.',
        notifications_empty: 'No notifications yet.',
        mark_read: 'Mark Read',
        dismiss: 'Dismiss',
        open_link: 'Open',
        logout: 'Log out',
        loading: 'Loading…',
        online: 'online',
        offline: 'offline',
        cache_cold: 'cold',
        cache_warm: 'warm',
        route_resolving: 'Resolving module UI registry.',
        using_cached_data: 'Using cached data for',
        login_title: 'Platform Access',
        login_subtitle: 'Sign in to continue.',
        google_button: 'Continue with Google',
        username: 'Username',
        password: 'Password',
        sign_in: 'Sign in',
        sign_in_unavailable: 'No interactive sign-in method is enabled for this deployment.',
        or: 'or',
        view_unavailable: 'View unavailable',
        custom_loading: 'Loading custom module page…',
        search: 'Search',
        sort: 'Sort',
        sort_document: 'Document',
        sort_updated: 'Updated',
        sort_status: 'Status',
        sort_name: 'Name',
        all: 'All',
        new: 'New',
        open: 'Open',
        previous: 'Previous',
        next: 'Next',
        page: 'Page',
        queue_status: 'Status',
        queue_due: 'Due',
        queue_due_any: 'Any due date',
        queue_due_overdue: 'Overdue',
        queue_assignee: 'Assignment',
        queue_assignee_any: 'All assignments',
        queue_assignee_mine: 'Assigned to me',
        queue_save_filter: 'Save Filter',
        queue_reset_filter: 'Reset',
        queue_saved_filter: 'Saved filter restored.',
        queue_filter_saved: 'Worklist filter saved.',
        queue_filter_cleared: 'Worklist filter reset.',
        queue_target: 'Target',
        queue_target_status: 'Target Status',
        queue_workflow: 'Workflow',
        queue_assignment: 'Assignment',
        queue_tasks_label: 'Tasks',
        queue_approvals_label: 'Approvals',
        queue_mine_label: 'Mine',
        queue_requested_by_me: 'Requested By Me',
        queue_workflows_label: 'Workflows',
        queue_actions: 'Actions',
        queue_action_ready: 'Ready',
        workflow_context: 'Workflow Context',
        workflow_history: 'Workflow History',
        workflow_active_tasks: 'Active Tasks',
        workflow_active_approvals: 'Active Approvals',
        workflow_opened_from_queue: 'Opened from queue',
        workflow_back_to_queue: 'Back to Queue',
        standard_list: 'Standard list page rendered from the module manifest.',
        no_records: 'No records yet.',
        select_record: 'Select a record from the list to inspect its canonical record.',
        queue_sync: 'Queue Sync',
        save: 'Save',
        create: 'Create',
        save_local: 'Save Local',
        save_draft: 'Save Draft',
        record_updated: 'Record updated.',
        record_created: 'Record created.',
        draft_saved_local: 'Draft saved locally.',
        draft_queued: 'Draft queued for sync.',
        draft_updated: 'Draft updated through manifest-driven form.',
        ui_bootstrap_failed: 'UI bootstrap failed',
        ui_bootstrap_failed_status: 'Failed to bootstrap module UI.',
        route_not_found: 'Route unavailable',
        route_forbidden: 'Route not allowed',
        surface_mismatch: 'This page belongs to another surface.',
        session_expired: 'Your session expired. Sign in again to continue.',
        recovery_retry: 'Retry',
        recovery_go_default: 'Go to Default',
        recovery_switch_surface: 'Switch Surface',
        recovery_sign_in: 'Sign In',
        recovery_offline: 'Open Cached View',
        keyboard_shortcuts: 'Keyboard Shortcuts',
        shortcut_focus_filters: 'Focus filters',
        shortcut_go_default: 'Go to default page',
        shortcut_open_selected: 'Open current page action',
        density: 'Density',
        density_comfortable: 'Comfortable',
        density_compact: 'Compact',
        columns: 'Columns',
        preferences_saved: 'View preferences saved.',
        no_routes: 'No permitted routes are available for this principal.',
        resolved_from_module: 'Resolved from module',
        using_rendering: 'using',
        sync_pending: 'pending',
        sync_conflict: 'conflict',
        sync_failed: 'failed',
        value_active: 'Active',
        value_inactive: 'Inactive',
        value_blocked: 'Blocked',
        value_draft: 'Draft',
        value_registered: 'Registered',
        value_completed: 'Completed',
        value_submitted: 'Submitted',
        value_approved: 'Approved',
        value_rejected: 'Rejected',
        value_cancelled: 'Cancelled',
        value_failed: 'Failed',
        value_conflict: 'Conflict',
        value_queued: 'Queued',
        value_pending: 'Pending',
        value_enabled: 'Enabled',
        value_disabled: 'Disabled',
        value_true: 'Yes',
        value_false: 'No',
        action_submit: 'Submit',
        action_approve: 'Approve',
        action_reject: 'Reject',
        action_reopen: 'Reopen',
        action_cancel: 'Cancel',
        print_preview: 'Preview',
        print_document: 'Print',
        download_pdf: 'Download PDF',
        template_unavailable: 'No print template is available for this page.',
        print_preview_title: 'Print Preview',
        close_preview: 'Close Preview',
        add: 'Add',
        add_row: 'Add Row',
        edit: 'Edit',
        remove: 'Remove',
        no_related_items: 'No related items.',
        no_related_items_yet: 'No related items yet.',
        related_record_created: 'Related record created.'
      },
      id: {
        shell_brand: 'UI Platform Orbyte',
        shell_subtitle: 'Shell berbasis manifest dengan halaman generik dan entri modul kustom.',
        shell_eyebrow: 'Workspace',
        shell_search_placeholder: 'Cari halaman atau lompat ke rute',
        shell_search_go: 'Buka',
        shell_user_chip: 'Masuk',
        navigation: 'Navigasi',
        surfaces: 'Surface',
        locale_label: 'Bahasa',
        surface_backoffice: 'Backoffice',
        surface_worklist: 'Worklist',
        surface_self_service: 'Layanan Mandiri',
        surface_pos: 'POS',
        admin_link: 'Admin',
        notifications: 'Notifikasi',
        notifications_status: 'Pesan workflow dan riwayat komunikasi.',
        notifications_empty: 'Belum ada notifikasi.',
        mark_read: 'Tandai Dibaca',
        dismiss: 'Sembunyikan',
        open_link: 'Buka',
        logout: 'Keluar',
        loading: 'Memuat…',
        online: 'online',
        offline: 'offline',
        cache_cold: 'dingin',
        cache_warm: 'hangat',
        route_resolving: 'Menyelesaikan registri UI modul.',
        using_cached_data: 'Menggunakan data cache untuk',
        login_title: 'Akses Platform',
        login_subtitle: 'Masuk untuk melanjutkan.',
        google_button: 'Lanjut dengan Google',
        username: 'Nama pengguna',
        password: 'Kata sandi',
        sign_in: 'Masuk',
        sign_in_unavailable: 'Tidak ada metode masuk interaktif yang aktif untuk deployment ini.',
        or: 'atau',
        view_unavailable: 'Tampilan tidak tersedia',
        custom_loading: 'Memuat halaman modul kustom…',
        search: 'Cari',
        sort: 'Urutkan',
        sort_document: 'Dokumen',
        sort_updated: 'Diperbarui',
        sort_status: 'Status',
        sort_name: 'Nama',
        all: 'Semua',
        new: 'Baru',
        open: 'Buka',
        previous: 'Sebelumnya',
        next: 'Berikutnya',
        page: 'Halaman',
        queue_status: 'Status',
        queue_due: 'Jatuh Tempo',
        queue_due_any: 'Semua jatuh tempo',
        queue_due_overdue: 'Lewat jatuh tempo',
        queue_assignee: 'Penugasan',
        queue_assignee_any: 'Semua penugasan',
        queue_assignee_mine: 'Ditugaskan ke saya',
        queue_save_filter: 'Simpan Filter',
        queue_reset_filter: 'Reset',
        queue_saved_filter: 'Filter tersimpan dipulihkan.',
        queue_filter_saved: 'Filter antrian kerja disimpan.',
        queue_filter_cleared: 'Filter antrian kerja direset.',
        queue_target: 'Target',
        queue_target_status: 'Status Target',
        queue_workflow: 'Workflow',
        queue_assignment: 'Penugasan',
        queue_tasks_label: 'Tugas',
        queue_approvals_label: 'Persetujuan',
        queue_mine_label: 'Milik Saya',
        queue_requested_by_me: 'Diminta Oleh Saya',
        queue_workflows_label: 'Workflow',
        queue_actions: 'Aksi',
        queue_action_ready: 'Siap',
        workflow_context: 'Konteks Workflow',
        workflow_history: 'Riwayat Workflow',
        workflow_active_tasks: 'Tugas Aktif',
        workflow_active_approvals: 'Persetujuan Aktif',
        workflow_opened_from_queue: 'Dibuka dari antrian',
        workflow_back_to_queue: 'Kembali ke Antrian',
        standard_list: 'Halaman daftar standar yang dirender dari manifest modul.',
        no_records: 'Belum ada data.',
        select_record: 'Pilih data dari daftar untuk melihat catatan kanonisnya.',
        queue_sync: 'Antrikan Sinkronisasi',
        save: 'Simpan',
        create: 'Buat',
        save_local: 'Simpan Lokal',
        save_draft: 'Simpan Draf',
        record_updated: 'Data diperbarui.',
        record_created: 'Data dibuat.',
        draft_saved_local: 'Draf disimpan secara lokal.',
        draft_queued: 'Draf diantrikan untuk sinkronisasi.',
        draft_updated: 'Draf diperbarui melalui formulir berbasis manifest.',
        ui_bootstrap_failed: 'Bootstrap UI gagal',
        ui_bootstrap_failed_status: 'Gagal melakukan bootstrap UI modul.',
        route_not_found: 'Rute tidak tersedia',
        route_forbidden: 'Rute tidak diizinkan',
        surface_mismatch: 'Halaman ini milik surface lain.',
        session_expired: 'Sesi Anda berakhir. Masuk kembali untuk melanjutkan.',
        recovery_retry: 'Coba Lagi',
        recovery_go_default: 'Ke Rute Default',
        recovery_switch_surface: 'Ganti Surface',
        recovery_sign_in: 'Masuk',
        recovery_offline: 'Buka Versi Cache',
        keyboard_shortcuts: 'Pintasan Keyboard',
        shortcut_focus_filters: 'Fokus ke filter',
        shortcut_go_default: 'Ke halaman default',
        shortcut_open_selected: 'Buka aksi halaman saat ini',
        density: 'Kepadatan',
        density_comfortable: 'Nyaman',
        density_compact: 'Rapat',
        columns: 'Kolom',
        preferences_saved: 'Preferensi tampilan disimpan.',
        no_routes: 'Tidak ada rute yang diizinkan untuk principal ini.',
        resolved_from_module: 'Diselesaikan dari modul',
        using_rendering: 'menggunakan',
        sync_pending: 'tertunda',
        sync_conflict: 'konflik',
        sync_failed: 'gagal',
        value_active: 'Aktif',
        value_inactive: 'Tidak Aktif',
        value_blocked: 'Diblokir',
        value_draft: 'Draf',
        value_registered: 'Terdaftar',
        value_completed: 'Selesai',
        value_submitted: 'Diajukan',
        value_approved: 'Disetujui',
        value_rejected: 'Ditolak',
        value_cancelled: 'Dibatalkan',
        value_failed: 'Gagal',
        value_conflict: 'Konflik',
        value_queued: 'Diantrikan',
        value_pending: 'Tertunda',
        value_enabled: 'Aktif',
        value_disabled: 'Nonaktif',
        value_true: 'Ya',
        value_false: 'Tidak',
        action_submit: 'Ajukan',
        action_approve: 'Setujui',
        action_reject: 'Tolak',
        action_reopen: 'Buka Kembali',
        action_cancel: 'Batalkan',
        print_preview: 'Pratinjau',
        print_document: 'Cetak',
        download_pdf: 'Unduh PDF',
        template_unavailable: 'Tidak ada template cetak untuk halaman ini.',
        print_preview_title: 'Pratinjau Cetak',
        close_preview: 'Tutup Pratinjau',
        add: 'Tambah',
        add_row: 'Tambah Baris',
        edit: 'Ubah',
        remove: 'Hapus',
        no_related_items: 'Belum ada item terkait.',
        no_related_items_yet: 'Belum ada item terkait.',
        related_record_created: 'Data terkait berhasil dibuat.'
      }
    };
    const state = {
      bootstrap: null,
      route: null,
      bundles: {},
      flowTabs: {},
      authOptions: null,
      offlineBootstrap: null,
      syncStats: {pending: 0, conflict: 0, failed: 0},
      cacheWarm: false,
      locale: 'en',
      supportedLocales: defaultSupportedLocales,
      surface: 'backoffice',
      shellKind: 'workspace',
      navCollapsed: false,
      routeState: 'booting',
      routeFocusSelector: '',
      agent: {
        open: false,
        providers: [],
        sessions: [],
        currentSessionId: '',
        attachContext: true,
        stream: null
      }
    };

    function normalizeLocale(locale) {
      return orbyteNormalizeLocale(locale);
    }

    function detectPreferredLocale() {
      if (navigator.languages && navigator.languages.length) return normalizeLocale(navigator.languages[0]);
      return normalizeLocale(navigator.language || 'en');
    }

    function t(key) {
      const locale = state.locale || 'en';
      return (uiMessages[locale] && uiMessages[locale][key]) || (uiMessages.en && uiMessages.en[key]) || key;
    }

    function pickText(item, baseField) {
      if (!item) return '';
      const localized = item[baseField + '_i18n'];
      if (localized && typeof localized === 'object') {
        const current = localized[state.locale];
        if (current) return current;
        if (localized.en) return localized.en;
        if (localized.id) return localized.id;
      }
      return item[baseField] || '';
    }

    function humanizeToken(value) {
      const raw = String(value == null ? '' : value).trim();
      if (!raw) return '';
      if (/[A-Z]/.test(raw) || raw.indexOf(' ') >= 0 || !/^[a-z0-9_-]+$/.test(raw)) return raw;
      return raw.split(/[_-]+/).filter(Boolean).map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ');
    }

    function translateToken(prefix, value) {
      const raw = String(value == null ? '' : value).trim();
      if (!raw) return '';
      const key = prefix + '_' + raw.toLowerCase().replace(/[^a-z0-9]+/g, '_');
      const translated = t(key);
      if (translated !== key) return translated;
      return humanizeToken(raw);
    }

    function displayValue(value) {
      if (value == null) return '';
      if (typeof value === 'boolean') return t(value ? 'value_true' : 'value_false');
      if (typeof value === 'number') return String(value);
      if (typeof value === 'string') return translateToken('value', value);
      return String(value);
    }

    async function persistLocale(locale) {
      try {
        const response = await fetch('/locale?locale=' + encodeURIComponent(locale), {credentials: 'same-origin'});
        if (!response.ok) throw new Error('locale update failed');
        const payload = await response.json();
        state.locale = normalizeLocale(payload.locale || locale);
        state.supportedLocales = payload.supported_locales || state.supportedLocales || defaultSupportedLocales;
      } catch (_) {
        state.locale = normalizeLocale(locale);
      }
    }

    function escapeHTML(value) {
      return orbyteEscapeHTML(value);
    }

    function enhanceControlAccessibility(root) {
      const scope = root || document;
      scope.querySelectorAll('input, select, textarea').forEach((field, index) => {
        if (!field.name) {
          field.name = field.id || field.dataset.path || field.dataset.filter || field.dataset.worklistFilter || field.dataset.columnToggle || ('field_' + index);
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
        if (!labelText && field.dataset.filter) labelText = field.dataset.filter.replace(/_/g, ' ');
        if (!labelText && field.dataset.worklistFilter) labelText = field.dataset.worklistFilter.replace(/_/g, ' ');
        if (!labelText && field.dataset.path) labelText = field.dataset.path.split('.').pop();
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

    function applyLocale() {
      document.documentElement.lang = state.locale;
      document.title = t('shell_brand');
      const eyebrow = document.getElementById('shell-eyebrow');
      const brand = document.getElementById('shell-brand');
      const subtitle = document.getElementById('shell-subtitle');
      const localeLabel = document.getElementById('locale-label');
      const adminLinkButton = document.getElementById('admin-link-button');
      const logoutButton = document.getElementById('logout-button');
      const commandInput = document.getElementById('shell-command-input');
      const commandSubmit = document.getElementById('shell-command-submit');
      const userChip = document.getElementById('shell-user-chip');
      const routeTitle = document.getElementById('route-title');
      const routeStatus = document.getElementById('route-status');
      if (eyebrow) eyebrow.textContent = t('shell_eyebrow');
      if (brand) brand.textContent = t('shell_brand');
      if (subtitle) subtitle.textContent = t('shell_subtitle');
      if (localeLabel) localeLabel.textContent = t('locale_label');
      if (adminLinkButton) adminLinkButton.textContent = t('admin_link');
      if (logoutButton) logoutButton.textContent = t('logout');
      if (commandInput) commandInput.placeholder = t('shell_search_placeholder');
      if (commandSubmit) commandSubmit.textContent = t('shell_search_go');
      if (userChip) userChip.textContent = state.bootstrap && state.bootstrap.auth_context ? (t('shell_user_chip') + ' · ' + (state.bootstrap.auth_context.effective_user_id || state.bootstrap.auth_context.actor_user_id || 'user')) : t('shell_user_chip');
      if (routeTitle && !state.route) routeTitle.innerHTML = '<h2>' + escapeHTML(t('loading')) + '</h2>';
      if (routeStatus && !state.route) routeStatus.textContent = t('route_resolving');
      refreshOfflineStatus();
    }

    function loadShellPrefs() {
      try {
        const stored = window.localStorage.getItem('orbyte.ui.navCollapsed');
        state.navCollapsed = stored == null ? window.innerWidth < 1024 : stored === '1';
      } catch (_) {
        state.navCollapsed = window.innerWidth < 1024;
      }
    }

    function persistShellPrefs() {
      try {
        window.localStorage.setItem('orbyte.ui.navCollapsed', state.navCollapsed ? '1' : '0');
      } catch (_) {}
    }

    function applyWorkspaceNavState() {
      const shell = document.getElementById('shell-root');
      const sidebar = document.getElementById('shell-sidebar');
      const toggle = document.getElementById('shell-nav-toggle');
      const backdrop = document.getElementById('shell-backdrop');
      if (shell) shell.classList.toggle('nav-collapsed', !!state.navCollapsed);
      if (sidebar) sidebar.classList.toggle('open', !state.navCollapsed);
      if (toggle) toggle.setAttribute('aria-expanded', String(!state.navCollapsed));
      if (backdrop) backdrop.hidden = state.navCollapsed || window.innerWidth >= 1024;
    }

    function bindWorkspaceShellControls() {
      const toggle = document.getElementById('shell-nav-toggle');
      const backdrop = document.getElementById('shell-backdrop');
      const commandForm = document.getElementById('shell-command-form');
      const commandInput = document.getElementById('shell-command-input');
      if (toggle && !toggle.dataset.bound) {
        toggle.dataset.bound = '1';
        toggle.addEventListener('click', () => {
          state.navCollapsed = !state.navCollapsed;
          persistShellPrefs();
          applyWorkspaceNavState();
        });
      }
      if (backdrop && !backdrop.dataset.bound) {
        backdrop.dataset.bound = '1';
        backdrop.addEventListener('click', () => {
          state.navCollapsed = true;
          persistShellPrefs();
          applyWorkspaceNavState();
        });
      }
      if (commandForm && !commandForm.dataset.bound) {
        commandForm.dataset.bound = '1';
        commandForm.addEventListener('submit', (event) => {
          event.preventDefault();
          const raw = (commandInput && commandInput.value || '').trim();
          if (!raw) return;
          const staticRoutes = [];
          staticRoutes.push({label: t('notifications'), route_path: '/notifications'});
          if (state.bootstrap && state.bootstrap.acp && state.bootstrap.acp.enabled) {
            staticRoutes.push({label: 'Agent Workspace', route_path: '/agent/workspace'});
          }
          const match = (((state.bootstrap || {}).actions) || []).find((item) => item.route_path === raw || pickText(item, 'label').toLowerCase() === raw.toLowerCase()) ||
            staticRoutes.find((item) => item.route_path === raw || item.label.toLowerCase() === raw.toLowerCase());
          const target = match ? match.route_path : raw;
          window.location.hash = target.startsWith('#') ? target : ('#' + target);
          if (window.innerWidth < 1024) {
            state.navCollapsed = true;
            persistShellPrefs();
            applyWorkspaceNavState();
          }
        });
      }
      window.addEventListener('resize', applyWorkspaceNavState, {passive: true});
    }

    function renderRouteJumpOptions() {
      const datalist = document.getElementById('shell-route-options');
      if (!datalist) return;
      const options = ((state.bootstrap && state.bootstrap.actions) || [])
        .filter((item) => item && item.route_path)
        .map((item) => '<option value="' + escapeHTML(item.route_path) + '">' + escapeHTML(pickText(item, 'label')) + '</option>');
      options.push('<option value="/notifications">' + escapeHTML(t('notifications')) + '</option>');
      if (state.bootstrap && state.bootstrap.acp && state.bootstrap.acp.enabled) {
        options.push('<option value="/agent/workspace">Agent Workspace</option>');
      }
      datalist.innerHTML = options.join('');
    }

    function applyShellLayout(authenticated) {
      const shell = document.getElementById('shell-root');
      const sidebar = document.getElementById('shell-sidebar');
      const routePanel = document.getElementById('route-panel');
      const content = document.querySelector('.content');
      const shellBar = document.getElementById('shell-bar');
      if (authenticated) {
        if (shell) {
          shell.classList.remove('login-mode');
          shell.classList.add('workspace-mode');
        }
        if (sidebar) sidebar.hidden = false;
        if (routePanel) routePanel.hidden = false;
        if (content) {
          content.classList.remove('login-mode');
          content.classList.add('workspace-mode');
        }
        if (shellBar) shellBar.hidden = false;
        applyWorkspaceNavState();
        return;
      }
      if (shell) {
        shell.classList.remove('workspace-mode');
        shell.classList.add('login-mode');
      }
      if (sidebar) sidebar.hidden = true;
      if (routePanel) routePanel.hidden = true;
      if (content) {
        content.classList.remove('workspace-mode');
        content.classList.add('login-mode');
      }
      if (shellBar) shellBar.hidden = true;
    }

    function renderLocaleSwitcher() {
      const select = document.getElementById('locale-switcher');
      if (!select) return;
      select.innerHTML = (state.supportedLocales || defaultSupportedLocales).map((locale) => {
        const name = locale === 'id' ? 'Bahasa Indonesia' : 'English';
        return '<option value="' + locale + '">' + name + '</option>';
      }).join('');
      select.value = state.locale;
      select.onchange = async () => {
        await persistLocale(select.value);
        applyLocale();
        if (state.bootstrap) {
          await renderRoute();
        } else {
          renderLogin(authErrorFromQuery());
        }
      };
    }
