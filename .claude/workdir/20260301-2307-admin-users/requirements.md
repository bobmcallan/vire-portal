# Admin Users Page — Implementation Spec

## Scope
Add an admin-only page at `/admin/users` that displays all registered users in a table with email, name, role, provider, and creation date. Add a conditional "Users" nav link visible only to users with the `admin` role. Propagate `UserRole` through all page handlers that render the nav template.

**NOT in scope:** Role editing UI, user deletion, last_login field (API doesn't support it yet — show CreatedAt as "Joined").

---

## 1. New Handler: `internal/handlers/users.go`

**Struct:**
```go
type AdminUsersHandler struct {
    logger           *common.Logger
    templates        *template.Template
    devMode          bool
    jwtSecret        []byte
    userLookupFn     func(string) (*client.UserProfile, error)
    adminListUsersFn func(string) ([]client.AdminUser, error)
    serviceUserID    string
    apiURL           string
}
```

**Constructor:**
```go
func NewAdminUsersHandler(
    logger *common.Logger,
    devMode bool,
    jwtSecret []byte,
    userLookupFn func(string) (*client.UserProfile, error),
    adminListUsersFn func(string) ([]client.AdminUser, error),
    serviceUserID string,
) *AdminUsersHandler
```

Follow standard template parse pattern:
```go
pagesDir := FindPagesDir()
templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))
```

**SetAPIURL method:**
```go
func (h *AdminUsersHandler) SetAPIURL(apiURL string) {
    h.apiURL = apiURL
}
```

**ServeHTTP method:**
```go
func (h *AdminUsersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
    if !loggedIn {
        http.Redirect(w, r, "/", http.StatusFound)
        return
    }

    // Gate: require admin role
    var userRole string
    if claims != nil && claims.Sub != "" && h.userLookupFn != nil {
        user, err := h.userLookupFn(claims.Sub)
        if err == nil && user != nil {
            userRole = user.Role
        }
    }
    if userRole != "admin" {
        http.Redirect(w, r, "/dashboard", http.StatusFound)
        return
    }

    // Fetch user list via admin API
    var users []client.AdminUser
    var fetchErr string
    if h.adminListUsersFn != nil && h.serviceUserID != "" {
        var err error
        users, err = h.adminListUsersFn(h.serviceUserID)
        if err != nil {
            if h.logger != nil {
                h.logger.Error().Str("error", err.Error()).Msg("failed to fetch admin user list")
            }
            fetchErr = "Failed to load user list. Ensure vire-server is running."
        }
    } else {
        fetchErr = "Admin API not configured. Set VIRE_SERVICE_KEY to enable."
    }

    data := map[string]interface{}{
        "Page":          "users",
        "DevMode":       h.devMode,
        "LoggedIn":      loggedIn,
        "UserRole":      userRole,
        "Users":         users,
        "UserCount":     len(users),
        "FetchError":    fetchErr,
        "PortalVersion": config.GetVersion(),
        "ServerVersion": GetServerVersion(h.apiURL),
    }

    if err := h.templates.ExecuteTemplate(w, "users.html", data); err != nil {
        if h.logger != nil {
            h.logger.Error().Str("template", "users.html").Str("error", err.Error()).Msg("failed to render users page")
        }
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}
```

---

## 2. New Template: `pages/users.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
    {{template "head.html" .}}
    <title>VIRE USERS</title>
</head>
<body>
    {{if .LoggedIn}}{{template "nav.html" .}}{{end}}
    <main class="page">
        <div class="page-body">

            {{if .FetchError}}
            <div class="warning-banner">
                <strong>ERROR:</strong> {{.FetchError}}
            </div>
            {{end}}

            <section class="panel-headed">
                <div class="panel-header">USERS [{{.UserCount}}]</div>
                <div class="panel-content">
                    {{if .Users}}
                    <div class="table-wrap">
                        <table class="tool-table">
                            <thead>
                                <tr>
                                    <th>Email</th>
                                    <th>Name</th>
                                    <th>Role</th>
                                    <th>Provider</th>
                                    <th>Joined</th>
                                </tr>
                            </thead>
                            <tbody>
                                {{range .Users}}
                                <tr>
                                    <td>{{.Email}}</td>
                                    <td>{{.Name}}</td>
                                    <td>{{.Role}}</td>
                                    <td>{{.Provider}}</td>
                                    <td>{{.CreatedAt}}</td>
                                </tr>
                                {{end}}
                            </tbody>
                        </table>
                    </div>
                    {{else}}
                    <p class="no-tools">No users found.</p>
                    {{end}}
                </div>
            </section>

        </div>
    </main>
    {{template "footer.html" .}}
</body>
</html>
```

Uses existing CSS classes: `.panel-headed`, `.panel-header`, `.panel-content`, `.table-wrap`, `.tool-table`, `.no-tools`, `.warning-banner`.

---

## 3. Nav Update: `pages/partials/nav.html`

Add conditional Users link in THREE places:

**Desktop nav links** (after Docs `<li>`):
```html
{{if eq .UserRole "admin"}}<li><a href="/admin/users" {{if eq .Page "users"}}class="active"{{end}}>Users</a></li>{{end}}
```

**Hamburger dropdown** (before Docs `<a>`):
```html
{{if eq .UserRole "admin"}}<a href="/admin/users">Users</a>{{end}}
```

**Mobile menu** (before Docs `<a>`):
```html
{{if eq .UserRole "admin"}}<a href="/admin/users">Users</a>{{end}}
```

---

## 4. Propagate UserRole to Existing Handlers

### `internal/handlers/dashboard.go`
The handler already calls `h.userLookupFn(claims.Sub)` for `navexaKeyMissing`. Capture the user role from that same call. Modify the block at lines 54-60 to also capture `userRole`:

```go
var userRole string
navexaKeyMissing := false
if h.userLookupFn != nil && claims != nil && claims.Sub != "" {
    user, err := h.userLookupFn(claims.Sub)
    if err == nil && user != nil {
        if !user.NavexaKeySet {
            navexaKeyMissing = true
        }
        userRole = user.Role
    }
}
```

Add `"UserRole": userRole,` to the data map.

### `internal/handlers/strategy.go`
Read the file to find the exact pattern. Same as dashboard — the handler has `userLookupFn`. Add UserRole capture from existing lookup and add to data map.

### `internal/handlers/capital.go`
Same pattern as dashboard/strategy. Add UserRole capture and add to data map.

### `internal/handlers/mcp_page.go`
This handler does NOT have `userLookupFn`. Must add:
1. Add `userLookupFn func(string) (*client.UserProfile, error)` field to struct
2. Add parameter to `NewMCPPageHandler` constructor
3. In `ServeHTTP`, look up user role and add to data map:
```go
var userRole string
if h.userLookupFn != nil && claims != nil && claims.Sub != "" {
    user, err := h.userLookupFn(claims.Sub)
    if err == nil && user != nil {
        userRole = user.Role
    }
}
// Add "UserRole": userRole to data map
```

---

## 5. App Struct: `internal/app/app.go`

**Add field:**
```go
AdminUsersHandler *handlers.AdminUsersHandler
```

**In `initHandlers()`**, after existing handler init:
```go
// Construct service user ID for admin API calls
portalID := a.Config.Service.PortalID
if portalID == "" {
    portalID, _ = os.Hostname()
}
serviceUserID := ""
if a.Config.Service.Key != "" {
    serviceUserID = "service:" + portalID
}

a.AdminUsersHandler = handlers.NewAdminUsersHandler(
    a.Logger,
    a.Config.IsDevMode(),
    jwtSecret,
    userLookup,
    vireClient.AdminListUsers,
    serviceUserID,
)
a.AdminUsersHandler.SetAPIURL(a.Config.API.URL)
```

**Update MCPPageHandler constructor** to pass `userLookup` as new parameter.

---

## 6. Route: `internal/server/routes.go`

Add after the profile routes:
```go
// Admin routes
mux.HandleFunc("GET /admin/users", s.app.AdminUsersHandler.ServeHTTP)
```

---

## 7. Unit Tests

Add to `internal/handlers/handlers_test.go` (or new file `internal/handlers/users_test.go`):

| Test | Description |
|------|-------------|
| `TestAdminUsersHandler_Returns200` | Admin gets 200, sees user emails and count |
| `TestAdminUsersHandler_RedirectsUnauthenticated` | No cookie → redirect to `/` |
| `TestAdminUsersHandler_RedirectsNonAdmin` | Non-admin → redirect to `/dashboard` |
| `TestAdminUsersHandler_ShowsErrorWhenAPIFails` | API error → warning banner with error message |
| `TestAdminUsersHandler_ShowsErrorWhenNotConfigured` | No service key → config error message |
| `TestAdminUsersHandler_XSSEscaping` | HTML in user data is escaped |
| `TestNavTemplate_UsersLinkForAdmin` | Admin sees Users link in nav |
| `TestNavTemplate_NoUsersLinkForNonAdmin` | Non-admin does NOT see Users link |
| `TestDashboardHandler_PassesUserRole` | Dashboard sets UserRole for admin nav rendering |

---

## 8. UI Tests: `tests/ui/users_test.go`

| Test | Description |
|------|-------------|
| `TestUsersPageLayout` | `.page` and `.page-body` classes present |
| `TestUsersPageNavVisible` | Nav bar is rendered |
| `TestUsersPagePanelHeader` | `.panel-header` contains "USERS" |
| `TestUsersPageTableHeaders` | Table has Email, Name, Role, Provider, Joined headers |
| `TestUsersPageFooterVisible` | Footer is rendered |
| `TestUsersPageNoJSErrors` | No JavaScript console errors |

**Note**: UI tests need the dev user to have admin role. Check test config for `VIRE_ADMIN_USERS`.

---

## 9. Implementation Order

1. `pages/users.html` — template must exist before handler parses it
2. `pages/partials/nav.html` — conditional Users link
3. `internal/handlers/users.go` — new handler
4. `internal/handlers/dashboard.go` — add UserRole
5. `internal/handlers/strategy.go` — add UserRole
6. `internal/handlers/capital.go` — add UserRole
7. `internal/handlers/mcp_page.go` — add userLookupFn + UserRole
8. `internal/app/app.go` — wire handler, update MCPPageHandler
9. `internal/server/routes.go` — register route
10. Unit tests
11. UI tests

---

## 10. Edge Cases

- **Service not registered yet**: Handler shows error banner, page still renders
- **API returns empty list**: Shows "No users found." message
- **Non-admin direct URL access**: Redirected to `/dashboard` (not 403)
- **XSS in user data**: Go templates auto-escape HTML
- **UserRole not set**: Nav conditional safely evaluates to false (link hidden)
