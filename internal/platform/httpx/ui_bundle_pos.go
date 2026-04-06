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
      const initialCustomer = params.party_id ? { party_id: String(params.party_id), customer_name: String(params.party_name || params.party_id) } : null;
      const state = {
        loading: true,
        bootstrapping: false,
        searchBusy: false,
        lookupBusy: false,
        customerBusy: false,
        busy: false,
        terminalBusy: false,
        message: "",
        bootstrap: null,
        storeCode: params.store_code || "",
        registerCode: params.register_code || "",
        shiftId: "",
        terminalPIN: "",
        terminalOpeningCash: "0",
        terminalNotes: "",
        searchQuery: "",
        searchResults: [],
        cart: [],
        currentSaleID: "",
        tenders: [{ tender_type_code: "", amount: 0, reference: "", notes: "" }],
        heldSales: [],
        transactions: [],
        cartQuantityDrafts: {},
        tenderAmountDrafts: {},
        lookupQuery: "",
        customerQuery: "",
        customerResults: [],
        customer: initialCustomer,
        promotionCodes: "",
        promotionValidationBusy: false,
        promotionValidation: null,
        tenderInsights: {},
        catalogOpen: false,
        customerModalOpen: false,
        promoModalOpen: false,
        tenderModalOpen: false,
        heldExpanded: false,
        transactionsExpanded: false,
        controlCollapsed: true,
        lastCheckoutResult: null,
        receiptPrinting: false,
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
      const parseJSONArray = function(raw) {
        if (!raw) return [];
        if (Array.isArray(raw)) return raw;
        try {
          const parsed = JSON.parse(String(raw));
          return Array.isArray(parsed) ? parsed : [];
        } catch (_) {
          return [];
        }
      };
      const formatDateTime = function(value) {
        const parsed = value ? new Date(String(value)) : null;
        if (!parsed || Number.isNaN(parsed.getTime())) return "";
        return new Intl.DateTimeFormat(ctx.locale === "id" ? "id-ID" : "en-US", {
          year: "numeric",
          month: "short",
          day: "2-digit",
          hour: "2-digit",
          minute: "2-digit",
        }).format(parsed);
      };
      const compactDocumentNumber = function(value) {
        const raw = String(value || "").trim();
        if (!raw) return "";
        const compacted = raw
          .replace(/^INVOICE_NUMBER-/i, "INV-")
          .replace(/^ORDER_NUMBER-/i, "ORD-")
          .replace(/^PAYMENT_NUMBER-/i, "PAY-");
        if (compacted.length <= 30) return compacted;
        return compacted.slice(0, 12) + "…" + compacted.slice(-12);
      };
      const humanizeCode = function(value) {
        const raw = String(value || "").trim();
        if (!raw) return "";
        return raw
          .replace(/[-_]+/g, " ")
          .replace(/\s+/g, " ")
          .toLowerCase()
          .replace(/\b\w/g, function(char) { return char.toUpperCase(); });
      };
      const compactLabel = function(value, max) {
        const raw = String(value || "").trim();
        if (!raw) return "";
        if (raw.length <= max) return raw;
        return raw.slice(0, Math.max(0, max - 1)) + "…";
      };
      const truthy = function(value) {
        if (typeof value === "boolean") return value;
        const raw = String(value || "").trim().toLowerCase();
        return raw === "true" || raw === "1" || raw === "yes" || raw === "on";
      };
      const splitCSV = function(value) {
        return String(value || "").split(",").map(function(item) { return item.trim(); }).filter(Boolean);
      };
      const pickValue = function() {
        for (let index = 0; index < arguments.length; index += 1) {
          const current = arguments[index];
          if (current == null) continue;
          if (typeof current === "string") {
            if (current.trim()) return current.trim();
            continue;
          }
          if (typeof current === "number" && Number.isFinite(current)) return current;
          if (typeof current === "boolean") return current;
        }
        return "";
      };
      const buildReceiptLookupBars = function(value) {
        const raw = String(value || "").trim();
        if (!raw) return "";
        const encoded = raw.toUpperCase();
        const widths = [1, 1, 2, 1, 3, 1, 2, 2, 1, 3, 2, 1];
        let x = 2;
        const bars = [];
        for (let index = 0; index < encoded.length; index += 1) {
          const code = encoded.charCodeAt(index);
          for (let bit = 0; bit < 7; bit += 1) {
            if (((code >> bit) & 1) === 1) {
              const width = widths[(index + bit) % widths.length];
              const height = 18 + ((index + bit * 2) % 4) * 4;
              const y = 28 - height;
              bars.push('<rect x="' + x + '" y="' + y + '" width="' + width + '" height="' + height + '" rx="0.45"></rect>');
              x += width;
            }
            x += ((index + bit) % 3) + 1;
          }
          x += 2;
        }
        const width = Math.max(94, x + 2);
        return '<svg class="pos-terminal__receipt-lookup-bars" viewBox="0 0 ' + width + ' 28" preserveAspectRatio="none" aria-hidden="true"><rect x="0" y="0" width="' + width + '" height="28" fill="#fff"></rect>' + bars.join("") + '</svg>';
      };
      const buildReceiptLookupMatrix = function(value) {
        const raw = String(value || "").trim();
        if (!raw) return "";
        let seed = 0;
        for (let index = 0; index < raw.length; index += 1) {
          seed = (seed * 33 + raw.charCodeAt(index)) >>> 0;
        }
        const size = 21;
        const cell = 2;
        const padding = 2;
        const viewBox = size * cell + padding * 2;
        const finder = function(col, row) {
          return (col >= 0 && col < 7 && row >= 0 && row < 7)
            || (col >= size - 7 && col < size && row >= 0 && row < 7)
            || (col >= 0 && col < 7 && row >= size - 7 && row < size);
        };
        const isFinderDark = function(col, row) {
          const localCol = col >= size - 7 ? col - (size - 7) : col;
          const localRow = row >= size - 7 ? row - (size - 7) : row;
          if (localCol === 0 || localCol === 6 || localRow === 0 || localRow === 6) return true;
          if (localCol >= 2 && localCol <= 4 && localRow >= 2 && localRow <= 4) return true;
          return false;
        };
        const cells = [];
        for (let row = 0; row < size; row += 1) {
          for (let col = 0; col < size; col += 1) {
            let dark = false;
            if (finder(col, row)) {
              dark = isFinderDark(col, row);
            } else {
              const bit = ((seed >> ((row + col) % 24)) & 1) ^ ((row * 7 + col * 11 + raw.length) % 2);
              dark = bit === 1;
            }
            if (!dark) continue;
            cells.push('<rect x="' + (padding + col * cell) + '" y="' + (padding + row * cell) + '" width="' + cell + '" height="' + cell + '" rx="0.2"></rect>');
          }
        }
        return '<svg class="pos-terminal__receipt-lookup-matrix" viewBox="0 0 ' + viewBox + ' ' + viewBox + '" aria-hidden="true"><rect x="0" y="0" width="' + viewBox + '" height="' + viewBox + '" fill="#fff"></rect>' + cells.join("") + '</svg>';
      };
      const readCookie = function(name) {
        const match = document.cookie.match(new RegExp("(?:^|; )" + name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "=([^;]*)"));
        return match ? decodeURIComponent(match[1]) : "";
      };
      const nextFrame = function() {
        return new Promise(function(resolve) {
          window.requestAnimationFrame(function() { resolve(); });
        });
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
        }, tone === "error" ? 5200 : 2800);
      };
      const persist = function() {
        try {
          localStorage.setItem(storageKey, JSON.stringify({
            storeCode: state.storeCode,
            registerCode: state.registerCode,
            shiftId: state.shiftId,
            cart: state.cart,
            currentSaleID: state.currentSaleID,
            tenders: state.tenders,
            promotionCodes: state.promotionCodes,
            customer: state.customer,
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
            state.currentSaleID = String(payload.currentSaleID || "");
            state.tenders = Array.isArray(payload.tenders) && payload.tenders.length ? payload.tenders : state.tenders;
            state.promotionCodes = String(payload.promotionCodes || "");
            if (!state.customer && payload.customer && typeof payload.customer === "object") {
              state.customer = payload.customer;
            }
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
      const tenderKind = function(code) {
        const item = tenderTypeByCode(code);
        return item ? String((item.values || {}).kind || "") : "";
      };
      const tenderRequiresReference = function(code) {
        const item = tenderTypeByCode(code);
        return item ? String(((item.values || {}).requires_reference || "false")) === "true" : false;
      };
      const tenderRequiresParty = function(code) {
        const item = tenderTypeByCode(code);
        return item ? String(((item.values || {}).requires_party || "false")) === "true" : false;
      };
      const currentCashier = function() {
        return ((state.bootstrap || {}).current_cashier || null);
      };
      const receiptConfig = function() {
        const store = selectedStore();
        const register = selectedRegister();
        const storeValues = (store && store.values) || {};
        const registerValues = (register && register.values) || {};
        const variantCSV = pickValue(
          registerValues.receipt_print_variants,
          registerValues.receipt_variants,
          storeValues.receipt_print_variants,
          storeValues.receipt_variants,
          "customer,merchant"
        );
        const seenVariants = {};
        const variants = splitCSV(variantCSV).map(function(item) { return item.toLowerCase(); }).filter(function(item) {
          return item === "customer" || item === "merchant";
        }).filter(function(item) {
          if (seenVariants[item]) return false;
          seenVariants[item] = true;
          return true;
        });
        return {
          brandName: String(pickValue(registerValues.receipt_brand_name, storeValues.receipt_brand_name, registerValues.name, storeValues.name, state.storeCode || text("Store receipt", "Struk toko"))),
          headerText: String(pickValue(registerValues.receipt_header_text, storeValues.receipt_header_text, "")),
          footerText: String(pickValue(registerValues.receipt_footer_text, storeValues.receipt_footer_text, text("Thanks for shopping with Orbyte.", "Terima kasih sudah berbelanja dengan Orbyte."))),
          supportText: String(pickValue(registerValues.receipt_support_text, storeValues.receipt_support_text, text("Please keep this receipt for exchange or support.", "Simpan struk ini untuk penukaran atau bantuan."))),
          serviceText: String(pickValue(registerValues.receipt_service_text, storeValues.receipt_service_text, text("Present the lookup code at the register for faster assistance.", "Tunjukkan kode lookup di kasir untuk bantuan lebih cepat."))),
          merchantNote: String(pickValue(registerValues.receipt_merchant_note, storeValues.receipt_merchant_note, text("Retain this copy for register balancing and operational support.", "Simpan salinan ini untuk balancing register dan dukungan operasional."))),
          showQRCode: truthy(pickValue(registerValues.receipt_show_qr, storeValues.receipt_show_qr, true)),
          variants: variants.length ? variants : ["customer", "merchant"],
        };
      };
      const terminalContext = function() {
        return ((state.bootstrap || {}).terminal_context || null);
      };
      const terminalUnlocked = function() {
        const context = terminalContext();
        return !!(context && context.shift_id);
      };
      const resetSaleState = function() {
        state.cart = [];
        state.currentSaleID = "";
        state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
        state.cartQuantityDrafts = {};
        state.tenderAmountDrafts = {};
        state.customer = initialCustomer;
        state.customerResults = [];
        state.customerQuery = "";
        state.promotionCodes = "";
        state.promotionValidation = null;
        state.tenderInsights = {};
      };
      const effectiveTenderAmount = function(index, line) {
        const key = String(index);
        if (Object.prototype.hasOwnProperty.call(state.tenderAmountDrafts, key)) {
          const raw = String(state.tenderAmountDrafts[key] || "");
          return raw === "" ? 0 : number(raw);
        }
        return number((line || {}).amount);
      };
      const totals = function() {
        const subtotal = state.cart.reduce(function(sum, line) { return sum + number(line.line_subtotal || line.unit_price * line.quantity); }, 0);
        const tax = state.cart.reduce(function(sum, line) { return sum + number(line.tax_amount); }, 0);
        const total = state.cart.reduce(function(sum, line) { return sum + number(line.line_total || (line.line_subtotal + line.tax_amount)); }, 0);
        const tendered = state.tenders.reduce(function(sum, line, index) { return sum + effectiveTenderAmount(index, line); }, 0);
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
          + ".pos-terminal { height:min(calc(100vh - 11rem), 84rem); min-height:46rem; display:grid; grid-template-rows:auto minmax(0,1fr); gap:1rem; color:var(--color-body); }"
          + ".pos-terminal__controlbar, .pos-terminal__panel { border:1px solid color-mix(in srgb, var(--color-line) 88%, #101317 12%); background:color-mix(in srgb, var(--color-surface) 96%, #f7f2ec 4%); box-shadow:var(--shadow-panel); }"
          + ".pos-terminal__controlbar { display:grid; gap:0.65rem; border-radius:1.2rem; padding:0.75rem 0.9rem; }"
          + ".pos-terminal__controlbar--compact { gap:0.5rem; }"
          + ".pos-terminal__control-grid { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:0.8rem; align-items:start; }"
          + ".pos-terminal__control-actions, .pos-terminal__control-meta { display:flex; flex-wrap:wrap; gap:0.55rem; align-items:center; }"
          + ".pos-terminal__control-selects { display:grid; grid-template-columns:repeat(2,minmax(10rem,1fr)); gap:0.7rem; }"
          + ".pos-terminal__control-summary { display:flex; flex-wrap:wrap; gap:0.45rem; align-items:center; }"
          + ".pos-terminal__control-pill { display:grid; gap:0.08rem; min-width:7rem; padding:0.45rem 0.7rem; border:1px solid color-mix(in srgb, var(--color-line) 86%, #171b20 14%); border-radius:0.9rem; background:color-mix(in srgb, var(--color-shell) 34%, var(--color-surface)); }"
          + ".pos-terminal__control-pill strong { font-size:0.86rem; line-height:1.2; }"
          + ".pos-terminal__chiprow { display:flex; flex-wrap:wrap; gap:0.45rem; margin-top:0.8rem; }"
          + ".pos-terminal__chip { display:inline-flex; align-items:center; gap:0.35rem; border-radius:999px; padding:0.38rem 0.7rem; background:#efe4d3; color:#6d371c; font-size:0.74rem; font-weight:800; letter-spacing:0.08em; text-transform:uppercase; }"
          + ".pos-terminal__chip--ok { background:#dae9dc; color:#1e5632; }"
          + ".pos-terminal__chip--warn { background:#f4e4d6; color:#9a4b17; }"
          + ".pos-terminal__chip--danger { background:#f3d8d8; color:#8a2121; }"
          + ".pos-terminal__meta { display:grid; gap:0.3rem; padding:0.72rem 0.82rem; border-radius:1rem; background:linear-gradient(180deg, color-mix(in srgb, var(--color-shell) 55%, var(--color-surface) 45%), var(--color-surface)); }"
          + ".pos-terminal__meta-label { font-size:0.72rem; font-weight:800; letter-spacing:0.12em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__meta-value { font-size:0.95rem; font-weight:800; }"
          + ".pos-terminal__meta-sub { color:var(--color-muted); font-size:0.8rem; }"
          + ".pos-terminal__row { display:flex; flex-wrap:wrap; gap:0.7rem; align-items:end; }"
          + ".pos-terminal__field { display:flex; flex-direction:column; gap:0.35rem; min-width:10rem; flex:1; }"
          + ".pos-terminal__field span { font-size:0.72rem; font-weight:800; letter-spacing:0.12em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__field input, .pos-terminal__field select, .pos-terminal__field textarea { width:100%; min-width:0; min-height:2.75rem; border:1px solid color-mix(in srgb, var(--color-line) 84%, #15191d 16%); border-radius:0.9rem; background:var(--color-surface); color:var(--color-body); padding:0.7rem 0.85rem; font:inherit; }"
          + ".pos-terminal__field textarea { min-height:5rem; resize:vertical; }"
          + ".pos-terminal__buttons { display:flex; flex-wrap:wrap; gap:0.55rem; }"
          + ".pos-terminal__button { appearance:none; border:1px solid color-mix(in srgb, var(--color-line) 88%, #1a1d21 12%); border-radius:0.9rem; background:var(--color-surface); color:var(--color-body); padding:0.8rem 1rem; font:inherit; font-weight:800; cursor:pointer; transition:background 120ms ease, border-color 120ms ease, color 120ms ease, transform 120ms ease; }"
          + ".pos-terminal__button:hover { border-color:var(--color-accent); color:var(--color-accent-dark); }"
          + ".pos-terminal__button:disabled { opacity:0.55; cursor:not-allowed; transform:none; }"
          + ".pos-terminal__button--primary { background:#a04d22; border-color:#a04d22; color:#fff; }"
          + ".pos-terminal__button--primary:hover { background:#8d4119; border-color:#8d4119; color:#fff; }"
          + ".pos-terminal__button--warn { border-color:#d45555; color:#a12d2d; }"
          + ".pos-terminal__button--soft { background:color-mix(in srgb, var(--color-accent-soft) 62%, var(--color-surface) 38%); color:#7b3a1d; border-color:color-mix(in srgb, var(--color-accent) 35%, var(--color-line) 65%); }"
          + ".pos-terminal__button--compact { padding:0.62rem 0.82rem; min-height:2.35rem; }"
          + ".pos-terminal__customer-results, .pos-terminal__promo-list { display:grid; gap:0.55rem; max-height:11rem; overflow:auto; }"
          + ".pos-terminal__customer-card, .pos-terminal__promo-chip { border:1px solid color-mix(in srgb, var(--color-line) 88%, #171b20 12%); border-radius:0.95rem; background:color-mix(in srgb, var(--color-shell) 42%, var(--color-surface)); padding:0.75rem 0.85rem; }"
          + ".pos-terminal__customer-card { display:flex; justify-content:space-between; gap:0.8rem; align-items:flex-start; }"
          + ".pos-terminal__customer-name { font-weight:800; }"
          + ".pos-terminal__customer-meta { color:var(--color-muted); font-size:0.83rem; margin-top:0.2rem; }"
          + ".pos-terminal__mini-stack { display:grid; gap:0.25rem; }"
          + ".pos-terminal__mini-meta { display:flex; flex-wrap:wrap; gap:0.55rem; align-items:center; }"
          + ".pos-terminal__summary-chipline { display:flex; flex-wrap:wrap; gap:0.45rem; align-items:center; justify-content:flex-end; }"
          + ".pos-terminal__summary-pill { display:inline-flex; align-items:center; gap:0.35rem; min-height:2.75rem; padding:0.45rem 0.8rem; border-radius:0.9rem; border:1px solid color-mix(in srgb, var(--color-line) 86%, #171b20 14%); background:color-mix(in srgb, var(--color-shell) 36%, var(--color-surface)); font-size:0.83rem; }"
          + ".pos-terminal__readonly { display:block; min-height:2.35rem; padding:0.55rem 0.65rem; border-radius:0.8rem; background:color-mix(in srgb, var(--color-shell) 45%, var(--color-surface)); border:1px solid color-mix(in srgb, var(--color-line) 84%, #171b20 16%); font-weight:700; }"
          + ".pos-terminal__workspace { min-height:0; display:grid; grid-template-columns:minmax(0,1.5fr) minmax(25rem,28rem); gap:1rem; }"
          + ".pos-terminal__left { min-height:0; display:grid; grid-template-rows:minmax(0,1fr) auto; gap:1rem; }"
          + ".pos-terminal__aux { min-height:0; display:grid; grid-template-columns:minmax(0,1fr) minmax(0,1fr); gap:1rem; align-content:start; }"
          + ".pos-terminal__rail { min-height:0; display:grid; grid-template-rows:auto minmax(0,1fr) auto; gap:1rem; }"
          + ".pos-terminal__panel { display:grid; grid-template-rows:auto minmax(0,1fr); min-height:0; border-radius:1.1rem; overflow:hidden; }"
          + ".pos-terminal__panel--collapsed { grid-template-rows:auto; }"
          + ".pos-terminal__panel-head { display:flex; justify-content:space-between; gap:0.75rem; align-items:flex-start; padding:0.95rem 1rem; border-bottom:1px solid color-mix(in srgb, var(--color-line) 86%, #12161b 14%); }"
          + ".pos-terminal__panel--collapsed .pos-terminal__panel-head { align-items:center; padding:0.78rem 1rem; min-height:4.2rem; }"
          + ".pos-terminal__panel--collapsed .pos-terminal__panel-sub { display:none; }"
          + ".pos-terminal__panel-title { margin:0; font-size:0.92rem; font-weight:900; letter-spacing:0.1em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__panel-sub { color:var(--color-muted); font-size:0.83rem; margin-top:0.25rem; }"
          + ".pos-terminal__panel-body { min-height:0; padding:0.95rem 1rem; }"
          + ".pos-terminal__scroll { min-height:0; overflow:auto; }"
          + ".pos-terminal__result-list, .pos-terminal__held-list, .pos-terminal__txn-list, .pos-terminal__tender-list { display:grid; gap:0.7rem; }"
          + ".pos-terminal__result, .pos-terminal__held, .pos-terminal__txn, .pos-terminal__tender { border:1px solid color-mix(in srgb, var(--color-line) 85%, #161a1e 15%); border-radius:1rem; background:color-mix(in srgb, var(--color-shell) 34%, var(--color-surface)); padding:0.9rem; }"
          + ".pos-terminal__result-head, .pos-terminal__sale-head { display:flex; justify-content:space-between; gap:0.75rem; align-items:flex-start; }"
          + ".pos-terminal__title { font-weight:800; }"
          + ".pos-terminal__muted { color:var(--color-muted); font-size:0.84rem; }"
          + ".pos-terminal__statgrid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:0.6rem; }"
          + ".pos-terminal__stat { border-radius:0.95rem; padding:0.8rem; background:color-mix(in srgb, var(--color-shell) 55%, var(--color-surface)); border:1px solid color-mix(in srgb, var(--color-line) 84%, #161a1e 16%); }"
          + ".pos-terminal__stat label { display:block; font-size:0.7rem; font-weight:800; letter-spacing:0.1em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__stat strong { display:block; margin-top:0.2rem; font-size:1.2rem; }"
          + ".pos-terminal__cart-table { width:100%; border-collapse:collapse; table-layout:fixed; }"
          + ".pos-terminal__cart-table th, .pos-terminal__cart-table td { padding:0.72rem 0.45rem; border-top:1px solid color-mix(in srgb, var(--color-line) 86%, #171b20 14%); vertical-align:top; text-align:left; }"
          + ".pos-terminal__cart-table th { font-size:0.72rem; font-weight:900; letter-spacing:0.1em; text-transform:uppercase; color:var(--color-muted); }"
          + ".pos-terminal__cart-table td:last-child, .pos-terminal__cart-table th:last-child { text-align:right; }"
          + ".pos-terminal__cart-table input { width:100%; min-width:0; min-height:2.35rem; border:1px solid color-mix(in srgb, var(--color-line) 84%, #171b20 16%); border-radius:0.8rem; background:var(--color-surface); color:var(--color-body); padding:0 0.65rem; font:inherit; }"
          + ".pos-terminal__summary { display:grid; gap:0.6rem; }"
          + ".pos-terminal__summary-row { display:flex; justify-content:space-between; gap:1rem; color:var(--color-body); }"
          + ".pos-terminal__summary-row strong { font-size:1.08rem; }"
          + ".pos-terminal__due { border-radius:1rem; padding:1rem; background:#f4e5d7; color:#6d3418; }"
          + ".pos-terminal__due strong { display:block; font-size:2rem; line-height:1; margin-top:0.25rem; }"
          + ".pos-terminal__due small { display:block; margin-top:0.25rem; color:#8b4c22; }"
          + ".pos-terminal__empty, .pos-terminal__setup { display:grid; place-items:center; min-height:100%; padding:1.2rem; text-align:center; border:1px dashed color-mix(in srgb, var(--color-line) 80%, #191d22 20%); border-radius:1rem; color:var(--color-muted); }"
          + ".pos-terminal__setup { background:linear-gradient(180deg, color-mix(in srgb, var(--color-accent-soft) 58%, var(--color-surface) 42%), var(--color-surface)); }"
          + ".pos-terminal__notice { padding:0.85rem 1rem; border-radius:1rem; background:color-mix(in srgb, var(--color-accent-soft) 58%, var(--color-surface) 42%); color:var(--color-body); border:1px solid color-mix(in srgb, var(--color-accent) 24%, var(--color-line) 76%); }"
          + ".pos-terminal__stack { display:grid; gap:0.75rem; }"
          + ".pos-terminal__toggle { appearance:none; border:0; background:transparent; color:var(--color-accent-dark); font:inherit; font-weight:800; cursor:pointer; padding:0.2rem 0; }"
          + ".pos-terminal__collapsed { display:grid; place-items:center; min-height:4.35rem; padding:0.75rem 1rem; text-align:center; color:var(--color-muted); }"
          + ".pos-terminal__overlay { position:fixed; inset:0; z-index:80; display:grid; place-items:center; background:rgba(9, 12, 16, 0.46); padding:1.5rem; backdrop-filter:blur(5px); }"
          + ".pos-terminal__modal { width:min(72rem, calc(100vw - 3rem)); height:min(48rem, calc(100vh - 3rem)); display:grid; grid-template-rows:auto minmax(0,1fr); border:1px solid color-mix(in srgb, var(--color-line) 84%, #101317 16%); border-radius:1.3rem; background:color-mix(in srgb, var(--color-surface) 97%, #faf6f1 3%); box-shadow:0 2rem 5rem rgba(12, 16, 22, 0.24); overflow:hidden; }"
          + ".pos-terminal__modal-head { display:flex; justify-content:space-between; gap:0.9rem; align-items:flex-start; padding:1rem 1.1rem; border-bottom:1px solid color-mix(in srgb, var(--color-line) 86%, #12161b 14%); }"
          + ".pos-terminal__modal-body { min-height:0; display:grid; grid-template-rows:auto minmax(0,1fr); gap:0.95rem; padding:1rem 1.1rem 1.1rem; }"
          + ".pos-terminal__modal-body .pos-terminal__scroll { padding-right:0.25rem; }"
          + ".pos-terminal__receipt-print { display:none; }"
          + ".pos-terminal__receipt-shell { width:80mm; max-width:80mm; margin:0 auto; padding:4mm 4.5mm 6mm; background:#fff; color:#111; font-family:\"SFMono-Regular\", Menlo, Consolas, \"Liberation Mono\", monospace; }"
          + ".pos-terminal__receipt-copy { page-break-after:always; }"
          + ".pos-terminal__receipt-copy:last-child { page-break-after:auto; }"
          + ".pos-terminal__receipt-head { display:grid; gap:0.22rem; padding-bottom:0.7rem; border-bottom:1px dashed #7b7b7b; text-align:center; }"
          + ".pos-terminal__receipt-brand { font-size:1.04rem; font-weight:900; letter-spacing:0.06em; text-transform:uppercase; }"
          + ".pos-terminal__receipt-sub { font-size:0.69rem; line-height:1.42; color:#4b4b4b; }"
          + ".pos-terminal__receipt-header-grid { display:grid; gap:0.14rem; margin-top:0.22rem; }"
          + ".pos-terminal__receipt-divider { border-top:1px dashed #8b8b8b; margin:0.12rem 0; }"
          + ".pos-terminal__receipt-cutline { display:flex; align-items:center; gap:0.45rem; margin-top:0.7rem; color:#666; font-size:0.62rem; letter-spacing:0.14em; text-transform:uppercase; }"
          + ".pos-terminal__receipt-cutline::before, .pos-terminal__receipt-cutline::after { content:\"\"; flex:1; border-top:1px dashed #8b8b8b; }"
          + ".pos-terminal__receipt-section { display:grid; gap:0.26rem; padding:0.62rem 0; border-bottom:1px dashed #8b8b8b; }"
          + ".pos-terminal__receipt-section:last-child { border-bottom:0; }"
          + ".pos-terminal__receipt-kicker { font-size:0.64rem; font-weight:800; letter-spacing:0.14em; text-transform:uppercase; color:#666; }"
          + ".pos-terminal__receipt-meta { display:flex; justify-content:space-between; gap:0.55rem; font-size:0.71rem; line-height:1.42; }"
          + ".pos-terminal__receipt-meta span:first-child { color:#4b4b4b; }"
          + ".pos-terminal__receipt-meta strong, .pos-terminal__receipt-meta span:last-child { text-align:right; }"
          + ".pos-terminal__receipt-summaryline { display:flex; flex-wrap:wrap; justify-content:center; gap:0.28rem; padding:0.58rem 0 0.08rem; }"
          + ".pos-terminal__receipt-summarypill { display:inline-flex; align-items:center; gap:0.22rem; padding:0.14rem 0.38rem; border:1px solid #b9b9b9; border-radius:999px; font-size:0.6rem; font-weight:800; letter-spacing:0.08em; text-transform:uppercase; color:#333; }"
          + ".pos-terminal__receipt-summarypill--paid { border-color:#111; background:#111; color:#fff; }"
          + ".pos-terminal__receipt-code { max-width:42mm; overflow-wrap:anywhere; word-break:break-word; }"
          + ".pos-terminal__receipt-items { display:grid; gap:0.58rem; }"
          + ".pos-terminal__receipt-item { display:grid; gap:0.14rem; }"
          + ".pos-terminal__receipt-item-head { display:flex; justify-content:space-between; gap:0.55rem; align-items:flex-start; font-size:0.75rem; }"
          + ".pos-terminal__receipt-item-name { flex:1; font-weight:800; line-height:1.35; }"
          + ".pos-terminal__receipt-item-price { white-space:nowrap; text-align:right; font-weight:800; }"
          + ".pos-terminal__receipt-item-sub { display:flex; justify-content:space-between; gap:0.55rem; font-size:0.68rem; color:#555; }"
          + ".pos-terminal__receipt-item-note { font-size:0.64rem; color:#666; line-height:1.35; }"
          + ".pos-terminal__receipt-totals { display:grid; gap:0.22rem; }"
          + ".pos-terminal__receipt-total-row { display:flex; justify-content:space-between; gap:0.55rem; font-size:0.73rem; }"
          + ".pos-terminal__receipt-total-row--grand { margin-top:0.22rem; padding-top:0.42rem; border-top:1px dashed #8b8b8b; font-size:0.91rem; font-weight:900; }"
          + ".pos-terminal__receipt-total-row--change { font-weight:800; }"
          + ".pos-terminal__receipt-settlement { margin-top:0.34rem; text-align:center; font-size:0.64rem; font-weight:800; letter-spacing:0.08em; text-transform:uppercase; color:#333; }"
          + ".pos-terminal__receipt-payment { display:grid; gap:0.08rem; }"
          + ".pos-terminal__receipt-payment-ref { font-size:0.64rem; color:#5d5d5d; text-align:right; }"
          + ".pos-terminal__receipt-lookup { display:grid; gap:0.24rem; padding:0.62rem 0; text-align:center; border-bottom:1px dashed #8b8b8b; }"
          + ".pos-terminal__receipt-lookup-visuals { display:grid; grid-template-columns:minmax(0,1fr) 18mm; align-items:end; gap:0.5rem; }"
          + ".pos-terminal__receipt-lookup-bars { display:block; width:100%; height:1.2rem; margin:0 auto; fill:#111; }"
          + ".pos-terminal__receipt-lookup-matrix { display:block; width:18mm; height:18mm; justify-self:end; fill:#111; }"
          + ".pos-terminal__receipt-lookup-qr { display:block; width:18mm; height:18mm; justify-self:end; object-fit:contain; border:1px solid #d0d0d0; padding:1mm; background:#fff; }"
          + ".pos-terminal__receipt-lookup-code { font-size:0.82rem; font-weight:900; letter-spacing:0.08em; word-break:break-all; }"
          + ".pos-terminal__receipt-foot { display:grid; gap:0.32rem; padding-top:0.72rem; text-align:center; font-size:0.68rem; color:#4b4b4b; }"
          + ".pos-terminal__receipt-badge { display:inline-block; margin:0 auto; padding:0.14rem 0.42rem; border:1px solid #111; font-size:0.6rem; font-weight:900; letter-spacing:0.14em; text-transform:uppercase; }"
          + ".pos-terminal__receipt-accent { font-weight:800; color:#111; }"
          + ".pos-terminal__receipt-policy { display:grid; gap:0.18rem; padding:0.42rem 0.6rem; border:1px dashed #8b8b8b; border-radius:0.45rem; background:#fafafa; }"
          + "@media (max-width: 1360px) { .pos-terminal { height:auto; min-height:0; } .pos-terminal__workspace { grid-template-columns:1fr; } .pos-terminal__rail { grid-template-rows:auto auto auto; } .pos-terminal__left { grid-template-rows:auto auto auto; } .pos-terminal__aux { grid-template-columns:1fr; } .pos-terminal__control-grid { grid-template-columns:1fr; } .pos-terminal__control-summary { width:100%; } }"
          + "@media (max-width: 820px) { .pos-terminal__control-selects, .pos-terminal__statgrid { grid-template-columns:1fr; } .pos-terminal__overlay { padding:0.75rem; } .pos-terminal__modal { width:calc(100vw - 1.5rem); height:calc(100vh - 1.5rem); } }"
          + "@media print { @page { size:80mm auto; margin:0; } html, body { margin:0 !important; padding:0 !important; background:#fff !important; } body * { visibility:hidden !important; } .pos-terminal > :not(.pos-terminal__receipt-print) { display:none !important; } .pos-terminal__receipt-print, .pos-terminal__receipt-print * { visibility:visible !important; } .pos-terminal__receipt-print { display:block !important; position:absolute !important; top:0; left:0; right:auto; bottom:auto; width:100%; background:#fff; } .pos-terminal__receipt-shell { width:80mm; max-width:80mm; min-height:auto; box-shadow:none; } }"
          + "@media print and (max-width: 58mm) { @page { size:58mm auto; margin:0; } .pos-terminal__receipt-shell { width:58mm; max-width:58mm; padding:3mm 3.2mm 5mm; } .pos-terminal__receipt-brand { font-size:0.92rem; } .pos-terminal__receipt-lookup-visuals { grid-template-columns:minmax(0,1fr) 14mm; gap:0.35rem; } .pos-terminal__receipt-lookup-matrix, .pos-terminal__receipt-lookup-qr { width:14mm; height:14mm; } .pos-terminal__receipt-item-head, .pos-terminal__receipt-item-sub, .pos-terminal__receipt-meta, .pos-terminal__receipt-total-row { font-size:0.67rem; } .pos-terminal__receipt-lookup-code { font-size:0.74rem; } }";
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

      function payloadPromotionCodes() {
        return String(state.promotionCodes || "").split(/[\n,;]+/).map(function(item) {
          return String(item || "").trim();
        }).filter(Boolean);
      }

      function appliedPromotionCodes() {
        if (!state.promotionValidation || !Array.isArray(state.promotionValidation.codes)) return [];
        return state.promotionValidation.codes.filter(function(item) {
          return String(item && item.status || "").toLowerCase() === "applied";
        }).map(function(item) {
          return String(item.code || "").trim();
        }).filter(Boolean);
      }

      function activePromotionCodes() {
        return appliedPromotionCodes();
      }

      function payloadTenders() {
        return state.tenders.filter(function(tender, index) { return String(tender.tender_type_code || "").trim() !== "" && effectiveTenderAmount(index, tender) > 0; }).map(function(tender, index) {
          return {
            tender_type_code: tender.tender_type_code,
            amount: effectiveTenderAmount(index, tender),
            reference: tender.reference || "",
            notes: tender.notes || "",
          };
        });
      }

      function flushPendingDrafts() {
        Object.keys(state.cartQuantityDrafts).forEach(function(key) {
          const index = number(key);
          const node = mount.querySelector("[data-line-qty='" + String(index) + "']");
          const value = node ? node.value : state.cartQuantityDrafts[key];
          commitLineQuantity(index, value);
        });
        Object.keys(state.tenderAmountDrafts).forEach(function(key) {
          const index = number(key);
          const node = mount.querySelector("[data-tender-amount='" + String(index) + "']");
          const value = node ? node.value : state.tenderAmountDrafts[key];
          commitTenderAmount(index, value);
        });
      }

      function commitLineQuantity(index, rawValue) {
        const line = state.cart[index];
        if (!line) return;
        const value = rawValue == null ? "" : String(rawValue);
        const requested = value === "" ? 0 : number(value);
        const available = number(line.available_quantity);
        if (line.inventory_enabled && requested > available) {
          line.quantity = available;
          delete state.cartQuantityDrafts[String(index)];
          recalcLine(line);
          state.promotionValidation = null;
          persist();
          render();
          notify(text("Insufficient available stock for this item.", "Stok tersedia untuk item ini tidak cukup."), "error");
          return;
        }
        if (value === "") {
          line.quantity = 0;
        } else {
          line.quantity = requested;
        }
        delete state.cartQuantityDrafts[String(index)];
        recalcLine(line);
        state.promotionValidation = null;
        persist();
        render();
      }

      function commitTenderAmount(index, rawValue) {
        const line = state.tenders[index];
        if (!line) return;
        const value = rawValue == null ? "" : String(rawValue);
        if (value === "") {
          line.amount = 0;
        } else {
          line.amount = number(value);
        }
        delete state.tenderAmountDrafts[String(index)];
        persist();
        render();
      }

      function addCatalogItem(item) {
        const existing = state.cart.find(function(line) {
          return String(line.item_code || "") === String(item.item_code || "") && String(line.variant_signature || "") === String(item.variant_signature || "");
        });
        if (existing) {
          const nextQuantity = number(existing.quantity) + 1;
          if (existing.inventory_enabled && nextQuantity > number(existing.available_quantity)) {
            notify(text("Insufficient available stock for this item.", "Stok tersedia untuk item ini tidak cukup."), "error");
            return;
          }
          existing.quantity = nextQuantity;
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
        state.promotionValidation = null;
        persist();
        render();
      }

      async function loadBootstrap() {
        state.bootstrapping = true;
        render();
        try {
          const query = new URLSearchParams();
          if (state.storeCode) query.set("store_code", state.storeCode);
          if (state.registerCode) query.set("register_code", state.registerCode);
          state.bootstrap = await api("/ui/data/pos/bootstrap" + (query.toString() ? "?" + query.toString() : ""));
          if (state.bootstrap.terminal_context) {
            state.storeCode = String((state.bootstrap.terminal_context.store_code || state.storeCode || ""));
            state.registerCode = String((state.bootstrap.terminal_context.register_code || state.registerCode || ""));
            state.shiftId = String((state.bootstrap.terminal_context.shift_id || ""));
          } else {
            state.shiftId = "";
          }
          if (!state.storeCode && state.bootstrap.current_store && state.bootstrap.current_store.values) {
            state.storeCode = String(state.bootstrap.current_store.values.code || "");
          }
        } finally {
          state.bootstrapping = false;
        }
      }

      async function enterTerminal(mode) {
        if (!state.storeCode || !state.registerCode) {
          notify(text("Store and register are required.", "Toko dan register wajib diisi."), "error");
          return;
        }
        if (!String(state.terminalPIN || "").trim()) {
          notify(text("Enter your cashier PIN.", "Masukkan PIN kasir."), "error");
          return;
        }
        state.terminalBusy = true;
        render();
        try {
          const request = {
            store_code: state.storeCode,
            register_code: state.registerCode,
            pin: String(state.terminalPIN || ""),
            opening_cash_amount: number(state.terminalOpeningCash || 0),
            notes: state.terminalNotes || "",
          };
          if (mode === "resume" && state.bootstrap && state.bootstrap.open_shift && state.bootstrap.open_shift.id) {
            request.shift_id = String(state.bootstrap.open_shift.id || "");
          }
          const payload = await api("/ui/data/pos/terminal/enter", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(request),
          });
          state.terminalPIN = "";
          state.shiftId = String((((payload || {}).terminal_context || {}).shift_id) || "");
          await loadBootstrap();
          await loadHeldSales();
          notify(mode === "resume" ? text("Shift resumed.", "Shift dilanjutkan.") : text("Terminal unlocked.", "Terminal terbuka."));
          persist();
          render();
        } catch (error) {
          notify(error instanceof Error ? error.message : text("Failed to enter terminal.", "Gagal masuk terminal."), "error");
        } finally {
          state.terminalBusy = false;
          render();
        }
      }

      async function lockTerminal() {
        await api("/ui/data/pos/terminal/lock", { method: "POST" });
        state.shiftId = "";
        state.terminalPIN = "";
        resetSaleState();
        await loadBootstrap();
        await loadHeldSales();
        persist();
        notify(text("Terminal locked.", "Terminal dikunci."));
        render();
      }

      async function searchCatalog() {
        if (!state.storeCode) {
          notify(text("Select a store first.", "Pilih toko terlebih dahulu."), "error");
          return;
        }
        state.searchBusy = true;
        render();
        try {
          const query = new URLSearchParams();
          query.set("store_code", state.storeCode);
          query.set("q", state.searchQuery);
          const payload = await api("/ui/data/pos/catalog/search?" + query.toString());
          state.searchResults = payload.items || [];
          emitHardwareEvent("orbyte:pos-scanner-input", { query: state.searchQuery, matches: state.searchResults });
        } finally {
          state.searchBusy = false;
          render();
        }
      }

      function openCatalog() {
        state.catalogOpen = true;
        render();
        window.requestAnimationFrame(function() {
          mount.querySelector("#pos-search")?.focus();
        });
      }

      function closeCatalog() {
        state.catalogOpen = false;
        render();
      }

      function openCustomerModal() {
        state.customerModalOpen = true;
        render();
        window.requestAnimationFrame(function() {
          mount.querySelector("#pos-customer-search")?.focus();
        });
      }

      function closeCustomerModal() {
        state.customerModalOpen = false;
        render();
      }

      function openPromoModal() {
        state.promoModalOpen = true;
        render();
        window.requestAnimationFrame(function() {
          mount.querySelector("#pos-voucher-codes")?.focus();
        });
      }

      function closePromoModal() {
        state.promoModalOpen = false;
        render();
      }

      function openTenderModal() {
        state.tenderModalOpen = true;
        render();
      }

      function closeTenderModal() {
        flushPendingDrafts();
        state.tenderModalOpen = false;
        render();
      }

      function renderPromotionValidation() {
        if (state.promotionValidationBusy) {
          return '<div class="pos-terminal__empty">' + escapeHTML(text("Validating promo / voucher codes…", "Memvalidasi kode promo / voucher…")) + '</div>';
        }
        if (!state.promotionValidation || !Array.isArray(state.promotionValidation.codes) || !state.promotionValidation.codes.length) {
          return '<div class="pos-terminal__empty">' + escapeHTML(text("Validate codes to preview whether they apply to the current basket.", "Validasi kode untuk melihat apakah kode berlaku pada keranjang saat ini.")) + '</div>';
        }
        const summary = state.promotionValidation.valid
          ? text("All entered codes are valid for this basket.", "Semua kode valid untuk keranjang ini.")
          : text("Some codes are invalid or not applicable.", "Beberapa kode tidak valid atau tidak berlaku.");
        return ''
          + '<div class="pos-terminal__stack">'
          +   '<div class="pos-terminal__notice">' + escapeHTML(summary) + (state.promotionValidation.discount_amount_total > 0 ? ' ' + escapeHTML(text("Discount preview:", "Pratinjau diskon:")) + ' ' + escapeHTML(money(state.promotionValidation.discount_amount_total || 0)) : '') + '</div>'
          +   '<div class="pos-terminal__promo-list">' + state.promotionValidation.codes.map(function(item) {
                const tone = item.status === "applied" ? "pos-terminal__chip--ok" : item.status === "not_applicable" ? "pos-terminal__chip--warn" : "pos-terminal__chip--danger";
                return '<div class="pos-terminal__promo-chip"><div class="pos-terminal__sale-head"><div><div class="pos-terminal__title">' + escapeHTML(String(item.code || "")) + '</div><div class="pos-terminal__muted">' + escapeHTML(String(item.message || "")) + '</div></div><span class="pos-terminal__chip ' + tone + '">' + escapeHTML(String(item.status || "")) + '</span></div>' + (Number(item.discount_amount || 0) > 0 ? '<div class="pos-terminal__muted" style="margin-top:0.55rem">' + escapeHTML(text("Discount", "Diskon")) + ': ' + escapeHTML(money(item.discount_amount || 0)) + '</div>' : '') + '</div>';
              }).join("") + '</div>'
          + '</div>';
      }

      async function searchCustomers() {
        state.customerBusy = true;
        render();
        try {
          const query = new URLSearchParams();
          if (state.customerQuery) query.set("q", state.customerQuery);
          const payload = await api("/ui/data/pos/customers/search?" + query.toString());
          state.customerResults = payload.items || [];
        } catch (error) {
          notify(error instanceof Error ? error.message : text("Customer lookup failed.", "Lookup pelanggan gagal."), "error");
        } finally {
          state.customerBusy = false;
          render();
        }
      }

      async function validatePromotions() {
        const codes = payloadPromotionCodes();
        if (!codes.length) {
          state.promotionValidation = null;
          notify(text("Enter at least one promo or voucher code.", "Masukkan minimal satu kode promo atau voucher."), "error");
          render();
          return;
        }
        state.promotionValidationBusy = true;
        render();
        try {
          const payload = await api("/ui/data/pos/promotions/validate", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              store_code: state.storeCode,
              party_id: state.customer ? state.customer.party_id : "",
              party_name: state.customer ? state.customer.customer_name : "",
              lines: payloadLines(),
              promotion_codes: codes,
            }),
          });
          state.promotionValidation = payload;
          if (payload && payload.valid) {
            notify(text("Promo / voucher codes validated.", "Kode promo / voucher tervalidasi."));
          } else {
            notify(text("Some promo / voucher codes need attention.", "Ada kode promo / voucher yang perlu diperiksa."), "error");
          }
        } catch (error) {
          state.promotionValidation = null;
          notify(error instanceof Error ? error.message : text("Promo validation failed.", "Validasi promo gagal."), "error");
        } finally {
          state.promotionValidationBusy = false;
          render();
        }
      }

      function attachCustomer(item) {
        state.customer = item ? {
          party_id: String(item.party_id || ""),
          customer_name: String(item.customer_name || item.party_id || ""),
          member_status: String(item.member_status || ""),
          member_tier: String(item.member_tier || ""),
          member_valid_to: String(item.member_valid_to || ""),
          customer_type: String(item.customer_type || ""),
        } : null;
        state.customerResults = [];
        state.customerQuery = state.customer ? state.customer.customer_name : "";
        state.customerModalOpen = false;
        state.promotionValidation = null;
        persist();
        maybeRefreshStoreCreditInsights();
        render();
      }

      async function lookupStoredValue(index) {
        const tender = state.tenders[index];
        if (!tender) return;
        const kind = tenderKind(tender.tender_type_code);
        if (kind !== "gift_card" && kind !== "store_credit") return;
        state.tenderInsights[String(index)] = { loading: true };
        render();
        try {
          const query = new URLSearchParams();
          query.set("kind", kind);
          if (kind === "gift_card") {
            query.set("reference", String(tender.reference || ""));
          } else {
            query.set("party_id", String((state.customer || {}).party_id || ""));
          }
          const payload = await api("/ui/data/pos/stored-value/lookup?" + query.toString());
          state.tenderInsights[String(index)] = payload.item || {};
        } catch (error) {
          state.tenderInsights[String(index)] = { error: error instanceof Error ? error.message : "Lookup failed" };
        }
        render();
      }

      function maybeRefreshStoreCreditInsights() {
        state.tenders.forEach(function(tender, index) {
          if (tenderKind(tender.tender_type_code) === "store_credit" && state.customer && state.customer.party_id) {
            lookupStoredValue(index).catch(function() {});
          }
        });
      }

      async function openShift() {
        return enterTerminal("open");
      }

      async function closeShift() {
        if (!state.shiftId) {
          notify(text("No open shift.", "Tidak ada shift terbuka."), "error");
          return;
        }
        const entered = window.prompt(text("Counted cash amount", "Jumlah kas aktual"), String(totals().tendered || 0));
        if (entered === null) {
          return;
        }
        const actual = Number(String(entered).trim() || "0");
        await api("/ui/data/pos/shifts/" + encodeURIComponent(state.shiftId) + "/close", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ actual_cash_amount: Number.isFinite(actual) ? actual : 0 }),
        });
        state.shiftId = "";
        resetSaleState();
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
        flushPendingDrafts();
        if (!terminalUnlocked()) {
          notify(text("Open a shift first.", "Buka shift terlebih dahulu."), "error");
          return;
        }
        const payload = await api("/ui/data/pos/sales/hold", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            sale_id: state.currentSaleID,
            store_code: state.storeCode,
            register_code: state.registerCode,
            shift_id: state.shiftId,
            party_id: state.customer ? state.customer.party_id : "",
            party_name: state.customer ? state.customer.customer_name : "",
            lines: payloadLines(),
            tenders: payloadTenders(),
            promotion_codes: activePromotionCodes(),
            offline_cached: !navigator.onLine,
          }),
        });
        await loadHeldSales();
        state.cart = [];
        state.currentSaleID = "";
        state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
        state.promotionCodes = "";
        state.promotionValidation = null;
        state.tenderInsights = {};
        persist();
        notify(text("Sale held.", "Penjualan disimpan."));
        render();
        return payload;
      }

      function resumeHeldSale(record) {
        try {
          state.currentSaleID = String((record || {}).id || "");
          state.cart = JSON.parse(String(((record || {}).values || {}).lines_json || "[]"));
          state.tenders = JSON.parse(String(((record || {}).values || {}).tenders_json || "[{\"tender_type_code\":\"\",\"amount\":0}]"));
          state.promotionCodes = JSON.parse(String(((record || {}).values || {}).promotion_codes_json || "[]")).join(", ");
          if (!Array.isArray(state.tenders) || !state.tenders.length) {
            state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
          }
          state.customer = {
            party_id: String(((record || {}).values || {}).party_id || ""),
            customer_name: String(((record || {}).values || {}).party_name || text("Walk-in customer", "Pelanggan umum")),
          };
          state.cart = state.cart.map(recalcLine);
          state.tenderInsights = {};
          state.promotionValidation = null;
          persist();
          maybeRefreshStoreCreditInsights();
          notify(text("Held sale loaded.", "Penjualan tertahan dimuat."));
          render();
        } catch (_) {
          notify(text("Failed to load held sale.", "Gagal memuat penjualan tertahan."), "error");
        }
      }

      async function checkout() {
        flushPendingDrafts();
        if (!navigator.onLine) {
          notify(text("Checkout requires a live connection.", "Checkout membutuhkan koneksi aktif."), "error");
          return;
        }
        if (!terminalUnlocked()) {
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
              sale_id: state.currentSaleID,
              party_id: state.customer ? state.customer.party_id : "",
              party_name: state.customer ? state.customer.customer_name : "",
              lines: payloadLines(),
              tenders: payloadTenders(),
              promotion_codes: activePromotionCodes(),
              offline_cached: false,
            }),
          });
          emitHardwareEvent("orbyte:pos-receipt-print", result);
          emitHardwareEvent("orbyte:pos-cash-drawer-open", { total: totals().total, change: totals().change });
          state.lastCheckoutResult = result;
          state.cart = [];
          state.currentSaleID = "";
          state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
          state.promotionCodes = "";
          state.promotionValidation = null;
          state.tenderInsights = {};
          persist();
          await loadHeldSales();
          notify(text("Checkout completed.", "Checkout selesai."));
          render();
          if (window.confirm(text("Print receipt now?", "Cetak struk sekarang?"))) {
            await printReceipt();
          } else {
            state.lastCheckoutResult = null;
            render();
          }
        } catch (error) {
          notify(error instanceof Error ? error.message : text("Checkout failed.", "Checkout gagal."), "error");
        } finally {
          state.busy = false;
          render();
        }
      }

      async function waitForReceiptAssets() {
        await nextFrame();
        await nextFrame();
        const receipt = mount.querySelector(".pos-terminal__receipt-print");
        if (!receipt) return;
        const images = Array.from(receipt.querySelectorAll(".pos-terminal__receipt-lookup-qr"));
        if (!images.length) return;
        await Promise.all(images.map(function(image) {
          if (image.complete && image.naturalWidth > 0) return Promise.resolve();
          return new Promise(function(resolve) {
            let settled = false;
            const finish = function() {
              if (settled) return;
              settled = true;
              image.removeEventListener("load", finish);
              image.removeEventListener("error", finish);
              window.clearTimeout(timeoutID);
              resolve();
            };
            const timeoutID = window.setTimeout(finish, 4000);
            image.addEventListener("load", finish, { once: true });
            image.addEventListener("error", finish, { once: true });
          });
        }));
      }

      async function printReceipt() {
        if (!state.lastCheckoutResult || state.receiptPrinting) return;
        state.receiptPrinting = true;
        render();
        try {
          await waitForReceiptAssets();
          await new Promise(function(resolve) {
            let settled = false;
            const finish = function() {
              if (settled) return;
              settled = true;
              window.removeEventListener("afterprint", finish);
              window.clearTimeout(timeoutID);
              resolve();
            };
            const timeoutID = window.setTimeout(finish, 1500);
            window.addEventListener("afterprint", finish, { once: true });
            window.print();
          });
        } finally {
          state.receiptPrinting = false;
          state.lastCheckoutResult = null;
          render();
        }
      }

      async function lookupTransactions() {
        state.lookupBusy = true;
        render();
        try {
          const query = new URLSearchParams();
          if (state.lookupQuery) query.set("q", state.lookupQuery);
          if (state.storeCode) query.set("store_code", state.storeCode);
          if (state.registerCode) query.set("register_code", state.registerCode);
          const payload = await api("/ui/data/pos/transactions/search?" + query.toString());
          state.transactions = payload.items || [];
        } finally {
          state.lookupBusy = false;
          render();
        }
      }

      async function transactAction(saleID, action) {
        const payload = await api("/ui/data/pos/transactions/" + encodeURIComponent(saleID) + "/" + action, { method: "POST" });
        const label = action === "refund-store-credit"
          ? text("Store credit refund documents created.", "Dokumen refund store credit dibuat.")
          : action === "refund"
            ? text("Refund documents created.", "Dokumen refund dibuat.")
            : text("Exchange documents created.", "Dokumen tukar dibuat.");
        notify(label);
        emitHardwareEvent("orbyte:pos-payment-terminal-request", { action: action, payload: payload });
        await lookupTransactions();
      }

      function renderStoreOptions(items, valueKey, labelKey, current) {
        return ['<option value="">' + escapeHTML(text("Select", "Pilih")) + '</option>'].concat((items || []).map(function(item) {
          const values = item.values || {};
          const value = String(values[valueKey] || "");
          const selected = value === current ? " selected" : "";
          return '<option value="' + escapeHTML(value) + '"' + selected + ">" + escapeHTML(String(values[labelKey] || value)) + "</option>";
        })).join("");
      }

      function renderSetupState(bootstrap) {
        const missing = [];
        if (!(bootstrap.stores || []).length) missing.push(text("POS stores", "toko POS"));
        if (!(bootstrap.registers || []).length) missing.push(text("registers", "register"));
        if (!(bootstrap.tender_types || []).length) missing.push(text("tender types", "jenis tender"));
        return ''
          + '<section class="pos-terminal__setup">'
          +   '<div class="pos-terminal__stack">'
          +     '<strong>' + escapeHTML(text("Terminal setup is incomplete.", "Setup terminal belum lengkap.")) + '</strong>'
          +     '<div>' + escapeHTML(text("Create POS stores, registers, and tender types before the cashier can start selling.", "Buat toko POS, register, dan jenis tender sebelum kasir mulai berjualan.")) + '</div>'
          +     '<div class="pos-terminal__muted">' + escapeHTML(text("Missing: ", "Belum ada: ")) + escapeHTML(missing.join(", ")) + '</div>'
          +     '<div class="pos-terminal__buttons" style="justify-content:center">'
          +       '<button type="button" class="pos-terminal__button pos-terminal__button--primary" data-nav="/ui/pos/stores">' + escapeHTML(text("Open POS setup", "Buka setup POS")) + '</button>'
          +       '<button type="button" class="pos-terminal__button" data-nav="/ui/backoffice">' + escapeHTML(text("Back to backoffice", "Kembali ke backoffice")) + '</button>'
          +     '</div>'
          +   '</div>'
          + '</section>';
      }

      function renderCustomerSummary() {
        if (!state.customer) {
          return '<div class="pos-terminal__summary-pill">' + escapeHTML(text("No member attached", "Belum ada member")) + '</div>';
        }
        return ''
          + '<div class="pos-terminal__summary-pill">'
          +   '<div class="pos-terminal__mini-stack">'
          +     '<div class="pos-terminal__customer-name">' + escapeHTML(state.customer.customer_name || state.customer.party_id || "") + '</div>'
          +     '<div class="pos-terminal__customer-meta">' + escapeHTML(state.customer.party_id || "") + '</div>'
          +   '</div>'
          +   '<div class="pos-terminal__buttons">'
          +     '<button type="button" class="pos-terminal__button" data-action="clear-customer">' + escapeHTML(text("Clear", "Lepas")) + '</button>'
          +   '</div>'
          + '</div>';
      }

      function renderCustomerResults() {
        if (!state.customerResults.length) {
          return '<div class="pos-terminal__empty">' + escapeHTML(state.customerBusy ? text("Searching customers…", "Mencari pelanggan…") : text("Search by member ID, customer name, or party ID.", "Cari dengan ID member, nama pelanggan, atau ID party.")) + '</div>';
        }
        return '<div class="pos-terminal__customer-results">' + state.customerResults.map(function(item, index) {
          return ''
            + '<article class="pos-terminal__customer-card">'
            +   '<div>'
            +     '<div class="pos-terminal__customer-name">' + escapeHTML(item.customer_name || item.party_id || "") + '</div>'
            +     '<div class="pos-terminal__customer-meta">' + escapeHTML(String(item.party_id || "")) + '</div>'
            +   '</div>'
            +   '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-attach-customer="' + String(index) + '">' + escapeHTML(text("Attach", "Pakai")) + '</button></div>'
            + '</article>';
        }).join("") + '</div>';
      }

      function renderCatalogModal() {
        if (!state.catalogOpen) return "";
        return ''
          + '<div class="pos-terminal__overlay" data-action="close-catalog-overlay">'
          +   '<section class="pos-terminal__modal" role="dialog" aria-modal="true" aria-label="' + escapeHTML(text("Catalog search", "Pencarian katalog")) + '">'
          +     '<div class="pos-terminal__modal-head">'
          +       '<div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Catalog search", "Pencarian katalog")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Search by barcode first, then add items directly into the basket.", "Cari barcode lebih dulu, lalu tambah item langsung ke basket.")) + '</div></div>'
          +       '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="close-catalog">' + escapeHTML(text("Close", "Tutup")) + '</button></div>'
          +     '</div>'
          +     '<div class="pos-terminal__modal-body">'
          +     '<form class="pos-terminal__row" data-form="catalog-search">'
          +       '<div class="pos-terminal__field"><span>' + escapeHTML(text("Barcode or item", "Barcode atau item")) + '</span><input id="pos-search" name="pos_search" placeholder="' + escapeHTML(text("Scan barcode or type item name", "Scan barcode atau ketik nama barang")) + '" value="' + escapeHTML(state.searchQuery) + '"></div>'
          +       '<div class="pos-terminal__buttons"><button type="submit" class="pos-terminal__button pos-terminal__button--primary" data-action="search">' + escapeHTML(state.searchBusy ? text("Searching…", "Mencari…") : text("Search", "Cari")) + '</button></div>'
          +     '</form>'
          +     '<div class="pos-terminal__scroll">' + (state.searchBusy
              ? '<div class="pos-terminal__empty">' + escapeHTML(text("Searching catalog…", "Mencari katalog…")) + '</div>'
              : state.searchResults.length ? '<div class="pos-terminal__result-list">' + state.searchResults.map(function(item, index) {
                return ''
                  + '<article class="pos-terminal__result">'
                  +   '<div class="pos-terminal__result-head">'
                  +     '<div><div class="pos-terminal__title">' + escapeHTML(item.name || item.item_code) + '</div><div class="pos-terminal__muted">' + escapeHTML((item.item_code || "") + (item.variant_label ? " • " + item.variant_label : "")) + '</div></div>'
                  +     '<div><strong>' + escapeHTML(money(item.unit_price)) + '</strong></div>'
                  +   '</div>'
                  +   '<div class="pos-terminal__muted" style="margin-top:0.55rem">' + (item.inventory_enabled ? escapeHTML(text("Available", "Tersedia")) + ': ' + escapeHTML(String(number(item.available_quantity))) : escapeHTML(text("Non-stock item", "Item nonstok"))) + '</div>'
                  +   '<div class="pos-terminal__buttons" style="margin-top:0.75rem"><button type="button" class="pos-terminal__button" data-add-result="' + String(index) + '">' + escapeHTML(text("Add to cart", "Tambah ke keranjang")) + '</button></div>'
                  + '</article>';
              }).join("") + '</div>' : '<div class="pos-terminal__empty">' + escapeHTML(text("Search results appear here.", "Hasil pencarian muncul di sini.")) + '</div>') + '</div>'
          +     '</div>'
          +   '</section>'
          + '</div>';
      }

      function renderCartPanel() {
        const promoCodes = appliedPromotionCodes();
        return ''
          + '<article class="pos-terminal__panel">'
          +   '<div class="pos-terminal__panel-head">'
          +     '<div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Cart", "Keranjang")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Keep the basket clean before tendering.", "Pastikan keranjang rapi sebelum tender.")) + '</div></div>'
          +     '<div class="pos-terminal__buttons">' + (state.cart.length ? '<span class="pos-terminal__chip">' + escapeHTML(String(state.cart.length)) + " " + escapeHTML(text("lines", "baris")) + '</span>' : "") + '<button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="open-catalog">' + escapeHTML(text("Find items", "Cari item")) + '</button></div>'
          +   '</div>'
          +   '<div class="pos-terminal__panel-body" style="display:grid;grid-template-rows:auto minmax(0,1fr);gap:0.9rem">'
          +     '<div class="pos-terminal__mini-meta">'
          +       renderCustomerSummary()
          +       '<div class="pos-terminal__summary-chipline">'
          +         '<button type="button" class="pos-terminal__button" data-action="open-customer">' + escapeHTML(state.customer ? text("Change member", "Ganti member") : text("Find member", "Cari member")) + '</button>'
          +         '<button type="button" class="pos-terminal__button" data-action="open-promo">' + escapeHTML(text("Promo / voucher", "Promo / voucher")) + '</button>'
          +         (promoCodes.length ? promoCodes.slice(0, 2).map(function(code) { return '<span class="pos-terminal__chip">' + escapeHTML(code) + '</span>'; }).join("") : '<span class="pos-terminal__summary-pill">' + escapeHTML(text("No promo applied", "Belum ada promo")) + '</span>')
          +         (promoCodes.length > 2 ? '<span class="pos-terminal__chip">+' + escapeHTML(String(promoCodes.length - 2)) + '</span>' : '')
          +       '</div>'
          +     '</div>'
          +     '<div class="pos-terminal__scroll">' + (state.cart.length ? '<table class="pos-terminal__cart-table"><thead><tr><th style="width:34%">' + escapeHTML(text("Item", "Item")) + '</th><th style="width:13%">' + escapeHTML(text("Qty", "Qty")) + '</th><th style="width:16%">' + escapeHTML(text("Price", "Harga")) + '</th><th style="width:16%">' + escapeHTML(text("Discount", "Diskon")) + '</th><th style="width:13%">' + escapeHTML(text("Total", "Total")) + '</th><th style="width:8%"></th></tr></thead><tbody>' + state.cart.map(function(line, index) {
                return '<tr><td><div class="pos-terminal__title">' + escapeHTML(line.description || line.item_code) + '</div><div class="pos-terminal__muted">' + escapeHTML(String(line.item_code || "")) + '</div>' + (line.inventory_enabled ? '<div class="pos-terminal__muted" style="margin-top:0.25rem">' + escapeHTML(text("Available", "Tersedia")) + ': ' + escapeHTML(String(number(line.available_quantity))) + '</div>' : '') + '</td><td><input id="pos-line-qty-' + String(index) + '" name="pos_line_qty_' + String(index) + '" type="number" min="0" step="1" data-line-qty="' + String(index) + '" value="' + escapeHTML(Object.prototype.hasOwnProperty.call(state.cartQuantityDrafts, String(index)) ? String(state.cartQuantityDrafts[String(index)]) : String(line.quantity || 0)) + '"></td><td><span class="pos-terminal__readonly">' + escapeHTML(money(line.unit_price || 0)) + '</span></td><td><span class="pos-terminal__readonly">' + escapeHTML(money(line.discount_amount || 0)) + '</span></td><td><strong>' + escapeHTML(money(line.line_total || 0)) + '</strong></td><td><button type="button" class="pos-terminal__button pos-terminal__button--warn" data-remove-line="' + String(index) + '">' + escapeHTML(text("Remove", "Hapus")) + '</button></td></tr>';
              }).join("") + '</tbody></table>' : '<div class="pos-terminal__empty">' + escapeHTML(text("No items in cart yet.", "Belum ada item di keranjang.")) + '</div>') + '</div>'
          +   '</div>'
          + '</article>';
      }

      function renderAuxPanel() {
        return ''
          + '<div class="pos-terminal__aux">'
          +   '<article class="pos-terminal__panel' + (state.heldExpanded ? '' : ' pos-terminal__panel--collapsed') + '">'
          +     '<div class="pos-terminal__panel-head"><div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Held sales", "Penjualan tertahan")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Resume suspended baskets without leaving the terminal.", "Lanjutkan basket tertahan tanpa keluar dari terminal.")) + '</div></div><div class="pos-terminal__buttons"><button type="button" class="pos-terminal__toggle" data-action="toggle-held">' + escapeHTML(state.heldExpanded ? text("Collapse", "Tutup") : text("Expand", "Buka")) + '</button></div></div>'
          +     (state.heldExpanded
                ? '<div class="pos-terminal__panel-body pos-terminal__scroll">' + (state.heldSales.length ? '<div class="pos-terminal__held-list">' + state.heldSales.map(function(item, index) {
                const values = item.values || {};
                return '<article class="pos-terminal__held"><div class="pos-terminal__sale-head"><div><div class="pos-terminal__title">' + escapeHTML(String(values.sale_number || item.id || "")) + '</div><div class="pos-terminal__muted">' + escapeHTML(String(values.party_name || text("Walk-in customer", "Pelanggan umum"))) + '</div></div><strong>' + escapeHTML(money(values.total_amount || 0)) + '</strong></div><div class="pos-terminal__buttons" style="margin-top:0.75rem"><button type="button" class="pos-terminal__button" data-resume-held="' + String(index) + '">' + escapeHTML(text("Resume", "Lanjutkan")) + '</button></div></article>';
              }).join("") + '</div>' : '<div class="pos-terminal__empty">' + escapeHTML(text("No held sales for this register.", "Tidak ada penjualan tertahan untuk register ini.")) + '</div>') + '</div>'
                : '')
          +   '</article>'
          +   '<article class="pos-terminal__panel' + (state.transactionsExpanded ? '' : ' pos-terminal__panel--collapsed') + '">'
          +     '<div class="pos-terminal__panel-head"><div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Transaction lookup", "Pencarian transaksi")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Refund, exchange, or convert to store credit from here.", "Refund, tukar, atau ubah ke store credit dari sini.")) + '</div></div><div class="pos-terminal__buttons"><button type="button" class="pos-terminal__toggle" data-action="toggle-transactions">' + escapeHTML(state.transactionsExpanded ? text("Collapse", "Tutup") : text("Expand", "Buka")) + '</button></div></div>'
          +     (state.transactionsExpanded
                ? '<div class="pos-terminal__panel-body" style="display:grid;grid-template-rows:auto minmax(0,1fr);gap:0.9rem">'
          +       '<div class="pos-terminal__row"><div class="pos-terminal__field"><span>' + escapeHTML(text("Lookup", "Cari")) + '</span><input id="pos-lookup" name="pos_lookup" value="' + escapeHTML(state.lookupQuery) + '" placeholder="' + escapeHTML(text("Sale number, customer, invoice", "Nomor jual, pelanggan, invoice")) + '"></div><div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="lookup-transactions">' + escapeHTML(state.lookupBusy ? text("Loading…", "Memuat…") : text("Lookup", "Cari")) + '</button></div></div>'
          +       '<div class="pos-terminal__scroll">' + (state.transactions.length ? '<div class="pos-terminal__txn-list">' + state.transactions.map(function(item, index) {
                const values = item.values || {};
                return '<article class="pos-terminal__txn"><div class="pos-terminal__sale-head"><div><div class="pos-terminal__title">' + escapeHTML(String(values.sale_number || "")) + '</div><div class="pos-terminal__muted">' + escapeHTML(String(values.invoice_number || values.order_number || "")) + (values.party_name ? " • " + escapeHTML(String(values.party_name || "")) : "") + '</div></div><strong>' + escapeHTML(money(values.total_amount || 0)) + '</strong></div><div class="pos-terminal__buttons" style="margin-top:0.75rem"><button type="button" class="pos-terminal__button" data-refund-sale="' + String(index) + '">' + escapeHTML(text("Refund", "Refund")) + '</button><button type="button" class="pos-terminal__button" data-exchange-sale="' + String(index) + '">' + escapeHTML(text("Exchange", "Tukar")) + '</button><button type="button" class="pos-terminal__button pos-terminal__button--soft" data-refund-store-credit="' + String(index) + '">' + escapeHTML(text("Store credit", "Store credit")) + '</button></div></article>';
              }).join("") + '</div>' : '<div class="pos-terminal__empty">' + escapeHTML(text("No transactions loaded yet.", "Belum ada transaksi dimuat.")) + '</div>') + '</div>'
          +     '</div>'
                : '')
          +   '</article>'
          + '</div>';
      }

      function renderTenderCard(line, index) {
        const kind = tenderKind(line.tender_type_code);
        const insight = state.tenderInsights[String(index)] || null;
        const statusBlock = kind === "gift_card" || kind === "store_credit"
          ? '<div class="pos-terminal__muted" style="margin-top:0.65rem">'
              + (insight && insight.loading ? escapeHTML(text("Checking balance…", "Memeriksa saldo…"))
                : insight && insight.error ? escapeHTML(String(insight.error || ""))
                : insight && kind === "gift_card" ? escapeHTML(text("Balance", "Saldo")) + ": " + escapeHTML(money(insight.remaining_balance || 0)) + " • " + escapeHTML(String(insight.status || ""))
                : insight && kind === "store_credit" ? escapeHTML(text("Available", "Tersedia")) + ": " + escapeHTML(money(insight.balance_amount || 0)) + " • " + escapeHTML(String(insight.status || ""))
                : kind === "gift_card" ? escapeHTML(text("Enter the gift card code, then check its balance.", "Masukkan kode gift card, lalu cek saldonya."))
                : escapeHTML(state.customer && state.customer.party_id ? text("Load the attached customer's balance before checkout.", "Muat saldo pelanggan terpasang sebelum checkout.") : text("Attach a customer before using store credit.", "Pasang pelanggan sebelum memakai store credit.")))
            + '</div>'
          : kind === "voucher"
            ? '<div class="pos-terminal__muted" style="margin-top:0.65rem">' + escapeHTML(text("Use the promo / voucher field to capture voucher codes. Reference can store the issuer slip or approval number.", "Gunakan field promo / voucher untuk memasukkan kode voucher. Referensi bisa menyimpan nomor slip atau approval.")) + '</div>'
            : '';
        return ''
          + '<article class="pos-terminal__tender">'
          +   '<div class="pos-terminal__sale-head"><div><div class="pos-terminal__title">' + escapeHTML(text("Tender", "Tender")) + " " + escapeHTML(String(index + 1)) + '</div><div class="pos-terminal__muted">' + escapeHTML(kind || text("Select a tender type", "Pilih jenis tender")) + '</div></div><button type="button" class="pos-terminal__button" data-remove-tender="' + String(index) + '">' + escapeHTML(text("Remove", "Hapus")) + '</button></div>'
          +   '<div class="pos-terminal__row" style="margin-top:0.8rem">'
          +     '<div class="pos-terminal__field"><span>' + escapeHTML(text("Tender type", "Jenis tender")) + '</span><select id="pos-tender-type-' + String(index) + '" name="pos_tender_type_' + String(index) + '" data-tender-type="' + String(index) + '">' + ['<option value="">' + escapeHTML(text("Select", "Pilih")) + '</option>'].concat(tenderTypes().map(function(item) {
                  const code = String((item.values || {}).code || "");
                  const selected = code === String(line.tender_type_code || "") ? " selected" : "";
                  return '<option value="' + escapeHTML(code) + '"' + selected + ">" + escapeHTML(String((item.values || {}).name || code)) + "</option>";
                })).join("") + '</select></div>'
          +     '<div class="pos-terminal__field"><span>' + escapeHTML(text("Amount", "Jumlah")) + '</span><input id="pos-tender-amount-' + String(index) + '" name="pos_tender_amount_' + String(index) + '" type="number" min="0" step="0.01" data-tender-amount="' + String(index) + '" value="' + escapeHTML(Object.prototype.hasOwnProperty.call(state.tenderAmountDrafts, String(index)) ? String(state.tenderAmountDrafts[String(index)]) : String(line.amount || 0)) + '"></div>'
          +   '</div>'
          +   '<div class="pos-terminal__row" style="margin-top:0.7rem">'
          +     '<div class="pos-terminal__field"><span>' + escapeHTML(tenderRequiresReference(line.tender_type_code) ? text("Reference (required)", "Referensi (wajib)") : text("Reference", "Referensi")) + '</span><input id="pos-tender-reference-' + String(index) + '" name="pos_tender_reference_' + String(index) + '" type="text" data-tender-reference="' + String(index) + '" value="' + escapeHTML(String(line.reference || "")) + '" placeholder="' + escapeHTML(kind === "gift_card" ? text("Gift card code", "Kode gift card") : kind === "voucher" ? text("Voucher reference", "Referensi voucher") : text("Optional reference", "Referensi opsional")) + '"></div>'
          +   '</div>'
          +   statusBlock
          +   ((kind === "gift_card") || (kind === "store_credit" && state.customer && state.customer.party_id) ? '<div class="pos-terminal__buttons" style="margin-top:0.7rem"><button type="button" class="pos-terminal__button" data-check-tender="' + String(index) + '">' + escapeHTML(kind === "gift_card" ? text("Check balance", "Cek saldo") : text("Load balance", "Muat saldo")) + '</button></div>' : '')
          + '</article>';
      }

      function renderRail(totalsPayload) {
        const activeStore = selectedStore();
        const activeRegister = selectedRegister();
        const tenderCount = state.tenders.filter(function(line) {
          return String(line.tender_type_code || "").trim() !== "" || number(line.amount) > 0 || String(line.reference || "").trim() !== "";
        }).length;
        return ''
          + '<aside class="pos-terminal__rail">'
          +   '<article class="pos-terminal__panel">'
          +     '<div class="pos-terminal__panel-head"><div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Checkout summary", "Ringkasan checkout")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(state.shiftId ? text("Shift is open and ready to settle.", "Shift terbuka dan siap settle.") : text("Open a shift before accepting payments.", "Buka shift sebelum menerima pembayaran.")) + '</div></div></div>'
          +     '<div class="pos-terminal__panel-body">'
          +       '<div class="pos-terminal__due"><span class="pos-terminal__meta-label">' + escapeHTML(text("Amount due", "Jumlah kurang")) + '</span><strong>' + escapeHTML(money(totalsPayload.due)) + '</strong><small>' + escapeHTML(totalsPayload.change > 0 ? text("Change due: ", "Kembalian: ") + money(totalsPayload.change) : text("Tender until due becomes zero.", "Tender hingga nilai kurang menjadi nol.")) + '</small></div>'
          +       '<div class="pos-terminal__summary" style="margin-top:0.95rem">'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Store", "Toko")) + '</span><span>' + escapeHTML(String((activeStore && activeStore.values && activeStore.values.name) || state.storeCode || "—")) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Register", "Register")) + '</span><span>' + escapeHTML(String((activeRegister && activeRegister.values && activeRegister.values.name) || state.registerCode || "—")) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Subtotal", "Subtotal")) + '</span><span>' + escapeHTML(money(totalsPayload.subtotal)) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Tax", "Pajak")) + '</span><span>' + escapeHTML(money(totalsPayload.tax)) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><strong>' + escapeHTML(text("Total", "Total")) + '</strong><strong>' + escapeHTML(money(totalsPayload.total)) + '</strong></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Tendered", "Dibayar")) + '</span><span>' + escapeHTML(money(totalsPayload.tendered)) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Change", "Kembalian")) + '</span><span>' + escapeHTML(money(totalsPayload.change)) + '</span></div>'
          +         '<div class="pos-terminal__summary-row"><span>' + escapeHTML(text("Tender lines", "Baris tender")) + '</span><span>' + escapeHTML(String(tenderCount || 0)) + '</span></div>'
          +       '</div>'
          +     '</div>'
          +   '</article>'
          +   '<article class="pos-terminal__panel">'
          +     '<div class="pos-terminal__panel-head"><div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Tenders", "Tender")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Open the tender workspace to capture payments, voucher references, and stored-value checks.", "Buka workspace tender untuk menangkap pembayaran, referensi voucher, dan cek stored-value.")) + '</div></div></div>'
          +     '<div class="pos-terminal__panel-body"><div class="pos-terminal__buttons" style="justify-content:space-between"><span class="pos-terminal__summary-pill">' + escapeHTML(tenderCount ? text("Tender lines ready", "Baris tender siap") : text("No tender captured", "Belum ada tender")) + (tenderCount ? ': ' + escapeHTML(String(tenderCount)) : '') + '</span><button type="button" class="pos-terminal__button" data-action="open-tender">' + escapeHTML(text("Manage tenders", "Kelola tender")) + '</button></div></div>'
          +   '</article>'
          +   '<article class="pos-terminal__panel">'
          +     '<div class="pos-terminal__panel-body">'
          +       '<div class="pos-terminal__buttons" style="justify-content:space-between">'
          +         '<button type="button" class="pos-terminal__button" data-action="hold">' + escapeHTML(text("Hold sale", "Tahan penjualan")) + '</button>'
          +         '<button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="checkout"' + (state.busy ? " disabled" : "") + '>' + escapeHTML(state.busy ? text("Processing…", "Memproses…") : text("Complete sale", "Selesaikan penjualan")) + '</button>'
          +       '</div>'
          +     '</div>'
          +   '</article>'
          + '</aside>';
      }

      function renderControlBar() {
        const bootstrap = state.bootstrap || { stores: [], registers: [], tender_types: [] };
        const cashier = currentCashier();
        const context = terminalContext();
        const activeShift = (bootstrap.open_shift || {});
        const isCollapsed = state.controlCollapsed !== false;
        const cashierLabel = String((cashier && (cashier.name || cashier.username || cashier.user_id)) || "—");
        const cashierSub = String((cashier && (cashier.username || cashier.user_id)) || "");
        const storeLabel = String((selectedStore() && selectedStore().values && selectedStore().values.name) || state.storeCode || "—");
        const registerLabel = String((selectedRegister() && selectedRegister().values && selectedRegister().values.name) || state.registerCode || "—");
        const shiftLabel = String(((activeShift.values || {}).shift_number || context && context.shift_id || state.shiftId || "—"));
        const shiftSub = String(((activeShift.values || {}).status || text("opened", "terbuka")));
        return ''
          + '<section class="pos-terminal__controlbar' + (isCollapsed ? ' pos-terminal__controlbar--compact' : '') + '">'
          +   '<div class="pos-terminal__control-grid">'
          +     '<div class="pos-terminal__control-summary">'
          +       '<article class="pos-terminal__control-pill"><span class="pos-terminal__meta-label">' + escapeHTML(text("Cashier", "Kasir")) + '</span><strong>' + escapeHTML(cashierLabel) + '</strong>' + (isCollapsed ? '' : '<div class="pos-terminal__meta-sub">' + escapeHTML(cashierSub) + '</div>') + '</article>'
          +       '<article class="pos-terminal__control-pill"><span class="pos-terminal__meta-label">' + escapeHTML(text("Store", "Toko")) + '</span><strong>' + escapeHTML(storeLabel) + '</strong>' + (isCollapsed ? '' : '<div class="pos-terminal__meta-sub">' + escapeHTML(String(state.storeCode || "")) + '</div>') + '</article>'
          +       '<article class="pos-terminal__control-pill"><span class="pos-terminal__meta-label">' + escapeHTML(text("Register", "Register")) + '</span><strong>' + escapeHTML(registerLabel) + '</strong>' + (isCollapsed ? '' : '<div class="pos-terminal__meta-sub">' + escapeHTML(String(state.registerCode || "")) + '</div>') + '</article>'
          +       '<article class="pos-terminal__control-pill"><span class="pos-terminal__meta-label">' + escapeHTML(text("Shift", "Shift")) + '</span><strong>' + escapeHTML(shiftLabel) + '</strong>' + (isCollapsed ? '' : '<div class="pos-terminal__meta-sub">' + escapeHTML(shiftSub) + '</div>') + '</article>'
          +     '</div>'
          +     '<div class="pos-terminal__control-actions">'
          +       '<button type="button" class="pos-terminal__button pos-terminal__button--compact" data-action="toggle-control-details">' + escapeHTML(isCollapsed ? text("Details", "Detail") : text("Hide details", "Sembunyikan detail")) + '</button>'
          +       (state.shiftId ? '<button type="button" class="pos-terminal__button pos-terminal__button--warn pos-terminal__button--compact" data-action="close-shift">' + escapeHTML(text("Close shift", "Tutup shift")) + '</button>' : '<button type="button" class="pos-terminal__button pos-terminal__button--primary pos-terminal__button--compact" data-action="open-shift">' + escapeHTML(text("Open shift", "Buka shift")) + '</button>')
          +       '<button type="button" class="pos-terminal__button pos-terminal__button--compact" data-action="lock-terminal">' + escapeHTML(text("Lock", "Kunci")) + '</button>'
          +       '<button type="button" class="pos-terminal__button pos-terminal__button--compact" data-action="refresh-terminal">' + escapeHTML(text("Refresh", "Muat ulang")) + '</button>'
          +     '</div>'
          +   '</div>'
          +   (!isCollapsed
                ? '<div class="pos-terminal__control-selects">'
                  + '<article class="pos-terminal__meta"><span class="pos-terminal__meta-label">' + escapeHTML(text("Cashier", "Kasir")) + '</span><div class="pos-terminal__meta-value">' + escapeHTML(cashierLabel) + '</div><div class="pos-terminal__meta-sub">' + escapeHTML(cashierSub) + '</div></article>'
                  + '<article class="pos-terminal__meta"><span class="pos-terminal__meta-label">' + escapeHTML(text("Store", "Toko")) + '</span><div class="pos-terminal__meta-value">' + escapeHTML(storeLabel) + '</div><div class="pos-terminal__meta-sub">' + escapeHTML(String(state.storeCode || "")) + '</div></article>'
                  + '<article class="pos-terminal__meta"><span class="pos-terminal__meta-label">' + escapeHTML(text("Register", "Register")) + '</span><div class="pos-terminal__meta-value">' + escapeHTML(registerLabel) + '</div><div class="pos-terminal__meta-sub">' + escapeHTML(String(state.registerCode || "")) + '</div></article>'
                  + '<article class="pos-terminal__meta"><span class="pos-terminal__meta-label">' + escapeHTML(text("Shift", "Shift")) + '</span><div class="pos-terminal__meta-value">' + escapeHTML(shiftLabel) + '</div><div class="pos-terminal__meta-sub">' + escapeHTML(shiftSub) + '</div></article>'
                  + '</div>'
                : '')
          +   '<div class="pos-terminal__control-meta">'
          +       '<span class="pos-terminal__chip ' + (navigator.onLine ? 'pos-terminal__chip--ok' : 'pos-terminal__chip--danger') + '">' + escapeHTML(navigator.onLine ? text("Online", "Online") : text("Offline", "Offline")) + '</span>'
          +       '<span class="pos-terminal__chip ' + (state.shiftId ? 'pos-terminal__chip--ok' : 'pos-terminal__chip--warn') + '">' + escapeHTML(state.shiftId ? text("Shift open", "Shift terbuka") : text("Shift closed", "Shift tutup")) + '</span>'
          +       '<span class="pos-terminal__chip">' + escapeHTML(text("Stores", "Toko")) + ': ' + escapeHTML(String((bootstrap.stores || []).length)) + '</span>'
          +       '<span class="pos-terminal__chip">' + escapeHTML(text("Tenders", "Tender")) + ': ' + escapeHTML(String((bootstrap.tender_types || []).length)) + '</span>'
          +       (state.bootstrapping ? '<span class="pos-terminal__chip pos-terminal__chip--warn">' + escapeHTML(text("Refreshing terminal…", "Memuat ulang terminal…")) + '</span>' : '')
          +   '</div>'
          + '</section>';
      }

      function renderTerminalGate(bootstrap) {
        const cashier = currentCashier();
        const matchingShift = bootstrap.open_shift || null;
        return ''
          + '<section class="pos-terminal__setup">'
          +   '<div class="pos-terminal__stack" style="width:min(46rem,100%);text-align:left">'
          +     '<div style="display:grid;gap:0.5rem">'
          +       '<strong style="font-size:1.2rem">' + escapeHTML(text("Unlock terminal", "Buka terminal")) + '</strong>'
          +       '<div>' + escapeHTML(text("Select the counter context, confirm the cashier, then enter your cashier PIN to resume or open a shift.", "Pilih konteks counter, konfirmasi kasir, lalu masukkan PIN kasir untuk melanjutkan atau membuka shift.")) + '</div>'
          +     '</div>'
          +     '<div class="pos-terminal__row">'
          +       '<div class="pos-terminal__field"><span>' + escapeHTML(text("Cashier", "Kasir")) + '</span><div class="pos-terminal__readonly">' + escapeHTML(String((cashier && (cashier.name || cashier.username || cashier.user_id)) || "—")) + '<div class="pos-terminal__muted">' + escapeHTML(String((cashier && (cashier.username || cashier.user_id)) || "")) + '</div></div></div>'
          +       '<div class="pos-terminal__field"><span>' + escapeHTML(text("Store", "Toko")) + '</span><select id="pos-store" name="pos_store">' + renderStoreOptions(bootstrap.stores, "code", "name", state.storeCode) + '</select></div>'
          +       '<div class="pos-terminal__field"><span>' + escapeHTML(text("Register", "Register")) + '</span><select id="pos-register" name="pos_register">' + renderStoreOptions((bootstrap.registers || []).filter(function(item) { return !state.storeCode || String((item.values || {}).store_code || "") === state.storeCode; }), "code", "name", state.registerCode) + '</select></div>'
          +     '</div>'
          +     (!bootstrap.cashier_pin_configured ? '<div class="pos-terminal__notice">' + escapeHTML(text("Cashier PIN is not set yet. Set it once in Settings before using this terminal.", "PIN kasir belum disetel. Atur sekali di Pengaturan sebelum memakai terminal ini.")) + ' <button type="button" class="pos-terminal__button pos-terminal__button--compact" data-nav="/ui/settings">' + escapeHTML(text("Open settings", "Buka pengaturan")) + '</button></div>' : '')
          +     '<div class="pos-terminal__row">'
          +       '<div class="pos-terminal__field"><span>' + escapeHTML(text("Cashier PIN", "PIN kasir")) + '</span><input id="pos-terminal-pin" name="pos_terminal_pin" type="password" inputmode="numeric" autocomplete="one-time-code" placeholder="' + escapeHTML(text("6-digit PIN", "PIN 6 digit")) + '" value="' + escapeHTML(String(state.terminalPIN || "")) + '"></div>'
          +       '<div class="pos-terminal__field"><span>' + escapeHTML(text("Opening cash", "Kas awal")) + '</span><input id="pos-opening-cash" name="pos_opening_cash" type="number" min="0" step="0.01" value="' + escapeHTML(String(state.terminalOpeningCash || "0")) + '"></div>'
          +       '<div class="pos-terminal__field"><span>' + escapeHTML(text("Notes", "Catatan")) + '</span><input id="pos-terminal-notes" name="pos_terminal_notes" type="text" value="' + escapeHTML(String(state.terminalNotes || "")) + '" placeholder="' + escapeHTML(text("Optional opening note", "Catatan pembukaan opsional")) + '"></div>'
          +     '</div>'
          +     (matchingShift ? '<div class="pos-terminal__notice">' + escapeHTML(text("Matching open shift found for this cashier and register.", "Ditemukan shift terbuka yang cocok untuk kasir dan register ini.")) + ' <strong>' + escapeHTML(String(((matchingShift.values || {}).shift_number || matchingShift.id || ""))) + '</strong></div>' : '')
          +     '<div class="pos-terminal__buttons">'
          +       (matchingShift ? '<button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="resume-terminal"' + (state.terminalBusy || !bootstrap.cashier_pin_configured ? " disabled" : "") + '>' + escapeHTML(state.terminalBusy ? text("Entering…", "Masuk…") : text("Resume shift", "Lanjutkan shift")) + '</button>' : '<button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="open-shift"' + (state.terminalBusy || !bootstrap.cashier_pin_configured ? " disabled" : "") + '>' + escapeHTML(state.terminalBusy ? text("Entering…", "Masuk…") : text("Open shift", "Buka shift")) + '</button>')
          +       '<button type="button" class="pos-terminal__button" data-action="refresh-terminal">' + escapeHTML(text("Refresh", "Muat ulang")) + '</button>'
          +     '</div>'
          +   '</div>'
          + '</section>';
      }

      function renderCustomerModal() {
        if (!state.customerModalOpen) return "";
        return ''
          + '<div class="pos-terminal__overlay" data-action="close-customer-overlay">'
          +   '<section class="pos-terminal__modal" role="dialog" aria-modal="true" aria-label="' + escapeHTML(text("Member search", "Pencarian member")) + '">'
          +     '<div class="pos-terminal__modal-head">'
          +       '<div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Member search", "Pencarian member")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Search by member ID, customer name, or party ID, then attach the result to this sale.", "Cari dengan ID member, nama pelanggan, atau ID party, lalu pasang ke transaksi ini.")) + '</div></div>'
          +       '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="close-customer">' + escapeHTML(text("Close", "Tutup")) + '</button></div>'
          +     '</div>'
          +     '<div class="pos-terminal__modal-body">'
          +       '<div class="pos-terminal__row">'
          +         '<div class="pos-terminal__field"><span>' + escapeHTML(text("Customer or member", "Pelanggan atau member")) + '</span><input id="pos-customer-search" name="pos_customer_search" placeholder="' + escapeHTML(text("Member ID, customer name, or party ID", "ID member, nama pelanggan, atau ID party")) + '" value="' + escapeHTML(state.customerQuery) + '"></div>'
          +         '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button pos-terminal__button--primary" data-action="search-customers">' + escapeHTML(state.customerBusy ? text("Searching…", "Mencari…") : text("Search", "Cari")) + '</button></div>'
          +       '</div>'
          +       '<div class="pos-terminal__scroll">' + renderCustomerResults() + '</div>'
          +     '</div>'
          +   '</section>'
          + '</div>';
      }

      function renderPromoModal() {
        if (!state.promoModalOpen) return "";
        return ''
          + '<div class="pos-terminal__overlay" data-action="close-promo-overlay">'
          +   '<section class="pos-terminal__modal" role="dialog" aria-modal="true" aria-label="' + escapeHTML(text("Promo and voucher codes", "Kode promo dan voucher")) + '">'
          +     '<div class="pos-terminal__modal-head">'
          +       '<div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Promo / voucher codes", "Kode promo / voucher")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Keep voucher redemption compact here while gift card and store credit stay in the tender rail.", "Kelola voucher di sini secara ringkas, sementara gift card dan store credit tetap di rail tender.")) + '</div></div>'
          +       '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="close-promo">' + escapeHTML(text("Close", "Tutup")) + '</button></div>'
          +     '</div>'
          +     '<div class="pos-terminal__modal-body">'
          +       '<div class="pos-terminal__field"><span>' + escapeHTML(text("Promo / voucher codes", "Kode promo / voucher")) + '</span><textarea id="pos-voucher-codes" name="pos_voucher_codes" placeholder="' + escapeHTML(text("Each line or comma-separated code", "Satu baris atau dipisah koma")) + '">' + escapeHTML(state.promotionCodes) + '</textarea></div>'
          +       '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button pos-terminal__button--soft pos-terminal__button--compact" data-action="validate-promo">' + escapeHTML(state.promotionValidationBusy ? text("Validating…", "Memvalidasi…") : text("Validate codes", "Validasi kode")) + '</button></div>'
          +       '<div class="pos-terminal__scroll">' + renderPromotionValidation() + '</div>'
          +     '</div>'
          +   '</section>'
          + '</div>';
      }

      function renderTenderModal() {
        if (!state.tenderModalOpen) return "";
        return ''
          + '<div class="pos-terminal__overlay" data-action="close-tender-overlay">'
          +   '<section class="pos-terminal__modal" role="dialog" aria-modal="true" aria-label="' + escapeHTML(text("Manage tenders", "Kelola tender")) + '">'
          +     '<div class="pos-terminal__modal-head">'
          +       '<div><h3 class="pos-terminal__panel-title">' + escapeHTML(text("Tenders", "Tender")) + '</h3><div class="pos-terminal__panel-sub">' + escapeHTML(text("Capture cash, voucher, gift card, and store credit flows without shrinking the cart.", "Kelola kas, voucher, gift card, dan store credit tanpa mengecilkan keranjang.")) + '</div></div>'
          +       '<div class="pos-terminal__buttons"><button type="button" class="pos-terminal__button" data-action="close-tender">' + escapeHTML(text("Close", "Tutup")) + '</button></div>'
          +     '</div>'
          +     '<div class="pos-terminal__modal-body">'
          +       '<div class="pos-terminal__buttons" style="justify-content:space-between"><span class="pos-terminal__summary-pill">' + escapeHTML(text("Tender lines", "Baris tender")) + ': ' + escapeHTML(String(state.tenders.length)) + '</span><button type="button" class="pos-terminal__button" data-action="add-tender">' + escapeHTML(text("Add tender", "Tambah tender")) + '</button></div>'
          +       '<div class="pos-terminal__scroll"><div class="pos-terminal__tender-list">' + state.tenders.map(renderTenderCard).join("") + '</div></div>'
          +     '</div>'
          +   '</section>'
          + '</div>';
      }

      function renderReceiptPrint() {
        const result = state.lastCheckoutResult;
        if (!result || !result.sale || !result.sale.values) return "";
        const sale = result.sale;
        const values = sale.values || {};
        const store = selectedStore();
        const register = selectedRegister();
        const lines = parseJSONArray(values.lines_json);
        const tenders = parseJSONArray(values.tenders_json);
        const promotionCodes = parseJSONArray(values.promotion_codes_json);
        const cashier = currentCashier();
        const receiptNumber = String(result.receipt_title || values.invoice_number || values.order_number || values.sale_number || sale.id || "");
        const saleNumber = String(values.sale_number || "");
        const customerName = String(values.party_name || "");
        const customerCode = String(values.party_id || "");
        const printedAt = formatDateTime(sale.created_at || new Date().toISOString());
        const currencyCode = String(values.currency_code || "IDR");
        const cashierName = String((cashier && (cashier.name || cashier.username || cashier.user_id)) || values.cashier_user_id || "—");
        const registerLabel = String((register && ((register.values || {}).name || (register.values || {}).code)) || state.registerCode || "");
        const storeLabel = String((store && ((store.values || {}).name || (store.values || {}).code)) || state.storeCode || "");
        const shiftLabel = String(values.shift_id || state.shiftId || "");
        const lookupCode = compactDocumentNumber(saleNumber || receiptNumber || sale.id || "—");
        const config = receiptConfig();
        const compactReceiptNumber = compactDocumentNumber(receiptNumber);
        const compactSaleNumber = compactDocumentNumber(saleNumber);
        const compactInvoiceNumber = compactDocumentNumber(values.invoice_number || "");
        const compactOrderNumber = compactDocumentNumber(values.order_number || "");
        const showInvoiceNumber = compactInvoiceNumber && compactInvoiceNumber !== compactReceiptNumber && compactInvoiceNumber !== compactSaleNumber;
        const showOrderNumber = compactOrderNumber && compactOrderNumber !== compactReceiptNumber && compactOrderNumber !== compactSaleNumber && compactOrderNumber !== compactInvoiceNumber;
        const totals = {
          subtotal: number(values.subtotal_amount),
          tax: number(values.tax_amount),
          total: number(values.total_amount),
          tendered: number(values.tendered_amount),
          change: number(values.change_due_amount),
        };
        const summaryPills = [
          totals.tendered >= totals.total && totals.total > 0 ? text("paid", "lunas") : "",
          lines.length ? String(lines.length) + " " + text("items", "item") : "",
          tenders.length ? String(tenders.length) + " " + text("tenders", "tender") : "",
          promotionCodes.length ? String(promotionCodes.length) + " " + text("promos", "promo") : "",
          customerName ? text("member", "member") : text("walk-in", "umum"),
        ].filter(Boolean);
        const qrURL = config.showQRCode ? '/ui/data/pos/receipt/qr?value=' + encodeURIComponent(String(saleNumber || receiptNumber || sale.id || lookupCode)) : '';
        const renderReceiptCopy = function(variant) {
          const isMerchant = variant === "merchant";
          const badgeLabel = isMerchant ? text("Merchant copy", "Salinan merchant") : text("Customer copy", "Salinan pelanggan");
          const policyTitle = isMerchant ? config.merchantNote : config.supportText;
          const policyDetail = isMerchant ? text("Use this copy for reconciliation, balancing, and audit support.", "Gunakan salinan ini untuk rekonsiliasi, balancing, dan dukungan audit.") : config.serviceText;
          const customerLabel = customerName || text("Walk-in guest", "Tamu umum");
          return ''
          +   '<section class="pos-terminal__receipt-shell pos-terminal__receipt-copy" data-receipt-variant="' + escapeHTML(variant) + '">'
          +     '<div class="pos-terminal__receipt-head">'
          +       '<div class="pos-terminal__receipt-brand">' + escapeHTML(compactLabel(config.brandName || storeLabel || text("Store receipt", "Struk toko"), 28)) + '</div>'
          +       '<div class="pos-terminal__receipt-header-grid">'
          +         '<div class="pos-terminal__receipt-sub">' + escapeHTML(String((store && store.values && store.values.code) || state.storeCode || "")) + (registerLabel ? ' · ' + escapeHTML(compactLabel(registerLabel, 24)) : '') + '</div>'
          +         '<div class="pos-terminal__receipt-sub">' + escapeHTML(text("Printed", "Dicetak")) + ': ' + escapeHTML(printedAt || "—") + '</div>'
          +       '</div>'
          +       (config.headerText ? '<div class="pos-terminal__receipt-sub">' + escapeHTML(config.headerText) + '</div>' : '')
          +       (summaryPills.length ? '<div class="pos-terminal__receipt-summaryline">' + summaryPills.map(function(item, index) { return '<span class="pos-terminal__receipt-summarypill' + (index === 0 && (totals.tendered >= totals.total && totals.total > 0) ? ' pos-terminal__receipt-summarypill--paid' : '') + '">' + escapeHTML(item) + '</span>'; }).join("") + '</div>' : '')
          +     '</div>'
          +     '<div class="pos-terminal__receipt-section">'
          +       '<div class="pos-terminal__receipt-kicker">' + escapeHTML(text("Transaction", "Transaksi")) + '</div>'
          +       '<div class="pos-terminal__receipt-meta"><span>' + escapeHTML(text("Receipt", "Struk")) + '</span><strong class="pos-terminal__receipt-code">' + escapeHTML(compactReceiptNumber || "—") + '</strong></div>'
          +       '<div class="pos-terminal__receipt-meta"><span>' + escapeHTML(text("Sale", "Penjualan")) + '</span><span class="pos-terminal__receipt-code">' + escapeHTML(compactSaleNumber || "—") + '</span></div>'
          +       (showInvoiceNumber ? '<div class="pos-terminal__receipt-meta"><span>' + escapeHTML(text("Invoice", "Invoice")) + '</span><span class="pos-terminal__receipt-code">' + escapeHTML(compactInvoiceNumber) + '</span></div>' : '')
          +       (showOrderNumber ? '<div class="pos-terminal__receipt-meta"><span>' + escapeHTML(text("Order", "Order")) + '</span><span class="pos-terminal__receipt-code">' + escapeHTML(compactOrderNumber) + '</span></div>' : '')
          +       '<div class="pos-terminal__receipt-meta"><span>' + escapeHTML(text("Cashier", "Kasir")) + '</span><span>' + escapeHTML(cashierName || "—") + '</span></div>'
          +       (shiftLabel ? '<div class="pos-terminal__receipt-meta"><span>' + escapeHTML(text("Shift", "Shift")) + '</span><span>' + escapeHTML(compactDocumentNumber(shiftLabel)) + '</span></div>' : '')
          +       '<div class="pos-terminal__receipt-meta"><span>' + escapeHTML(text("Customer", "Pelanggan")) + '</span><span>' + escapeHTML(customerLabel) + '</span></div>'
          +       (customerCode ? '<div class="pos-terminal__receipt-meta"><span>' + escapeHTML(text("Member ID", "ID member")) + '</span><span class="pos-terminal__receipt-code">' + escapeHTML(compactDocumentNumber(customerCode)) + '</span></div>' : '')
          +     '</div>'
          +     '<div class="pos-terminal__receipt-lookup"><div class="pos-terminal__receipt-kicker">' + escapeHTML(text("Lookup", "Pencarian")) + '</div><div class="pos-terminal__receipt-lookup-visuals"><div>' + buildReceiptLookupBars(lookupCode) + '</div>' + (qrURL ? '<img class="pos-terminal__receipt-lookup-qr" src="' + escapeHTML(qrURL) + '" alt="' + escapeHTML(text("Receipt lookup QR", "QR lookup struk")) + '">' : buildReceiptLookupMatrix(lookupCode)) + '</div><div class="pos-terminal__receipt-lookup-code">' + escapeHTML(lookupCode) + '</div></div>'
          +     '<div class="pos-terminal__receipt-section">'
          +       '<div class="pos-terminal__receipt-kicker">' + escapeHTML(text("Items", "Item")) + '</div>'
          +       '<div class="pos-terminal__receipt-items">' + (lines.length ? lines.map(function(line) {
                    const qty = number(line.quantity);
                    const unitPrice = number(line.unit_price);
                    const lineTotal = number(line.line_total || line.extended_amount || line.amount);
                    const discount = number(line.discount_amount);
                    const note = String(line.note || "").trim();
                    return ''
                      + '<div class="pos-terminal__receipt-item">'
                      +   '<div class="pos-terminal__receipt-item-head"><span class="pos-terminal__receipt-item-name">' + escapeHTML(String(line.description || line.item_name || line.item_code || "Item")) + '</span><span class="pos-terminal__receipt-item-price">' + escapeHTML(money(lineTotal)) + '</span></div>'
                      +   '<div class="pos-terminal__receipt-item-sub"><span>' + escapeHTML(String(qty)) + ' × ' + escapeHTML(money(unitPrice)) + '</span>' + (discount > 0 ? '<span>' + escapeHTML(text("Disc", "Diskon")) + ' ' + escapeHTML(money(discount)) + '</span>' : '<span>' + escapeHTML(currencyCode) + '</span>') + '</div>'
                      +   (note ? '<div class="pos-terminal__receipt-item-note">' + escapeHTML(note) + '</div>' : '')
                      + '</div>';
                  }).join("") : '<div class="pos-terminal__receipt-sub">' + escapeHTML(text("No line items.", "Tidak ada baris item.")) + '</div>') + '</div>'
          +     '</div>'
          +     '<div class="pos-terminal__receipt-section">'
          +       '<div class="pos-terminal__receipt-kicker">' + escapeHTML(text("Totals", "Total")) + '</div>'
          +       '<div class="pos-terminal__receipt-totals">'
          +         '<div class="pos-terminal__receipt-total-row"><span>' + escapeHTML(text("Subtotal", "Subtotal")) + '</span><span>' + escapeHTML(money(totals.subtotal)) + '</span></div>'
          +         (totals.tax > 0 ? '<div class="pos-terminal__receipt-total-row"><span>' + escapeHTML(text("Tax", "Pajak")) + '</span><span>' + escapeHTML(money(totals.tax)) + '</span></div>' : '')
          +         '<div class="pos-terminal__receipt-total-row pos-terminal__receipt-total-row--grand"><span>' + escapeHTML(text("Total", "Total")) + '</span><span>' + escapeHTML(money(totals.total)) + '</span></div>'
          +         '<div class="pos-terminal__receipt-total-row"><span>' + escapeHTML(text("Tendered", "Dibayar")) + '</span><span>' + escapeHTML(money(totals.tendered)) + '</span></div>'
          +         '<div class="pos-terminal__receipt-total-row pos-terminal__receipt-total-row--change"><span>' + escapeHTML(text("Change", "Kembalian")) + '</span><span>' + escapeHTML(money(totals.change)) + '</span></div>'
          +         (totals.total > 0 ? '<div class="pos-terminal__receipt-settlement">' + escapeHTML(totals.tendered >= totals.total ? text("Paid in full", "Lunas") : text("Payment pending", "Pembayaran belum lengkap")) + '</div>' : '')
          +       '</div>'
          +     '</div>'
          +     (tenders.length ? '<div class="pos-terminal__receipt-section"><div class="pos-terminal__receipt-kicker">' + escapeHTML(text("Payment", "Pembayaran")) + '</div>' + tenders.map(function(tender) {
                const tenderCode = String(tender.tender_type_code || tender.kind || "");
                const tenderLabel = (function() {
                  const tenderType = tenderTypeByCode(tenderCode);
                  const named = String((tenderType && tenderType.values && tenderType.values.name) || "");
                  return named || humanizeCode(tenderCode) || text("Tender", "Tender");
                })();
                const reference = String(tender.reference || "").trim();
                return '<div class="pos-terminal__receipt-payment"><div class="pos-terminal__receipt-meta"><span>' + escapeHTML(tenderLabel) + '</span><span>' + escapeHTML(money(tender.amount)) + '</span></div>' + (reference ? '<div class="pos-terminal__receipt-payment-ref">' + escapeHTML(reference) + '</div>' : '') + '</div>';
              }).join("") + '</div>' : '')
          +     (promotionCodes.length ? '<div class="pos-terminal__receipt-section"><div class="pos-terminal__receipt-kicker">' + escapeHTML(text("Applied promos", "Promo aktif")) + '</div><div class="pos-terminal__receipt-sub"><span class="pos-terminal__receipt-accent">' + escapeHTML(promotionCodes.join(", ")) + '</span></div></div>' : '')
          +     '<div class="pos-terminal__receipt-foot">'
          +       '<span class="pos-terminal__receipt-badge">' + escapeHTML(badgeLabel) + '</span>'
          +       '<div>' + escapeHTML(config.footerText) + '</div>'
          +       '<div class="pos-terminal__receipt-policy"><div>' + escapeHTML(policyTitle) + '</div><div>' + escapeHTML(policyDetail) + '</div></div>'
          +       '<div class="pos-terminal__receipt-divider"></div>'
          +       '<div>' + escapeHTML(text("Operational copy only. Taxes and payments are recorded in Orbyte.", "Salinan operasional. Pajak dan pembayaran tercatat di Orbyte.")) + '</div>'
          +       '<div>' + escapeHTML(text("Register", "Register")) + ': ' + escapeHTML(compactLabel(registerLabel || state.registerCode || "—", 28)) + '</div>'
          +       '<div>' + escapeHTML(text("Served by", "Dilayani oleh")) + ': ' + escapeHTML(compactLabel(cashierName || "—", 28)) + '</div>'
          +       '<div class="pos-terminal__receipt-cutline">' + escapeHTML(badgeLabel) + '</div>'
          +     '</div>'
          +   '</section>';
        };
        return ''
          + '<aside class="pos-terminal__receipt-print" aria-hidden="true">'
          +   config.variants.map(renderReceiptCopy).join("")
          + '</aside>';
      }

      function bindEvents() {
        mount.querySelectorAll("[data-nav]").forEach(function(node) {
          node.addEventListener("click", function() {
            window.location.href = String(node.getAttribute("data-nav") || "/ui/backoffice");
          });
        });
        mount.querySelector("#pos-store")?.addEventListener("change", function(event) {
          state.storeCode = String(event.target.value || "");
          state.registerCode = "";
          state.shiftId = "";
          state.searchResults = [];
          resetSaleState();
          state.promotionValidation = null;
          persist();
          loadBootstrap().then(function() { return loadHeldSales(); }).then(render).catch(function(error) {
            notify(error instanceof Error ? error.message : "Failed to load store context", "error");
          });
        });
        mount.querySelector("#pos-register")?.addEventListener("change", function(event) {
          state.registerCode = String(event.target.value || "");
          state.shiftId = "";
          resetSaleState();
          persist();
          loadBootstrap().then(function() { return loadHeldSales(); }).then(render).catch(function(error) {
            notify(error instanceof Error ? error.message : "Failed to load register context", "error");
          });
        });
        mount.querySelector("#pos-terminal-pin")?.addEventListener("input", function(event) {
          state.terminalPIN = String(event.target.value || "");
        });
        mount.querySelector("#pos-opening-cash")?.addEventListener("input", function(event) {
          state.terminalOpeningCash = String(event.target.value || "0");
        });
        mount.querySelector("#pos-terminal-notes")?.addEventListener("input", function(event) {
          state.terminalNotes = String(event.target.value || "");
        });
        mount.querySelector("#pos-terminal-pin")?.addEventListener("keydown", function(event) {
          if (event.key === "Enter" || event.code === "NumpadEnter") {
            event.preventDefault();
            enterTerminal(state.bootstrap && state.bootstrap.open_shift ? "resume" : "open");
          }
        });
        mount.querySelector("#pos-search")?.addEventListener("input", function(event) {
          state.searchQuery = String(event.target.value || "");
        });
        mount.querySelector("[data-form='catalog-search']")?.addEventListener("submit", function(event) {
          event.preventDefault();
          searchCatalog().catch(function(error) {
            notify(error instanceof Error ? error.message : "Search failed", "error");
          });
        });
        mount.querySelector("#pos-search")?.addEventListener("keydown", function(event) {
          if (event.key === "Enter" || event.code === "NumpadEnter") {
            event.preventDefault();
            searchCatalog().catch(function(error) {
              notify(error instanceof Error ? error.message : "Search failed", "error");
            });
          }
        });
        mount.querySelector("#pos-customer-search")?.addEventListener("input", function(event) {
          state.customerQuery = String(event.target.value || "");
        });
        mount.querySelector("#pos-customer-search")?.addEventListener("keydown", function(event) {
          if (event.key === "Enter") {
            event.preventDefault();
            searchCustomers().catch(function(error) {
              notify(error instanceof Error ? error.message : "Customer lookup failed", "error");
            });
          }
        });
        mount.querySelector("#pos-lookup")?.addEventListener("input", function(event) {
          state.lookupQuery = String(event.target.value || "");
        });
        mount.querySelector("#pos-voucher-codes")?.addEventListener("input", function(event) {
          state.promotionCodes = String(event.target.value || "");
          state.promotionValidation = null;
          persist();
        });
        mount.querySelectorAll("[data-attach-customer]").forEach(function(node) {
          node.addEventListener("click", function() {
            const item = state.customerResults[number(node.getAttribute("data-attach-customer"))];
            if (item) attachCustomer(item);
          });
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
              transactAction(String(record.id || ""), "refund").catch(function(error) {
                notify(error instanceof Error ? error.message : "Refund failed", "error");
              });
            }
          });
        });
        mount.querySelectorAll("[data-refund-store-credit]").forEach(function(node) {
          node.addEventListener("click", function() {
            const record = state.transactions[number(node.getAttribute("data-refund-store-credit"))];
            if (record && window.confirm(text("Convert this refund to store credit?", "Ubah refund ini menjadi store credit?"))) {
              transactAction(String(record.id || ""), "refund-store-credit").catch(function(error) {
                notify(error instanceof Error ? error.message : "Store credit refund failed", "error");
              });
            }
          });
        });
        mount.querySelectorAll("[data-exchange-sale]").forEach(function(node) {
          node.addEventListener("click", function() {
            const record = state.transactions[number(node.getAttribute("data-exchange-sale"))];
            if (record && window.confirm(text("Create exchange documents for this sale?", "Buat dokumen tukar untuk penjualan ini?"))) {
              transactAction(String(record.id || ""), "exchange").catch(function(error) {
                notify(error instanceof Error ? error.message : "Exchange failed", "error");
              });
            }
          });
        });
        mount.querySelectorAll("[data-line-qty]").forEach(function(node) {
          node.addEventListener("input", function() {
            const index = number(node.getAttribute("data-line-qty"));
            state.cartQuantityDrafts[String(index)] = String(node.value || "");
          });
          node.addEventListener("blur", function() {
            const index = number(node.getAttribute("data-line-qty"));
            commitLineQuantity(index, node.value);
          });
          node.addEventListener("keydown", function(event) {
            if (event.key === "Enter" || event.code === "NumpadEnter") {
              event.preventDefault();
              const index = number(node.getAttribute("data-line-qty"));
              commitLineQuantity(index, node.value);
            }
          });
        });
        mount.querySelectorAll("[data-remove-line]").forEach(function(node) {
          node.addEventListener("click", function() {
            state.cart.splice(number(node.getAttribute("data-remove-line")), 1);
            state.promotionValidation = null;
            persist();
            render();
          });
        });
        mount.querySelectorAll("[data-tender-type]").forEach(function(node) {
          node.addEventListener("change", function() {
            const line = state.tenders[number(node.getAttribute("data-tender-type"))];
            if (!line) return;
            line.tender_type_code = String(node.value || "");
            if (tenderRequiresReference(line.tender_type_code) && !line.reference) {
              line.reference = "";
            }
            if (tenderRequiresParty(line.tender_type_code) && (!state.customer || !state.customer.party_id)) {
              notify(text("Attach a customer for this tender type.", "Pasang pelanggan untuk jenis tender ini."), "error");
            }
            state.tenderInsights[String(node.getAttribute("data-tender-type"))] = null;
            persist();
            maybeRefreshStoreCreditInsights();
            render();
          });
        });
        mount.querySelectorAll("[data-tender-amount]").forEach(function(node) {
          node.addEventListener("input", function() {
            const index = number(node.getAttribute("data-tender-amount"));
            state.tenderAmountDrafts[String(index)] = String(node.value || "");
          });
          node.addEventListener("blur", function() {
            const index = number(node.getAttribute("data-tender-amount"));
            commitTenderAmount(index, node.value);
          });
          node.addEventListener("keydown", function(event) {
            if (event.key === "Enter" || event.code === "NumpadEnter") {
              event.preventDefault();
              const index = number(node.getAttribute("data-tender-amount"));
              commitTenderAmount(index, node.value);
            }
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
        mount.querySelectorAll("[data-check-tender]").forEach(function(node) {
          node.addEventListener("click", function() {
            lookupStoredValue(number(node.getAttribute("data-check-tender"))).catch(function(error) {
              notify(error instanceof Error ? error.message : "Lookup failed", "error");
            });
          });
        });
        mount.querySelectorAll("[data-remove-tender]").forEach(function(node) {
          node.addEventListener("click", function() {
            state.tenders.splice(number(node.getAttribute("data-remove-tender")), 1);
            if (!state.tenders.length) state.tenders = [{ tender_type_code: "", amount: 0, reference: "", notes: "" }];
            state.tenderInsights = {};
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
            if (action === "open-catalog") {
              openCatalog();
              return;
            }
            if (action === "close-catalog") {
              closeCatalog();
              return;
            }
            if (action === "open-customer") {
              openCustomerModal();
              return;
            }
            if (action === "close-customer") {
              closeCustomerModal();
              return;
            }
            if (action === "search-customers") {
              searchCustomers().catch(function(error) { notify(error instanceof Error ? error.message : "Customer lookup failed", "error"); });
              return;
            }
            if (action === "open-promo") {
              openPromoModal();
              return;
            }
            if (action === "open-tender") {
              openTenderModal();
              return;
            }
            if (action === "validate-promo") {
              validatePromotions();
              return;
            }
            if (action === "close-promo") {
              closePromoModal();
              return;
            }
            if (action === "close-tender") {
              closeTenderModal();
              return;
            }
            if (action === "open-shift") {
              openShift().catch(function(error) { notify(error instanceof Error ? error.message : "Open shift failed", "error"); });
              return;
            }
            if (action === "resume-terminal") {
              enterTerminal("resume").catch(function(error) { notify(error instanceof Error ? error.message : "Resume shift failed", "error"); });
              return;
            }
            if (action === "close-shift") {
              closeShift().catch(function(error) { notify(error instanceof Error ? error.message : "Close shift failed", "error"); });
              return;
            }
            if (action === "lock-terminal") {
              lockTerminal().catch(function(error) { notify(error instanceof Error ? error.message : "Lock failed", "error"); });
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
            if (action === "refresh-terminal") {
              loadBootstrap().then(function() { return loadHeldSales(); }).then(render).catch(function(error) { notify(error instanceof Error ? error.message : "Refresh failed", "error"); });
              return;
            }
            if (action === "lookup-transactions") {
              lookupTransactions().catch(function(error) { notify(error instanceof Error ? error.message : "Lookup failed", "error"); });
              return;
            }
            if (action === "toggle-held") {
              state.heldExpanded = !state.heldExpanded;
              render();
              return;
            }
            if (action === "toggle-control-details") {
              state.controlCollapsed = !state.controlCollapsed;
              render();
              return;
            }
            if (action === "toggle-transactions") {
              state.transactionsExpanded = !state.transactionsExpanded;
              render();
              return;
            }
            if (action === "add-tender") {
              state.tenders.push({ tender_type_code: "", amount: 0, reference: "", notes: "" });
              persist();
              render();
              return;
            }
            if (action === "clear-customer") {
              attachCustomer(null);
            }
          });
        });
        mount.querySelectorAll("[data-action='close-catalog-overlay']").forEach(function(node) {
          node.addEventListener("click", function(event) {
            if (event.target === node) closeCatalog();
          });
        });
        mount.querySelectorAll("[data-action='close-customer-overlay']").forEach(function(node) {
          node.addEventListener("click", function(event) {
            if (event.target === node) closeCustomerModal();
          });
        });
        mount.querySelectorAll("[data-action='close-promo-overlay']").forEach(function(node) {
          node.addEventListener("click", function(event) {
            if (event.target === node) closePromoModal();
          });
        });
        mount.querySelectorAll("[data-action='close-tender-overlay']").forEach(function(node) {
          node.addEventListener("click", function(event) {
            if (event.target === node) closeTenderModal();
          });
        });
      }

      function render() {
        ensureStyles();
        const bootstrap = state.bootstrap || { stores: [], registers: [], tender_types: [] };
        const totalsPayload = totals();
        if (state.loading) {
          mount.innerHTML = '<section class="pos-terminal"><section class="pos-terminal__setup"><div class="pos-terminal__stack"><strong>' + escapeHTML(text("Loading terminal…", "Memuat terminal…")) + '</strong><div>' + escapeHTML(text("Loading store context, registers, and active tender methods.", "Memuat konteks toko, register, dan metode tender aktif.")) + '</div></div></section></section>';
          return;
        }

        mount.innerHTML = ''
          + '<section class="pos-terminal">'
          +   (state.message ? '<div class="pos-terminal__notice">' + escapeHTML(state.message) + '</div>' : '')
          +   (!(bootstrap.stores || []).length || !(bootstrap.registers || []).length || !(bootstrap.tender_types || []).length
                ? renderSetupState(bootstrap)
                : (terminalUnlocked()
                    ? renderControlBar() + '<section class="pos-terminal__workspace"><div class="pos-terminal__left">' + renderCartPanel() + renderAuxPanel() + '</div>' + renderRail(totalsPayload) + '</section>' + renderCatalogModal() + renderCustomerModal() + renderPromoModal() + renderTenderModal()
                    : renderTerminalGate(bootstrap)))
          +   renderReceiptPrint()
          + '</section>';
        bindEvents();
      }

      restore();
      render();
      try {
        await loadBootstrap();
        await loadHeldSales();
        state.loading = false;
        render();
      } catch (error) {
        state.loading = false;
        state.message = error instanceof Error ? error.message : text("Failed to load terminal.", "Gagal memuat terminal.");
        render();
      }
    }
  };
})();`
}
