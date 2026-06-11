# Code Signing & Notarization

Bzz uses an **active** CGEventTap (it can suppress events to intercept Enter
before submit). macOS only lets a properly **signed + notarized** binary use
that reliably — an unsigned build's active tap is silently ignored on a clean
system. So a signed release isn't optional for distribution.

## One-time setup (needs an Apple Developer Program membership, $99/yr)

### 1. Create a "Developer ID Application" certificate

Easiest way — via Xcode:
1. **Xcode → Settings → Accounts** → `+` → sign in with your Apple ID
2. Select the team → **Manage Certificates…**
3. `+` → **Developer ID Application**
4. The certificate lands in your login keychain automatically

Verify:
```bash
security find-identity -v -p codesigning
# should list:  "Developer ID Application: Your Name (TEAMID)"
```

### 2. Create an app-specific password (for notarytool)

1. https://appleid.apple.com/account/manage → **Sign-In and Security** → **App-Specific Passwords**
2. `+`, label it `bzz-notarization`
3. Copy the password (format `abcd-efgh-ijkl-mnop`)

### 3. Store the notarytool credentials

```bash
make setup-notary APPLE_ID=you@example.com TEAM_ID=ABCD123XYZ APP_PW=abcd-efgh-ijkl-mnop
```

`TEAM_ID` is shown at https://developer.apple.com/account → Membership.

## Per-release

```bash
make release-signed   # build → sign (hardened runtime) → DMG → notarize → staple → + Windows .exe
```

Outputs `build/Bzz.dmg` (notarized, stapled) and `build/Bzz.exe`.

Upload to a GitHub release as usual:
```bash
gh release create vX.Y.Z build/Bzz.dmg build/Bzz.exe --title "..." --notes "..."
```

## Entitlements

`packaging/entitlements.plist` keeps the hardened runtime strict: JIT and
unsigned-executable-memory are both off. No additional entitlements are
needed — Cmd+Shift+X synthesizes Cmd+C / Cmd+V via `CGEventPost` and reads
the clipboard via `NSPasteboard`, both of which are covered by the
Accessibility permission, not Apple Events.

## If you don't have a cert yet

`make release` (unsigned) still works for testing. Users will see the Gatekeeper
"unidentified developer" dialog and need **Open Anyway** in System Settings — a
~30% drop-off per step. Get the cert before any real launch (HN/Habr/PH).
