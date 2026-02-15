// VIRE Portal - Alpine.js components
// Load BEFORE Alpine.js in head.html

document.addEventListener('alpine:init', () => {

    // ── Dropdown ────────────────────────────────────────
    // <div x-data="dropdown()" class="dropdown">
    //   <button @click="toggle()" class="dropdown-trigger">
    //     Menu <span class="dropdown-arrow" :class="open && 'open'">▼</span>
    //   </button>
    //   <div x-show="open" @click.outside="close()" x-transition.opacity class="dropdown-menu">
    //     <a href="#">Item</a>
    //   </div>
    // </div>
    Alpine.data('dropdown', () => ({
        open: false,
        toggle() { this.open = !this.open; },
        close()  { this.open = false; },
    }));

    // ── Mobile Menu ─────────────────────────────────────
    // Place x-data="mobileMenu()" on a parent (e.g. the nav or page wrapper)
    // <button @click="open()" class="nav-hamburger">...</button>
    // <template x-if="visible">
    //   <div>
    //     <div class="mobile-overlay" @click="close()"></div>
    //     <div class="mobile-menu">
    //       <button @click="close()" class="mobile-menu-close">✕</button>
    //       <a href="/">Dashboard</a>
    //     </div>
    //   </div>
    // </template>
    Alpine.data('mobileMenu', () => ({
        visible: false,
        open()  { this.visible = true; },
        close() { this.visible = false; },
    }));

    // ── Tabs ────────────────────────────────────────────
    // <div x-data="tabs('overview')">
    //   <div class="tabs">
    //     <button class="tab" :class="isActive('overview')" @click="select('overview')">Overview</button>
    //     <button class="tab" :class="isActive('config')"   @click="select('config')">Config</button>
    //   </div>
    //   <div x-show="current === 'overview'">...</div>
    //   <div x-show="current === 'config'">...</div>
    // </div>
    Alpine.data('tabs', (initial) => ({
        current: initial || '',
        select(t)   { this.current = t; },
        isActive(t) { return this.current === t ? 'active' : ''; },
    }));

    // ── Collapsible Panel ───────────────────────────────
    // <div x-data="collapse()" class="panel-collapse">
    //   <button @click="toggle()" class="panel-collapse-trigger">
    //     Title <span :style="arrow()">▼</span>
    //   </button>
    //   <div x-show="open" x-transition class="panel-collapse-body">
    //     ...content...
    //   </div>
    // </div>
    Alpine.data('collapse', (startOpen) => ({
        open: startOpen || false,
        toggle() { this.open = !this.open; },
        arrow()  { return this.open ? 'transform:rotate(180deg);transition:transform 0.15s' : 'transition:transform 0.15s'; },
    }));

    // ── Toast Notifications ─────────────────────────────
    // Place ONCE in your layout (e.g. footer.html or page wrapper):
    //   <div x-data="toasts()" @toast.window="add($event.detail)" class="toast-container">
    //     <template x-for="t in list" :key="t.id">
    //       <div class="toast" :class="t.dark && 'toast-dark'" x-text="t.msg"></div>
    //     </template>
    //   </div>
    //
    // Dispatch from anywhere:
    //   $dispatch('toast', { msg: 'Saved!' })
    //   $dispatch('toast', { msg: 'Error', dark: true })
    Alpine.data('toasts', () => ({
        list: [],
        add(detail) {
            const t = {
                id: Date.now() + Math.random(),
                msg: detail.msg || '',
                dark: detail.dark || false,
            };
            this.list.push(t);
            setTimeout(() => {
                this.list = this.list.filter(x => x.id !== t.id);
            }, detail.duration || 3000);
        },
    }));

    // ── Confirm Action ──────────────────────────────────
    // <button x-data="confirm('Delete?')" @click="ask(() => { /* action */ })">Delete</button>
    Alpine.data('confirm', (message) => ({
        ask(action) {
            if (window.confirm(message)) action();
        },
    }));

});
