# Admin Prompt Template Management Pages

**Feedback:** fb_3d4d763f
**Status:** implementing

## Scope

Add two admin pages for managing AI prompt templates:
1. **Prompts List** (`/admin/prompts`) — table of all prompt templates
2. **Prompt Edit** (`/admin/prompts/{name}`) — view/edit single prompt with save

**NOT in scope:** creating new prompts, deleting prompts, prompt versioning/history.

## Data Shape (from live MCP)

```json
{
  "name": "filing_summary",
  "description": "Filing summary extraction prompt for Gemini",
  "content": "Each object: {...}",
  "hash": "d7b170c8...",
  "is_override": false,
  "updated_at": "0001-01-01T00:00:00Z"
}
```

API: `admin_list_prompts` returns array, `admin_get_prompt` returns single object, `admin_set_prompt` takes name + content.

## File Changes

### 1. `internal/client/vire_client.go` — Add AdminPrompt type and 3 methods

**New type:**
```go
type AdminPrompt struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Content     string `json:"content"`
    Hash        string `json:"hash"`
    IsOverride  bool   `json:"is_override"`
    UpdatedAt   string `json:"updated_at"`
}
```

**New methods** (follow AdminListUsers pattern):
```go
func (c *VireClient) AdminListPrompts(serviceID string) ([]AdminPrompt, error)
// GET /api/admin/prompts with X-Vire-Service-ID header

func (c *VireClient) AdminGetPrompt(serviceID, name string) (*AdminPrompt, error)
// GET /api/admin/prompts/{name} with X-Vire-Service-ID header

func (c *VireClient) AdminSetPrompt(serviceID, name, content string) error
// PUT /api/admin/prompts/{name} with X-Vire-Service-ID header, body: {"content":"..."}
```

### 2. `internal/handlers/prompts.go` — New handler (CREATE)

**Struct:**
```go
type AdminPromptsHandler struct {
    logger             *common.Logger
    templates          *template.Template
    devMode            bool
    jwtSecret          []byte
    userLookupFn       func(string) (*client.UserProfile, error)
    adminListPromptsFn func(string) ([]client.AdminPrompt, error)
    adminGetPromptFn   func(string, string) (*client.AdminPrompt, error)
    adminSetPromptFn   func(string, string, string) error
    serviceUserID      string
    apiURL             string
}
```

**Constructor:** `NewAdminPromptsHandler(logger, devMode, jwtSecret, userLookupFn, listFn, getFn, setFn, serviceUserID) *AdminPromptsHandler`

**Methods:**
- `SetAPIURL(apiURL string)`
- `ServeHTTP(w, r)` — dispatcher: GET without name → list, GET with name → edit, POST with name → save
- `serveList(w, r, loggedIn bool, userRole string)` — fetch prompts, render `prompts.html`
- `serveEdit(w, r, loggedIn bool, userRole, name string)` — fetch prompt, JSON-encode for hydration, render `prompt-edit.html`
- `handleSave(w, r, userRole, name string)` — parse JSON body, call setFn, return JSON response

**Auth pattern** (same as users.go):
```go
loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
if !loggedIn { redirect to "/" }
var userRole string
if claims != nil && claims.Sub != "" && h.userLookupFn != nil {
    user, _ := h.userLookupFn(claims.Sub)
    if user != nil { userRole = user.Role }
}
if userRole != "admin" { redirect to "/dashboard" }
```

**Template data (list):**
```go
data := map[string]interface{}{
    "Page":          "prompts",
    "DevMode":       h.devMode,
    "LoggedIn":      loggedIn,
    "UserRole":      userRole,
    "Prompts":       prompts,
    "PromptCount":   len(prompts),
    "FetchError":    fetchErr,
    "PortalVersion": config.GetVersion(),
    "ServerVersion": GetServerVersion(h.apiURL),
}
```

