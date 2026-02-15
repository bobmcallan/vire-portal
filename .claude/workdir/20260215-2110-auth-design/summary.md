# Summary: Authentication Design Document

**Date:** 2026-02-15
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `docs/authentication.md` | New — authentication design document |

## Notes
- Document covers current state, target architecture, all four auth methods (Google, GitHub, email/password, dev login)
- Dev login redesigned to call vire-server for a signed JWT instead of building an unsigned token locally
- Recommends server-side redirect pattern — portal doesn't hold OAuth secrets, only jwt_secret and callback_url
- Four implementation phases: signed JWT + dev login, Google OAuth, GitHub OAuth, email/password
- Portal and vire-server change lists included with file-level detail
