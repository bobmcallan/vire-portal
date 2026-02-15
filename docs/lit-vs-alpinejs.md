# Lit vs Alpine.js — Framework Evaluation for vire-portal

**Date:** 2026-02-15

## Current State

Alpine.js 3.x is loaded from CDN in `pages/partials/head.html` but has **zero active usage**. No Alpine directives (`x-data`, `x-bind`, `x-on`, etc.) exist in any template. `pages/static/common.js` contains only a placeholder `alpine:init` listener. All interactivity is server-side Go templates with CSS hover states.

The frontend is 2 pages (`landing.html`, `dashboard.html`), 3 partials (`head`, `nav`, `footer`), 1 CSS file (325 lines, 80s terminal aesthetic), and 9 lines of JS.

## Framework Profiles

| Aspect | Alpine.js | Lit |
|--------|-----------|-----|
| Size (gzipped) | ~15 KB | ~5 KB |
| Philosophy | HTML-first directives | Component-based web standards |
| Build step | None | Optional (works without) |
| Rendering | Enhances existing HTML | Owns component render pipeline |
| Standards | Custom (Vue-inspired reactivity) | Native Web Components (Custom Elements, Shadow DOM) |
| Template language | HTML attributes (`x-data`, `x-bind`) | JavaScript tagged templates (`` html`...` ``) |
| Backed by | Community (Caleb Porzio) | Google |

## Evaluation Criteria

### 1. Fit with Go server-rendered templates

**Alpine.js: Strong fit.** Designed explicitly for server-rendered pages. Directives go directly in the HTML that Go templates produce. No template duplication — Go renders the structure, Alpine adds interactivity inline. 95% of Alpine users work with server-rendered pages.

**Lit: Poor fit.** Lit wants to own the render pipeline via its `render()` method and `html` tagged templates in JS. Using Lit with Go templates means either duplicating templates across Go and JS, or limiting Lit to isolated widgets. Moving a Go template partial into a Lit component translates it from HTML to JS — moving from "JS sprinkles" to "JS chunks."

### 2. Complexity of adoption

**Alpine.js: Minimal.** Add directives to existing HTML. No files to create, no build step, no component class hierarchy. A toggle becomes `<div x-data="{ open: false }">`. Existing Go template structure stays intact.

**Lit: Moderate.** Each component is a JS class extending `LitElement` with lifecycle methods, reactive properties, and a `render()` function returning `html` tagged templates. Requires defining custom elements, understanding Shadow DOM (or opting out), and managing a component registry. Even without a build step, the authoring model is heavier.

### 3. Future interactivity needs

Planned features: tool filtering, config editing, real-time updates, user settings.

**Alpine.js:** Handles these with `x-data` stores, `x-model` for forms, `$watch` for reactivity, and `fetch()` for API calls. For a dashboard with ~5-10 interactive areas, Alpine keeps things simple and contained.

**Lit:** Better suited if components need to be shared across multiple apps or published as a design system. Lit components are framework-agnostic and work anywhere HTML works. If vire-portal components were reused in vire-mcp or other projects, Lit's portability is an advantage.

### 4. Performance

**Alpine.js:** ~15 KB gzipped. Initializes by scanning the DOM for directives. Performance is fine for dozens of interactive elements. Can become slow with hundreds of reactive bindings on a single page (unlikely for this app).

**Lit:** ~5 KB gzipped. Updates only dynamic parts of the DOM without virtual DOM diffing. More efficient for component-heavy UIs. Better memory profile under stress. For this app's scale, the difference is negligible.

### 5. Long-term durability

**Alpine.js:** Community-driven, stable at v3 since 2021. Not a web standard — if the project is abandoned, directives stop working when the library is removed. API has been stable but is framework-specific syntax.

**Lit:** Built on Web Components standards (Custom Elements, Shadow DOM, HTML Templates). Even if Lit itself is abandoned, the underlying web standards remain. Components work in any browser natively. Google maintains it actively. More future-proof by design.

### 6. Developer experience

**Alpine.js:** Low learning curve. Familiar to anyone who's used Vue. Write HTML, add attributes, done. Debugging is straightforward — inspect the DOM. No tooling required.

**Lit:** Steeper learning curve. Requires understanding Web Components, Shadow DOM encapsulation, reactive properties, and the `html` template literal syntax. TypeScript recommended (Lit docs favor it). Better IDE support with lit-analyzer plugin.

### 7. CSS and styling

**Alpine.js:** Uses the existing global CSS. No encapsulation concerns. The current `portal.css` works as-is with Alpine directives.

**Lit:** Shadow DOM encapsulates styles by default — global CSS doesn't penetrate into components. The existing `portal.css` won't apply inside Lit components unless you: (a) use `::part()` selectors, (b) adopt constructable stylesheets, or (c) disable Shadow DOM. This is significant friction for the current styling approach.

## Pros and Cons Summary

### Staying with Alpine.js

| Pros | Cons |
|------|------|
| Zero migration cost (already loaded) | Not a web standard — framework-specific |
| Perfect fit for Go server templates | Larger bundle than Lit (~15 KB vs ~5 KB) |
| No build step required | Components not portable across frameworks |
| Existing CSS works unchanged | Scaling limits with very complex UIs |
| Minimal learning curve | Single maintainer risk |
| HTML stays readable in Go templates | |

### Switching to Lit

| Pros | Cons |
|------|------|
| Built on web standards (future-proof) | Poor fit with Go server-rendered templates |
| Smaller bundle (~5 KB) | Requires template duplication (Go + JS) |
| Framework-agnostic components | Shadow DOM breaks existing global CSS |
| Better for shared component libraries | Steeper learning curve |
| Google-backed, active ecosystem | Heavier authoring model (classes, lifecycle) |
| Superior performance at scale | Overkill for current complexity level |

## Recommendation

**Stay with Alpine.js.** The rationale:

1. **Architecture match.** vire-portal is a Go server-rendered app with progressive enhancement needs. Alpine is designed for exactly this. Lit is designed for component-owned rendering, which conflicts with Go templates.

2. **Zero migration cost.** Alpine is already loaded. Adding interactivity means adding HTML attributes to existing templates. Lit would require creating JS component files, duplicating template logic, and solving Shadow DOM CSS isolation.

3. **Right-sized.** The app has 2 pages and ~5-10 future interactive areas. Alpine handles this comfortably. Lit's component architecture is engineered for larger-scale component libraries and design systems — more infrastructure than needed here.

4. **CSS compatibility.** The 80s terminal aesthetic in `portal.css` relies on global styles. Shadow DOM encapsulation would break this without workarounds.

**When Lit would make sense:** If vire-portal evolves into a component library shared across multiple applications, or if the frontend grows to 20+ complex interactive components with their own state management needs. At that point, the investment in Lit's component architecture would pay off.

## Sources

- [Alpine.js vs Lit — Lightweight Framework Comparison](https://unitysangam.com/tech/alpine-js-vs-lit/)
- [Progressive Enhancement Options for Server-Rendered Sites — Jay Freestone](https://www.jayfreestone.com/writing/web-component-libraries/)
- [Alpine.js as a Stimulus Alternative — Felipe Vogel](https://fpsvogel.com/posts/2024/alpine-js-vs-stimulus)
- [Lit.js: Building Fast, Lightweight Web Components — Perficient](https://blogs.perficient.com/2025/05/05/lit-js-building-fast-lightweight-and-scalable-web-components/)
- [Web Components 2025: Lit 3.0 vs Stencil 4.0 — Markaicode](https://markaicode.com/web-components-2025-lit-stencil-enterprise/)
- [Lit Documentation](https://lit.dev/docs/)