**Template data (edit):**
```go
promptJSON, _ := json.Marshal(prompt) // for Alpine hydration
data := map[string]interface{}{
    "Page":          "prompts",
    "DevMode":       h.devMode,
    "LoggedIn":      loggedIn,
    "UserRole":      userRole,
    "Prompt":        prompt,       // for Go template rendering
    "PromptJSON":    template.JS(promptJSON), // for window.__VIRE_DATA__
    "FetchError":    fetchErr,
    "PortalVersion": config.GetVersion(),
    "ServerVersion": GetServerVersion(h.apiURL),
}
```

**Save handler response:**
```go
w.Header().Set("Content-Type", "application/json")
// success: {"status":"ok"}
// error: {"error":"message"} with appropriate status code
```

### 3. `pages/prompts.html` — New template (CREATE)

Follow `users.html` pattern exactly. Panel header: `PROMPTS [{{.PromptCount}}]`. Table columns: Name (linked to edit), Description, Override (YES/-), Updated.

The Name column links to `/admin/prompts/{{.Name}}`.

### 4. `pages/prompt-edit.html` — New template (CREATE)

Structure:
- Back link: `<a href="/admin/prompts">← Back to Prompts</a>`
- Panel header: `PROMPT: {{.Prompt.Name}}`
- Meta section: Description (read-only), Override status, Hash
- Textarea: `x-model="content"`, class `prompt-textarea`, rows="20", spellcheck="false"
- Actions: SAVE button with saving state, success/error message span
- Script block: `window.__VIRE_DATA__ = { prompt: {{.PromptJSON}} };`
- Alpine component: `x-data="promptEditor()"`

### 5. `pages/static/common.js` — Add promptEditor() function

```javascript
function promptEditor() {
    return {
        content: '',
        saving: false,
        message: '',
        success: false,
        promptName: '',
        init() {
            const data = window.__VIRE_DATA__;
            if (data && data.prompt) {
                this.content = data.prompt.content || '';
                this.promptName = data.prompt.name || '';
            }
        },
        async save() {
            this.saving = true;
            this.message = '';
            try {
                const res = await fetch('/admin/prompts/' + encodeURIComponent(this.promptName), {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ content: this.content }),
                });
                if (res.ok) {
                    this.success = true;
                    this.message = 'Saved successfully';
                } else {
                    const data = await res.json().catch(() => ({}));
                    this.success = false;
                    this.message = data.error || 'Save failed';
                }
            } catch (e) {
                this.success = false;
                this.message = 'Network error';
            } finally {
                this.saving = false;
                setTimeout(() => { this.message = ''; }, 5000);
            }
        }
    };
}
```

### 6. `pages/static/css/portal.css` — Add prompt styles

```css
/* Prompt edit page */
.prompt-textarea {
    width: 100%;
    min-height: 400px;
    font-family: 'IBM Plex Mono', ui-monospace, monospace;
    font-size: 0.85rem;
    line-height: 1.5;
    padding: 1rem;
    border: 1px solid #ddd;
    background: #fafafa;
    color: #333;
    resize: vertical;
    margin: 1rem 0;
}

.prompt-meta {
    font-size: 0.85rem;
    color: #666;
    margin-bottom: 0.5rem;
}

.prompt-meta div {
    margin-bottom: 0.25rem;
}

.prompt-actions {
    display: flex;
    align-items: center;
    gap: 1rem;
}

.btn-save {
    padding: 0.5rem 2rem;
    font-family: 'IBM Plex Mono', ui-monospace, monospace;
    font-weight: 700;
    font-size: 0.8rem;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    border: 1px solid #888;
    background: none;
    color: #333;
    cursor: pointer;
}

.btn-save:hover { background: #eee; }
.btn-save:disabled { opacity: 0.5; cursor: not-allowed; }

.prompt-success { color: #2e7d32; }
.prompt-error { color: #c62828; }
```

### 7. `pages/partials/nav.html` — Update admin nav links

