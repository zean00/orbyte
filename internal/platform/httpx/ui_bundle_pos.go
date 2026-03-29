package httpx

func POSTerminalBundle() string {
	return `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  window.ClinicModuleBundles["pos-terminal"] = {
    render: async function(ctx) {
      const text = function(en, id) { return ctx.locale === "id" ? id : en; };
      const storageKey = "orbyte:pos-terminal";
      const mount = ctx.mount;
      const params = ctx.params || {};
      const state = {
        bootstrap: null,
        storeCode: params.store_code || "",
        registerCode: params.register_code || "",
        shiftId: "",
        searchQuery: "",
        searchResults: [],
        cart: [],
        tenders: [{ tender_type_code: "", amount: 0, reference: "", notes: "" }],
        heldSales: [],
        transactions: [],
        lookupQuery: "",
        busy: false,
        message: "",
      };

      const escapeHTML = function(value) {
        return String(value == null ? "" : value).replace(/[&<>"]/g, function(char) {
          return { "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;" }[char];
        });
      };
      const number = function(value) {
        const parsed = Number(value || 0);
        return Number.isFinite(parsed) ? parsed : 0;
      };
      const money = function(value) {
        return new Intl.NumberFormat(ctx.locale === "id" ? "id-ID" : "en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(number(value));
      };
      const clone = function(value) {
        return JSON.parse(JSON.stringify(value));
      };
      const readCookie = function(name) {
        const match = document.cookie.match(new RegExp('(?:^|; )' + name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '=([^;]*)'));
        return match ? decodeURIComponent(match[1]) : "";
      };
      const emitHardwareEvent = function(name, detail) {
        window.dispatchEvent(new CustomEvent(name, { detail: detail || {} }));
      };
      const notify = function(message, tone) {
        state.message = message || "";
        render();
        window.clearTimeout(notify._timer);
        notify._timer = window.setTimeout(function() {
          state.message = "";
          render();
        }, tone === "error" ? 5000 : 2800);
      };
      const persist = function() {
        try {
          localStorage.setItem(storageKey, JSON.stringify({
            storeCode: state.storeCode,
            registerCode: state.registerCode,
            shiftId: state.shiftId,
            cart: state.cart,
            tenders: state.tenders,
          }));
        } catch (_) {}
      };
      const restore = function() {
        try {
          const raw = localStorage.getItem(storageKey);
          if (!raw) return;
          const payload = JSON.parse(raw);
          if (payload && typeof payload === "object") {
            state.storeCode = state.storeCode || String(payload.storeCode || "");
            state.registerCode = state.registerCode || String(payload.registerCode || "");
            state.shiftId = state.shiftId || String(payload.shiftId || "");
            state.cart = Array.isArray(payload.cart) ? payload.cart : state.cart;
            state.tenders = Array.isArray(payload.tenders) && payload.tenders.length ? payload.tenders : state.tenders;
          }
        } catch (_) {}
      };
      const selectedStore = function() {
        const stores = ((state.bootstrap || {}).stores || []);
        return stores.find(function(item) { return String((item.values || {}).code || "") === state.storeCode; }) || null;
      };
      const selectedRegister = function() {
        const registers = ((state.bootstrap || {}).registers || []);
        return registers.find(function(item) { return String((item.values || {}).code || "") === state.registerCode; }) || null;
      };
      const tenderTypes = function() {
        return ((state.bootstrap || {}).tender_types || []).filter(function(item) {
          return String(((item.values || {}).status || "active")).toLowerCase() === "active";
        });
      };
      const tenderTypeByCode = function(code) {
        return tenderTypes().find(function(item) { return String((item.values || {}).code || "") === String(code || ""); }) || null;
      };
      const totals = function() {
        const subtotal = state.cart.reduce(function(sum, line) { return sum + number(line.line_subtotal || line.unit_price * line.quantity); }, 0);
        const tax = state.cart.reduce(function(sum, line) { return sum + number(line.tax_amount); }, 0);
        const total = state.cart.reduce(function(sum, line) { return sum + number(line.line_total || (line.line_subtotal + line.tax_amount)); }, 0);
        const tendered = state.tenders.reduce(function(sum, line) { return sum + number(line.amount); }, 0);
        return {
          subtotal: subtotal,
          tax: tax,
          total: total,
          tendered: tendered,
          change: Math.max(tendered - total, 0),
          due: Math.max(total - tendered, 0),
        };
      };
      const ensureStyles = function() {
        if (document.getElementById("pos-terminal-styles")) return;
        const style = document.createElement("style");
        style.id = "pos-terminal-styles";
        style.textContent = ""
          + ".pos-terminal { display: grid; gap: 1rem; }"
          + ".pos-terminal__hero { display:flex; flex-wrap:wrap; justify-content:space-between; gap:1rem; padding:1rem 1.2rem; border:1px solid var(--color-line); border-radius:1rem; background:linear-gradient(135deg, color-mix(in srgb, var(--color-accent-soft) 70%, var(--color-surface) 30%), var(--color-surface)); box-shadow:var(--shadow-panel); }"
          + ".pos-terminal__hero h2 { margin:0; font-size:1.55rem; color:var(--color-body); }"
          + ".pos-terminal__hero p { margin:0.35rem 0 0; color:var(--color-muted); }"
          + ".pos-terminal__status { display:grid; grid-template-columns:repeat(auto-fit,minmax(12rem,1fr)); gap:0.8rem; }"
          + ".pos-terminal__card { border:1px solid var(--color-line); border-radius:1rem; background:var(--color-surface); box-shadow:var(--shadow-panel); padding:1rem; }"
          + ".pos-terminal__grid { display:grid; grid-template-columns:minmax(0,1.4fr) minmax(20rem,0.9fr); gap:1rem; align-items:start; }"
          + ".pos-terminal__section-title { margin:0 0 0.75rem; font-size:0.95rem; font-weight:800; letter-spacing:0.08em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__row { display:flex; flex-wrap:wrap; gap:0.75rem; align-items:end; }"
          + ".pos-terminal__field { display:flex; flex-direction:column; gap:0.35rem; min-width:10rem; flex:1; }"
          + ".pos-terminal__field span { font-size:0.74rem; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__field input, .pos-terminal__field select { height:2.6rem; border:1px solid var(--color-line); border-radius:0.8rem; background:var(--color-surface); color:var(--color-body); padding:0 0.85rem; font:inherit; }"
          + ".pos-terminal__search { display:grid; gap:0.75rem; }"
          + ".pos-terminal__button { appearance:none; border:1px solid var(--color-line); border-radius:0.85rem; background:var(--color-surface); color:var(--color-body); padding:0.8rem 1rem; font:inherit; font-weight:700; cursor:pointer; }"
          + ".pos-terminal__button--primary { background:var(--color-accent); border-color:var(--color-accent); color:#fff; }"
          + ".pos-terminal__button--warn { border-color:#d45555; color:#b73a3a; }"
          + ".pos-terminal__button:disabled { opacity:0.55; cursor:not-allowed; }"
          + ".pos-terminal__buttons { display:flex; flex-wrap:wrap; gap:0.6rem; }"
          + ".pos-terminal__list { display:grid; gap:0.65rem; }"
          + ".pos-terminal__item { border:1px solid var(--color-line); border-radius:0.9rem; padding:0.85rem; background:color-mix(in srgb, var(--color-shell) 30%, var(--color-surface)); }"
          + ".pos-terminal__item-head { display:flex; justify-content:space-between; gap:0.75rem; align-items:flex-start; }"
          + ".pos-terminal__item-title { font-weight:700; color:var(--color-body); }"
          + ".pos-terminal__item-meta { color:var(--color-muted); font-size:0.84rem; }"
          + ".pos-terminal__table { width:100%; border-collapse:collapse; }"
          + ".pos-terminal__table th, .pos-terminal__table td { padding:0.7rem 0.5rem; border-top:1px solid var(--color-line); text-align:left; vertical-align:top; }"
          + ".pos-terminal__table th { color:var(--color-muted); font-size:0.72rem; font-weight:800; letter-spacing:0.08em; text-transform:uppercase; }"
          + ".pos-terminal__table td:last-child, .pos-terminal__table th:last-child { text-align:right; }"
          + ".pos-terminal__table input, .pos-terminal__table select { width:100%; min-width:0; height:2.2rem; border:1px solid var(--color-line); border-radius:0.7rem; background:var(--color-surface); color:var(--color-body); padding:0 0.65rem; font:inherit; }"
          + ".pos-terminal__summary { display:grid; gap:0.55rem; }"
          + ".pos-terminal__summary-row { display:flex; justify-content:space-between; gap:1rem; color:var(--color-body); }"
          + ".pos-terminal__summary-row strong { font-size:1.05rem; }"
          + ".pos-terminal__pill { display:inline-flex; align-items:center; gap:0.35rem; padding:0.3rem 0.6rem; border-radius:999px; background:color-mix(in srgb, var(--color-accent) 12%, var(--color-surface)); color:var(--color-accent-dark); font-size:0.74rem; font-weight:800; letter-spacing:0.08em; text-transform:uppercase; }"
          + ".pos-terminal__empty { color:var(--color-muted); text-align:center; padding:1rem; border:1px dashed var(--color-line); border-radius:0.9rem; }"
          + ".pos-terminal__notice { padding:0.75rem 0.9rem; border-radius:0.85rem; background:color-mix(in srgb, var(--color-accent-soft) 55%, var(--color-surface)); color:var(--color-body); }"
          + ".pos-terminal__transactions { max-height:20rem; overflow:auto; }"
          + "@media (max-width: 960px) { .pos-terminal__grid { grid-template-columns:1fr; } }";
        document.head.appendChild(style);
      };

      async function api(path, options) {
        const request = options ? Object.assign({}, options) : {};
        const method = String(request.method || "GET").toUpperCase();
        const headers = Object.assign({}, request.headers || {});
        if (method !== "GET" && method !== "HEAD" && !headers["X-CSRF-Token"]) {
          headers["X-CSRF-Token"] = readCookie("orbyte_csrf");
        }
        request.headers = headers;
        return ctx.api(path, request);
      }

      async function loadBootstrap() {
        const query = new URLSearchParams();
        if (state.storeCode) query.set("store_code", state.storeCode);
        if (state.registerCode) query.set("register_code", state.registerCode);
        state.bootstrap = await api("/ui/data/pos/bootstrap" + (query.toString() ? "?" + query.toString() : ""));
        if (!state.storeCode && state.bootstrap.current_store && state.bootstrap.current_store.values) {
          state.storeCode = String(state.bootstrap.current_store.values.code || "");
        }
        if (!state.shiftId && state.bootstrap.open_shift && state.bootstrap.open_shift.id) {
          state.shiftId = String(state.bootstrap.open_shift.id || "");
        }
      }

      async function searchCatalog() {
        if (!state.storeCode) {
          notify(text("Select a store first.", "Pilih toko terlebih dahulu."), "error");
          return;
        }
        const query = new URLSearchParams();
        query.set("store_code", state.storeCode);
        query.set("q", state.searchQuery);
        const payload = await api("/ui/data/pos/catalog/search?" + query.toString());
        state.searchResults = payload.items || [];
        emitHardwareEvent("orbyte:pos-scanner-input", { query: state.searchQuery, matches: state.searchResults });
        render();
      }

      function addCatalogItem(item) {
        const existing = state.cart.find(function(line) { return String(line.item_code || "") === String(item.item_code || ""); });
        if (existing) {
          existing.quantity = number(existing.quantity) + 1;
          recalcLine(existing);
        } else {
          state.cart.push(recalcLine({
            product_code: item.product_code || "",
            variant_signature: item.variant_signature || "",
            item_code: item.item_code || "",
            description: item.description || item.name || item.item_code || "",
            quantity: 1,
            unit_price: number(item.unit_price),
            tax_code: item.tax_code || "",
            tax_rate: number(item.tax_rate),
            tax_mode: item.tax_mode || "exclusive",
            discount_amount: 0,
            inventory_enabled: !!item.inventory_enabled,
            available_quantity: number(item.available_quantity),
            line_subtotal: 0,
            tax_amount: 0,
            line_total: 0,
          }));
        }
        persist();
        render();
      }

      function recalcLine(line) {
        line.quantity = Math.max(number(line.quantity), 0);
        line.unit_price = number(line.unit_price);
        line.discount_amount = Math.max(number(line.discount_amount), 0);
        line.tax_rate = number(line.tax_rate);
        const gross = Math.max(line.quantity * line.unit_price - line.discount_amount, 0);
        const taxMode = String(line.tax_mode || "exclusive").toLowerCase();
        if (taxMode === "inclusive" && line.tax_rate > 0) {
          line.line_subtotal = Math.round((gross / (1 + line.tax_rate / 100)) * 100) / 100;
          line.tax_amount = Math.round((gross - line.line_subtotal) * 100) / 100;
          line.line_total = Math.round(gross * 100) / 100;
        } else if (taxMode === "exempt") {
          line.line_subtotal = Math.round(gross * 100) / 100;
          line.tax_amount = 0;
          line.line_total = line.line_subtotal;
        } else {
          line.line_subtotal = Math.round(gross * 100) / 100;
          line.tax_amount = Math.round((line.line_subtotal * line.tax_rate / 100) * 100) / 100;
          line.line_total = Math.round((line.line_subtotal + line.tax_amount) * 100) / 100;
        }
        return line;
      }

      function payloadLines() {
        return state.cart.filter(function(line) { return number(line.quantity) > 0; }).map(function(line) {
          return {
            product_code: line.product_code || "",
            variant_signature: line.variant_signature || "",
            item_code: line.item_code || "",
            description: line.description || "",
            quantity: number(line.quantity),
            discount_amount: number(line.discount_amount),
            note: line.note || "",
          };
        });
      }

      function payloadTenders() {
        return state.tenders.filter(function(tender) { return String(tender.tender_type_code || "").trim() !== "" && number(tender.amount) > 0; }).map(function(tender) {
          return {
            tender_type_code: tender.tender_type_code,
            amount: number(tender.amount),
            reference: tender.reference || "",
            notes: tender.notes || "",
          };
        });
      }

      async function openShift() {
        if (!state.storeCode || !state.registerCode) {
          notify(text("Store and register are required.", "Toko dan register wajib diisi."), "error");
          return;
        }
        const opening = Number(window.prompt(text("Opening cash amount", "Jumlah kas awal"), "0") || "0");
        const payload = await api("/ui/data/pos/shifts/open", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            store_code: state.storeCode,
            register_code: state.registerCode,
            opening_cash_amount: Number.isFinite(opening) ? opening : 0,
          }),
        });
        state.shiftId = String(((payload || {}).record || {}).id || "");
        await loadBootstrap();
        persist();
        notify(text("Shift opened.", "Shift dibuka."));
        render();
      }

      async function closeShift() {
        if (!state.shiftId) {
          notify(text("No open shift.", "Tidak ada shift terbuka."), "error");
          return;
        }
        const actual = Number(window.prompt(text("Counted cash amount", "Jumlah kas aktual"), String(totals().tendered || 0)) || "0");
        await api("/ui/data/pos/shifts/" + encodeURIComponent(state.shiftId) + "/close", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ actual_cash_amount: Number.isFinite(actual) ? actual : 0 }),
        });
        state.shiftId = "";
        await loadBootstrap();
        persist();
        notify(text("Shift closed.", "Shift ditutup."));
        render();
      }

      async function loadHeldSales() {
        if (!state.registerCode || !state.shiftId) {
          state.heldSales = [];
          render();
          return;
        }
        const query = new URLSearchParams({ register_code: state.registerCode, shift_id: state.shiftId });
        const payload = await api("/ui/data/pos/sales/held?" + query.toString());
        state.heldSales = payload.items || [];
        render();
      }

      async function holdSale() {
        if (!state.shiftId) {
          notify(text("Open a shift first.", "Buka shift terlebih dahulu."), "error");
          return;
        }
        const payload = await api("/ui/data/pos/sales/hold", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            store_code: state.storeCode,
            register_code: state.registerCode,
            shift_id: state.shiftId,
            lines: payloadLines(),
            tenders: payloadTenders(),
            offline_cached: !navigator.onLine,
          }),
        });
        await loadHeldSales();
        state.cart = [];
        state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
        persist();
        notify(text("Sale held.", "Penjualan disimpan."));
        render();
        return payload;
      }

      function resumeHeldSale(record) {
        try {
          state.cart = JSON.parse(String(((record || {}).values || {}).lines_json || "[]"));
          state.tenders = JSON.parse(String(((record || {}).values || {}).tenders_json || "[{\"tender_type_code\":\"\",\"amount\":0}]"));
          if (!Array.isArray(state.tenders) || !state.tenders.length) {
            state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
          }
          state.cart = state.cart.map(recalcLine);
          persist();
          notify(text("Held sale loaded.", "Penjualan tertahan dimuat."));
          render();
        } catch (_) {
          notify(text("Failed to load held sale.", "Gagal memuat penjualan tertahan."), "error");
        }
      }

      async function checkout() {
        if (!navigator.onLine) {
          notify(text("Checkout requires a live connection.", "Checkout membutuhkan koneksi aktif."), "error");
          return;
        }
        if (!state.shiftId) {
          notify(text("Open a shift before checkout.", "Buka shift sebelum checkout."), "error");
          return;
        }
        if (!payloadLines().length) {
          notify(text("Add at least one line.", "Tambahkan minimal satu baris."), "error");
          return;
        }
        state.busy = true;
        render();
        try {
          const result = await api("/ui/data/pos/checkout", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              store_code: state.storeCode,
              register_code: state.registerCode,
              shift_id: state.shiftId,
              lines: payloadLines(),
              tenders: payloadTenders(),
              offline_cached: false,
            }),
          });
          emitHardwareEvent("orbyte:pos-receipt-print", result);
          emitHardwareEvent("orbyte:pos-cash-drawer-open", { total: totals().total, change: totals().change });
          state.cart = [];
          state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
          persist();
          await loadHeldSales();
          notify(text("Checkout completed.", "Checkout selesai."));
          render();
          if (window.confirm(text("Print receipt now?", "Cetak struk sekarang?"))) {
            window.print();
          }
        } catch (error) {
          notify(error instanceof Error ? error.message : text("Checkout failed.", "Checkout gagal."), "error");
        } finally {
          state.busy = false;
          render();
        }
      }

      async function lookupTransactions() {
        const query = new URLSearchParams();
        if (state.lookupQuery) query.set("q", state.lookupQuery);
        const payload = await api("/ui/data/pos/transactions/search?" + query.toString());
        state.transactions = payload.items || [];
        render();
      }

      async function transactAction(saleID, action) {
        const payload = await api("/ui/data/pos/transactions/" + encodeURIComponent(saleID) + "/" + action, {
          method: "POST",
        });
        notify(action === "refund" ? text("Refund documents created.", "Dokumen refund dibuat.") : text("Exchange documents created.", "Dokumen tukar dibuat."));
        emitHardwareEvent("orbyte:pos-payment-terminal-request", { action: action, payload: payload });
        await lookupTransactions();
      }

      function renderStoreOptions(items, valueKey, labelKey, current) {
        return ['<option value="">' + escapeHTML(text("Select", "Pilih")) + '</option>'].concat((items || []).map(function(item) {
          const values = item.values || {};
          const value = String(values[valueKey] || "");
          const selected = value === current ? ' selected' : '';
          return '<option value="' + escapeHTML(value) + '"' + selected + '>' + escapeHTML(String(values[labelKey] || value)) + '</option>';
        })).join("");
      }

      function render() {
        ensureStyles();
        const totalsPayload = totals();
        const bootstrap = state.bootstrap || { stores: [], registers: [], tender_types: [] };
        mount.innerHTML = ''
          + '<section class="pos-terminal">'
          +   '<section class="pos-terminal__hero">'
          +     '<div><h2>' + escapeHTML(text("POS Terminal", "Terminal POS")) + '</h2><p>' + escapeHTML(text("Counter checkout on top of sales, fulfillment, payment, and returns.", "Checkout kasir di atas sales, fulfillment, payment, dan retur.")) + '</p></div>'
          +     '<div class="pos-terminal__buttons">'
          +       '<button type="button" class="pos-terminal__button" data-action="lookup">' + escapeHTML(text("Refresh", "Muat Ulang")) + '</button>'
          +       '<button type="button" class="pos-terminal__button" data-action="print">' + escapeHTML(text("Print", "Cetak")) + '</button>'
          +     '</div>'
          +   '</section>'
          +   (state.message ? '<div class="pos-terminal__notice">' + escapeHTML(state.message) + '</div>' : '')
          +   '<section class="pos-terminal__status">'
          +     '<article class="pos-terminal__card"><div class="pos-terminal__section-title">' + escapeHTML(text("Store", "Toko")) + '</div><div class="pos-terminal__row"><div class="pos-terminal__field"><span>' + escapeHTML(text("Store", "Toko")) + '</span><select id="pos-store" name="pos_store">' + renderStoreOptions(bootstrap.stores, "code", "name", state.storeCode) + '</select></div><div class="pos-terminal__field"><span>' + escapeHTML(text("Register", "Register")) + '</span><select id="pos-register" name="pos_register">' + renderStoreOptions((bootstrap.registers || []).filter(function(item) { return !state.storeCode || String((item.values || {}).store_code || "") === state.storeCode; }), "code", "name", state.registerCode) + '</select></div></div></article>'
          +     '<article class="pos-terminal__card"><div class="pos-terminal__section-title">' + escapeHTML(text("Shift", "Shift")) + '</div><div class="pos-terminal__row"><div><span class="pos-terminal__pill">' + escapeHTML(state.shiftId ? text("Open", "Terbuka") : text("Closed", "Tutup")) + '</span></div><div class="pos-terminal__buttons">' + (state.shiftId ? '<button type="button" class="pos-terminal__button pos-terminal__button--warn" data-action="close-shift">' + escapeHTML(text("Close Shift", "Tutup Shift")) + '</button>' : '<button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="open-shift">' + escapeHTML(text("Open Shift", "Buka Shift")) + '</button>') + '</div></div></article>'
          +     '<article class="pos-terminal__card"><div class="pos-terminal__section-title">' + escapeHTML(text("Connectivity", "Konektivitas")) + '</div><div class="pos-terminal__row"><span class="pos-terminal__pill">' + escapeHTML(navigator.onLine ? text("Online", "Online") : text("Offline", "Offline")) + '</span><span class="pos-terminal__item-meta">' + escapeHTML(navigator.onLine ? text("Checkout is enabled.", "Checkout aktif.") : text("Browse and hold only.", "Hanya browse dan hold.")) + '</span></div></article>'
          +   '</section>'
          +   '<section class="pos-terminal__grid">'
          +     '<div class="pos-terminal__list">'
          +       '<article class="pos-terminal__card">'
          +         '<div class="pos-terminal__section-title">' + escapeHTML(text("Catalog Search", "Pencarian Katalog")) + '</div>'
          +         '<div class="pos-terminal__search"><div class="pos-terminal__row"><div class="pos-terminal__field"><span>' + escapeHTML(text("Barcode or name", "Barcode atau nama")) + '</span><input id="pos-search" name="pos_search" placeholder="' + escapeHTML(text("Scan barcode or type item name", "Scan barcode atau ketik nama barang")) + '" value="' + escapeHTML(state.searchQuery) + '"></div><div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="search">' + escapeHTML(text("Search", "Cari")) + '</button></div></div>'
          +         (state.searchResults.length ? '<div class="pos-terminal__list">' + state.searchResults.map(function(item, index) {
                      return '<article class="pos-terminal__item"><div class="pos-terminal__item-head"><div><div class="pos-terminal__item-title">' + escapeHTML(item.name || item.item_code) + '</div><div class="pos-terminal__item-meta">' + escapeHTML((item.item_code || "") + " • " + (item.variant_label || "")) + '</div></div><div><strong>' + escapeHTML(money(item.unit_price)) + '</strong></div></div><div class="pos-terminal__item-meta">' + escapeHTML(text("Available", "Tersedia")) + ': ' + escapeHTML(money(item.available_quantity || 0)) + '</div><div class="pos-terminal__buttons" style="margin-top:0.75rem"><button type="button" class="pos-terminal__button" data-add-result="' + String(index) + '">' + escapeHTML(text("Add", "Tambah")) + '</button></div></article>';
                    }).join("") + '</div>' : '<div class="pos-terminal__empty">' + escapeHTML(text("Search results will appear here.", "Hasil pencarian akan muncul di sini.")) + '</div>')
          +         '</div>'
          +       '</article>'
          +       '<article class="pos-terminal__card">'
          +         '<div class="pos-terminal__section-title">' + escapeHTML(text("Held Sales", "Penjualan Tertahan")) + '</div>'
          +         (state.heldSales.length ? '<div class="pos-terminal__list">' + state.heldSales.map(function(item, index) {
                      const values = item.values || {};
                      return '<article class="pos-terminal__item"><div class="pos-terminal__item-head"><div><div class="pos-terminal__item-title">' + escapeHTML(String(values.sale_number || item.id || "")) + '</div><div class="pos-terminal__item-meta">' + escapeHTML(String(values.party_name || text("Walk-in customer", "Pelanggan umum"))) + '</div></div><div><strong>' + escapeHTML(money(values.total_amount || 0)) + '</strong></div></div><div class="pos-terminal__buttons" style="margin-top:0.75rem"><button type="button" class="pos-terminal__button" data-resume-held="' + String(index) + '">' + escapeHTML(text("Resume", "Lanjutkan")) + '</button></div></article>';
                    }).join("") + '</div>' : '<div class="pos-terminal__empty">' + escapeHTML(text("No held sales.", "Tidak ada penjualan tertahan.")) + '</div>')
          +       '</article>'
          +       '<article class="pos-terminal__card">'
          +         '<div class="pos-terminal__section-title">' + escapeHTML(text("Transaction Lookup", "Pencarian Transaksi")) + '</div>'
          +         '<div class="pos-terminal__row"><div class="pos-terminal__field"><span>' + escapeHTML(text("Search", "Cari")) + '</span><input id="pos-lookup" name="pos_lookup" value="' + escapeHTML(state.lookupQuery) + '" placeholder="' + escapeHTML(text("Sale number, customer, invoice", "Nomor jual, pelanggan, invoice")) + '"></div><div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="lookup-transactions">' + escapeHTML(text("Lookup", "Cari")) + '</button></div></div>'
          +         '<div class="pos-terminal__transactions">' + (state.transactions.length ? state.transactions.map(function(item, index) {
                      const values = item.values || {};
                      return '<article class="pos-terminal__item" style="margin-top:0.75rem"><div class="pos-terminal__item-head"><div><div class="pos-terminal__item-title">' + escapeHTML(String(values.sale_number || "")) + '</div><div class="pos-terminal__item-meta">' + escapeHTML(String(values.invoice_number || values.order_number || "")) + " • " + escapeHTML(String(values.party_name || "")) + '</div></div><div><strong>' + escapeHTML(money(values.total_amount || 0)) + '</strong></div></div><div class="pos-terminal__buttons" style="margin-top:0.75rem"><button type="button" class="pos-terminal__button" data-refund-sale="' + String(index) + '">' + escapeHTML(text("Refund", "Refund")) + '</button><button type="button" class="pos-terminal__button" data-exchange-sale="' + String(index) + '">' + escapeHTML(text("Exchange", "Tukar")) + '</button></div></article>';
                    }).join("") : '<div class="pos-terminal__empty" style="margin-top:0.75rem">' + escapeHTML(text("No transactions loaded.", "Belum ada transaksi dimuat.")) + '</div>') + '</div>'
          +       '</article>'
          +     '</div>'
          +     '<div class="pos-terminal__list">'
          +       '<article class="pos-terminal__card">'
          +         '<div class="pos-terminal__section-title">' + escapeHTML(text("Cart", "Keranjang")) + '</div>'
          +         (state.cart.length ? '<table class="pos-terminal__table"><thead><tr><th>' + escapeHTML(text("Item", "Item")) + '</th><th>' + escapeHTML(text("Qty", "Qty")) + '</th><th>' + escapeHTML(text("Price", "Harga")) + '</th><th>' + escapeHTML(text("Discount", "Diskon")) + '</th><th>' + escapeHTML(text("Total", "Total")) + '</th><th></th></tr></thead><tbody>' + state.cart.map(function(line, index) {
                      return '<tr><td><div class="pos-terminal__item-title">' + escapeHTML(line.description || line.item_code) + '</div><div class="pos-terminal__item-meta">' + escapeHTML(String(line.item_code || "")) + '</div></td><td><input id="pos-line-qty-' + String(index) + '" name="pos_line_qty_' + String(index) + '" type="number" min="0" step="1" data-line-qty="' + String(index) + '" value="' + escapeHTML(String(line.quantity || 0)) + '"></td><td><input id="pos-line-price-' + String(index) + '" name="pos_line_price_' + String(index) + '" type="number" min="0" step="0.01" data-line-price="' + String(index) + '" value="' + escapeHTML(String(line.unit_price || 0)) + '"></td><td><input id="pos-line-discount-' + String(index) + '" name="pos_line_discount_' + String(index) + '" type="number" min="0" step="0.01" data-line-discount="' + String(index) + '" value="' + escapeHTML(String(line.discount_amount || 0)) + '"></td><td><strong>' + escapeHTML(money(line.line_total || 0)) + '</strong></td><td><button type="button" class="pos-terminal__button pos-terminal__button--warn" data-remove-line="' + String(index) + '">' + escapeHTML(text("Remove", "Hapus")) + '</button></td></tr>';
                    }).join("") + '</tbody></table>' : '<div class="pos-terminal__empty">' + escapeHTML(text("No items in cart.", "Belum ada item di keranjang.")) + '</div>')
          +       '</article>'
          +       '<article class="pos-terminal__card">'
          +         '<div class="pos-terminal__section-title">' + escapeHTML(text("Tenders", "Tender")) + '</div>'
          +         '<table class="pos-terminal__table"><thead><tr><th>' + escapeHTML(text("Tender", "Tender")) + '</th><th>' + escapeHTML(text("Amount", "Jumlah")) + '</th><th>' + escapeHTML(text("Reference", "Referensi")) + '</th><th></th></tr></thead><tbody>' + state.tenders.map(function(line, index) {
                    return '<tr><td><select id="pos-tender-type-' + String(index) + '" name="pos_tender_type_' + String(index) + '" data-tender-type="' + String(index) + '">' + ['<option value="">' + escapeHTML(text("Select", "Pilih")) + '</option>'].concat(tenderTypes().map(function(item) {
                      const code = String((item.values || {}).code || "");
                      const selected = code === String(line.tender_type_code || "") ? ' selected' : '';
                      return '<option value="' + escapeHTML(code) + '"' + selected + '>' + escapeHTML(String((item.values || {}).name || code)) + '</option>';
                    })).join("") + '</select></td><td><input id="pos-tender-amount-' + String(index) + '" name="pos_tender_amount_' + String(index) + '" type="number" min="0" step="0.01" data-tender-amount="' + String(index) + '" value="' + escapeHTML(String(line.amount || 0)) + '"></td><td><input id="pos-tender-reference-' + String(index) + '" name="pos_tender_reference_' + String(index) + '" type="text" data-tender-reference="' + String(index) + '" value="' + escapeHTML(String(line.reference || "")) + '"></td><td><button type="button" class="pos-terminal__button" data-remove-tender="' + String(index) + '">' + escapeHTML(text("Remove", "Hapus")) + '</button></td></tr>';
                  }).join("") + '</tbody></table><div class="pos-terminal__buttons" style="margin-top:0.75rem"><button type="button" class="pos-terminal__button" data-action="add-tender">' + escapeHTML(text("Add Tender", "Tambah Tender")) + '</button></div>'
          +       '</article>'
          +       '<article class="pos-terminal__card">'
          +         '<div class="pos-terminal__section-title">' + escapeHTML(text("Summary", "Ringkasan")) + '</div>'
          +         '<div class="pos-terminal__summary">'
          +           '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Subtotal", "Subtotal")) + '</span><span>' + escapeHTML(money(totalsPayload.subtotal)) + '</span></div>'
          +           '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Tax", "Pajak")) + '</span><span>' + escapeHTML(money(totalsPayload.tax)) + '</span></div>'
          +           '<div class="pos-terminal__summary-row"><strong>' + escapeHTML(text("Total", "Total")) + '</strong><strong>' + escapeHTML(money(totalsPayload.total)) + '</strong></div>'
          +           '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Tendered", "Dibayar")) + '</span><span>' + escapeHTML(money(totalsPayload.tendered)) + '</span></div>'
          +           '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Change", "Kembalian")) + '</span><span>' + escapeHTML(money(totalsPayload.change)) + '</span></div>'
          +           '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Due", "Kurang")) + '</span><span>' + escapeHTML(money(totalsPayload.due)) + '</span></div>'
          +         '</div>'
          +         '<div class="pos-terminal__buttons" style="margin-top:1rem"><button type="button" class="pos-terminal__button" data-action="hold">' + escapeHTML(text("Hold Sale", "Tahan Penjualan")) + '</button><button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="checkout"' + (state.busy ? ' disabled' : '') + '>' + escapeHTML(state.busy ? text("Processing...", "Memproses...") : text("Complete Sale", "Selesaikan Penjualan")) + '</button></div>'
          +       '</article>'
          +     '</div>'
          +   '</section>'
          + '</section>';

        mount.querySelector("#pos-store")?.addEventListener("change", function(event) {
          state.storeCode = String(event.target.value || "");
          state.searchResults = [];
          persist();
          loadBootstrap().then(render).catch(function(error) { notify(error instanceof Error ? error.message : "Failed to load store context", "error"); });
        });
        mount.querySelector("#pos-register")?.addEventListener("change", function(event) {
          state.registerCode = String(event.target.value || "");
          persist();
          loadBootstrap().then(function() { return loadHeldSales(); }).catch(function(error) { notify(error instanceof Error ? error.message : "Failed to load register context", "error"); });
        });
        mount.querySelector("#pos-search")?.addEventListener("input", function(event) {
          state.searchQuery = String(event.target.value || "");
        });
        mount.querySelector("#pos-search")?.addEventListener("keydown", function(event) {
          if (event.key === "Enter") {
            event.preventDefault();
            searchCatalog().catch(function(error) { notify(error instanceof Error ? error.message : "Search failed", "error"); });
          }
        });
        mount.querySelector("#pos-lookup")?.addEventListener("input", function(event) {
          state.lookupQuery = String(event.target.value || "");
        });
        mount.querySelectorAll("[data-add-result]").forEach(function(node) {
          node.addEventListener("click", function() {
            const item = state.searchResults[number(node.getAttribute("data-add-result"))];
            if (item) addCatalogItem(item);
          });
        });
        mount.querySelectorAll("[data-resume-held]").forEach(function(node) {
          node.addEventListener("click", function() {
            const record = state.heldSales[number(node.getAttribute("data-resume-held"))];
            if (record) resumeHeldSale(record);
          });
        });
        mount.querySelectorAll("[data-refund-sale]").forEach(function(node) {
          node.addEventListener("click", function() {
            const record = state.transactions[number(node.getAttribute("data-refund-sale"))];
            if (record && window.confirm(text("Create refund documents for this sale?", "Buat dokumen refund untuk penjualan ini?"))) {
              transactAction(String(record.id || ""), "refund").catch(function(error) { notify(error instanceof Error ? error.message : "Refund failed", "error"); });
            }
          });
        });
        mount.querySelectorAll("[data-exchange-sale]").forEach(function(node) {
          node.addEventListener("click", function() {
            const record = state.transactions[number(node.getAttribute("data-exchange-sale"))];
            if (record && window.confirm(text("Create exchange documents for this sale?", "Buat dokumen tukar untuk penjualan ini?"))) {
              transactAction(String(record.id || ""), "exchange").catch(function(error) { notify(error instanceof Error ? error.message : "Exchange failed", "error"); });
            }
          });
        });
        mount.querySelectorAll("[data-line-qty]").forEach(function(node) {
          node.addEventListener("input", function() {
            const line = state.cart[number(node.getAttribute("data-line-qty"))];
            if (!line) return;
            line.quantity = number(node.value);
            recalcLine(line);
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-line-price]").forEach(function(node) {
          node.addEventListener("input", function() {
            const line = state.cart[number(node.getAttribute("data-line-price"))];
            if (!line) return;
            line.unit_price = number(node.value);
            recalcLine(line);
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-line-discount]").forEach(function(node) {
          node.addEventListener("input", function() {
            const line = state.cart[number(node.getAttribute("data-line-discount"))];
            if (!line) return;
            line.discount_amount = number(node.value);
            recalcLine(line);
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-remove-line]").forEach(function(node) {
          node.addEventListener("click", function() {
            state.cart.splice(number(node.getAttribute("data-remove-line")), 1);
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-tender-type]").forEach(function(node) {
          node.addEventListener("change", function() {
            const line = state.tenders[number(node.getAttribute("data-tender-type"))];
            if (!line) return;
            line.tender_type_code = String(node.value || "");
            const tenderType = tenderTypeByCode(line.tender_type_code);
            if (tenderType && !line.reference && String(((tenderType.values || {}).requires_reference || "false")) === "true") {
              line.reference = "";
            }
            persist();
          });
        });
        mount.querySelectorAll("[data-tender-amount]").forEach(function(node) {
          node.addEventListener("input", function() {
            const line = state.tenders[number(node.getAttribute("data-tender-amount"))];
            if (!line) return;
            line.amount = number(node.value);
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-tender-reference]").forEach(function(node) {
          node.addEventListener("input", function() {
            const line = state.tenders[number(node.getAttribute("data-tender-reference"))];
            if (!line) return;
            line.reference = String(node.value || "");
            persist();
          });
        });
        mount.querySelectorAll("[data-remove-tender]").forEach(function(node) {
          node.addEventListener("click", function() {
            state.tenders.splice(number(node.getAttribute("data-remove-tender")), 1);
            if (!state.tenders.length) state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-action]").forEach(function(node) {
          node.addEventListener("click", function() {
            const action = String(node.getAttribute("data-action") || "");
            if (action === "search") {
              searchCatalog().catch(function(error) { notify(error instanceof Error ? error.message : "Search failed", "error"); });
              return;
            }
            if (action === "open-shift") {
              openShift().catch(function(error) { notify(error instanceof Error ? error.message : "Open shift failed", "error"); });
              return;
            }
            if (action === "close-shift") {
              closeShift().catch(function(error) { notify(error instanceof Error ? error.message : "Close shift failed", "error"); });
              return;
            }
            if (action === "hold") {
              holdSale().catch(function(error) { notify(error instanceof Error ? error.message : "Hold sale failed", "error"); });
              return;
            }
            if (action === "checkout") {
              checkout();
              return;
            }
            if (action === "lookup") {
              loadBootstrap().then(function() { return loadHeldSales(); }).then(render).catch(function(error) { notify(error instanceof Error ? error.message : "Refresh failed", "error"); });
              return;
            }
            if (action === "lookup-transactions") {
              lookupTransactions().catch(function(error) { notify(error instanceof Error ? error.message : "Lookup failed", "error"); });
              return;
            }
            if (action === "add-tender") {
              state.tenders.push({ tender_type_code: "", amount: 0, reference: "", notes: "" });
              persist();
              render();
              return;
            }
            if (action === "print") {
              window.print();
            }
          });
        });
      }

      restore();
      await loadBootstrap();
      await loadHeldSales();
      render();
    }
  };
})();`
}