Change line 26 from:
```html
{{if eq .UserRole "admin"}}<a href="/admin/users">Admin</a>{{end}}
```
To:
```html
{{if eq .UserRole "admin"}}<a href="/admin/users">Users</a><a href="/admin/prompts">Prompts</a>{{end}}
```

Same change on line 48 for mobile menu.

### 8. `internal/app/app.go` — Wire handler

Add to `App` struct:
```go
AdminPromptsHandler *handlers.AdminPromptsHandler
```

Add to `initHandlers()` after AdminUsersHandler block (~line 232):
```go
a.AdminPromptsHandler = handlers.NewAdminPromptsHandler(
    a.Logger,
    a.Config.IsDevMode(),
    jwtSecret,
    userLookup,
    vireClient.AdminListPrompts,
    vireClient.AdminGetPrompt,
    vireClient.AdminSetPrompt,
    serviceUserID,
)
a.AdminPromptsHandler.SetAPIURL(a.Config.API.URL)
```

### 9. `internal/server/routes.go` — Add routes

After line 66:
```go
mux.HandleFunc("GET /admin/prompts", s.app.AdminPromptsHandler.ServeHTTP)
mux.HandleFunc("GET /admin/prompts/{name}", s.app.AdminPromptsHandler.ServeHTTP)
mux.HandleFunc("POST /admin/prompts/{name}", s.app.AdminPromptsHandler.ServeHTTP)
```

## Test Cases

### Unit Tests (`internal/handlers/prompts_test.go`)

1. `TestAdminPromptsHandler_ListAdminAccess` — admin gets 200 with prompt data
2. `TestAdminPromptsHandler_ListUnauthenticatedRedirect` — no cookie → redirect /
3. `TestAdminPromptsHandler_ListNonAdminRedirect` — non-admin → redirect /dashboard
4. `TestAdminPromptsHandler_ListAPIError` — API error → warning banner
5. `TestAdminPromptsHandler_ListNoServiceKey` — no service key → config error
6. `TestAdminPromptsHandler_ListXSSEscaping` — prompt name/desc XSS escaped
7. `TestAdminPromptsHandler_EditAdminAccess` — admin gets 200 with prompt content
8. `TestAdminPromptsHandler_EditUnauthenticatedRedirect` — no cookie → redirect
9. `TestAdminPromptsHandler_EditNonAdminRedirect` — non-admin → redirect
10. `TestAdminPromptsHandler_EditAPIError` — API error on edit page
11. `TestAdminPromptsHandler_SaveSuccess` — POST returns JSON success
12. `TestAdminPromptsHandler_SaveNonAdmin` — non-admin POST → redirect/403
13. `TestAdminPromptsHandler_SaveUnauthenticated` — no cookie POST → redirect/403

### UI Tests (`tests/ui/prompts_test.go`)

1. `TestPromptsPageLayout` — .page and .page-body visible
2. `TestPromptsPageNavVisible` — nav visible
3. `TestPromptsPagePanelHeader` — PROMPTS header exists
4. `TestPromptsPageTableHeaders` — Name, Description, Override, Updated columns
5. `TestPromptsPageFooterVisible` — footer visible
6. `TestPromptsPageNoJSErrors` — no JS errors

### Nav Tests (update existing)

Update `tests/ui/nav_test.go` if it checks admin link text — "Admin" becomes "Users"/"Prompts".

## Edge Cases

- Prompt names with special chars (URL-encode in links, PathUnescape in handler)
- Empty prompt list (show "No prompts found" message)
- Prompt not found on edit (show error, don't crash)
- Save with empty content (allow — server decides validation)
- Large prompt content (~50KB textarea)
- Concurrent saves (last-write-wins, server handles)
- Non-JSON POST body → return 400

## Dependencies

- No new Go packages (uses standard library only)
- No new JS libraries (Alpine.js already available)
- Requires vire-server with admin prompts API endpoints
