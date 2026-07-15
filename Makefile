.PHONY: build app icon dmg dmg-signed sign notarize staple release release-signed clean install setup-notary entitlements universal app-universal dmg-universal

BINARY_NAME = Bzz
APP_NAME    = Bzz.app
DMG_NAME    = Bzz.dmg
BUILD_DIR   = build
APP_DIR     = $(BUILD_DIR)/$(APP_NAME)
RESOURCES   = $(APP_DIR)/Contents/Resources
MACOS_DIR   = $(APP_DIR)/Contents/MacOS
ICONSET     = /tmp/Bzz.iconset
VERSION     = 0.7.0

# --- Code signing config (override on the command line or via env) -----------
# Find your "Developer ID Application" identity with:  security find-identity -v -p codesigning
SIGN_IDENTITY ?= Developer ID Application
# notarytool keychain profile name created by `make setup-notary`
NOTARY_PROFILE ?= bzz-notarization
BUNDLE_ID = com.zlopixatel.bzz

# --------------------------------------------------------------------------
build:
	@echo "Building $(BINARY_NAME)..."
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Binary: $(BUILD_DIR)/$(BINARY_NAME)"

# --------------------------------------------------------------------------
app: build icon
	@echo "Creating .app bundle..."
	@mkdir -p $(MACOS_DIR) $(RESOURCES)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(MACOS_DIR)/$(BINARY_NAME)
	@chmod +x $(MACOS_DIR)/$(BINARY_NAME)
	@cp packaging/Info.plist $(APP_DIR)/Contents/Info.plist
	@# Substitute version in Info.plist so VERSION (Makefile) is single source of truth
	@/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $(VERSION)" $(APP_DIR)/Contents/Info.plist
	@/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $(VERSION)" $(APP_DIR)/Contents/Info.plist
	@echo "  ✔  $(APP_DIR) (v$(VERSION))"

icon:
	@echo "Generating icon..."
	@python3 scripts/gen_icon.py
	@mkdir -p $(ICONSET)
	@for size in 16 32 64 128 256 512; do \
		sips -z $$size $$size /tmp/bzz_icon_src.png \
			--out $(ICONSET)/icon_$${size}x$${size}.png > /dev/null; \
		double=$$((size * 2)); \
		sips -z $$double $$double /tmp/bzz_icon_src.png \
			--out $(ICONSET)/icon_$${size}x$${size}@2x.png > /dev/null; \
	done
	@mkdir -p $(RESOURCES)
	@iconutil -c icns $(ICONSET) -o $(RESOURCES)/AppIcon.icns
	@echo "  ✔  $(RESOURCES)/AppIcon.icns"

# --------------------------------------------------------------------------
# Sign the .app with Developer ID + hardened runtime (required for notarization)
sign: app
	@echo "Signing $(APP_NAME) with '$(SIGN_IDENTITY)'..."
	codesign --force --deep --options runtime --timestamp \
		--entitlements packaging/entitlements.plist \
		--sign "$(SIGN_IDENTITY)" \
		$(APP_DIR)
	@echo "Verifying signature..."
	codesign --verify --deep --strict --verbose=2 $(APP_DIR)
	spctl --assess --type execute --verbose $(APP_DIR) || true
	@echo "  ✔  signed $(APP_DIR)"

# --------------------------------------------------------------------------
# One-time: store Apple ID credentials for notarytool in the login keychain.
# Usage:  make setup-notary APPLE_ID=you@example.com TEAM_ID=ABCD123XYZ APP_PW=abcd-efgh-ijkl-mnop
setup-notary:
	@test -n "$(APPLE_ID)" || (echo "ERROR: pass APPLE_ID=you@example.com"; exit 1)
	@test -n "$(TEAM_ID)"  || (echo "ERROR: pass TEAM_ID=ABCD123XYZ"; exit 1)
	@test -n "$(APP_PW)"   || (echo "ERROR: pass APP_PW=app-specific-password"; exit 1)
	xcrun notarytool store-credentials "$(NOTARY_PROFILE)" \
		--apple-id "$(APPLE_ID)" \
		--team-id "$(TEAM_ID)" \
		--password "$(APP_PW)"
	@echo "  ✔  notarytool profile '$(NOTARY_PROFILE)' stored"

# --------------------------------------------------------------------------
# Unsigned DMG — also used as the build step for dmg-signed.
dmg: app
	@echo "Creating DMG..."
	@rm -f $(BUILD_DIR)/$(DMG_NAME)
	@rm -rf /tmp/bzz_dmg && mkdir /tmp/bzz_dmg
	@cp -R $(APP_DIR) /tmp/bzz_dmg/
	@ln -s /Applications /tmp/bzz_dmg/Applications
	@hdiutil create -volname "$(BINARY_NAME) $(VERSION)" \
		-srcfolder /tmp/bzz_dmg -ov -format UDZO $(BUILD_DIR)/$(DMG_NAME) > /dev/null
	@rm -rf /tmp/bzz_dmg
	@echo "  ✔  $(BUILD_DIR)/$(DMG_NAME)"

# --------------------------------------------------------------------------
# Universal (arm64 + x86_64) build so one DMG runs on both Apple Silicon and
# Intel Macs. Uses lipo to fuse two cgo builds. Not notarized — ad-hoc signed.
universal:
	@echo "Building universal binary (arm64 + x86_64)..."
	@mkdir -p $(BUILD_DIR)
	@GOARCH=arm64 CGO_ENABLED=1 go build -ldflags="-s -w" -o $(BUILD_DIR)/Bzz-arm64 .
	@GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="-s -w" -o $(BUILD_DIR)/Bzz-amd64 .
	@lipo -create -output $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/Bzz-arm64 $(BUILD_DIR)/Bzz-amd64
	@rm -f $(BUILD_DIR)/Bzz-arm64 $(BUILD_DIR)/Bzz-amd64
	@lipo -info $(BUILD_DIR)/$(BINARY_NAME)

# .app around the universal binary (mirrors `app`, but skips the host-only build).
app-universal: universal icon
	@echo "Creating universal .app bundle..."
	@mkdir -p $(MACOS_DIR) $(RESOURCES)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(MACOS_DIR)/$(BINARY_NAME)
	@chmod +x $(MACOS_DIR)/$(BINARY_NAME)
	@cp packaging/Info.plist $(APP_DIR)/Contents/Info.plist
	@/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $(VERSION)" $(APP_DIR)/Contents/Info.plist
	@/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $(VERSION)" $(APP_DIR)/Contents/Info.plist
	@codesign --force --deep -s - $(APP_DIR)
	@echo "  ✔  $(APP_DIR) (v$(VERSION), universal, ad-hoc signed)"

# Universal unsigned DMG for GitHub Releases.
dmg-universal: app-universal
	@echo "Creating universal DMG..."
	@rm -f $(BUILD_DIR)/$(DMG_NAME)
	@rm -rf /tmp/bzz_dmg && mkdir /tmp/bzz_dmg
	@cp -R $(APP_DIR) /tmp/bzz_dmg/
	@ln -s /Applications /tmp/bzz_dmg/Applications
	@hdiutil create -volname "$(BINARY_NAME) $(VERSION)" \
		-srcfolder /tmp/bzz_dmg -ov -format UDZO $(BUILD_DIR)/$(DMG_NAME) > /dev/null
	@rm -rf /tmp/bzz_dmg
	@echo "  ✔  $(BUILD_DIR)/$(DMG_NAME)"

# Signed DMG: build the unsigned DMG via the `dmg` target, then codesign it.
# `dmg-signed` also depends on `sign` so the .app inside has hardened runtime.
dmg-signed: sign dmg
	codesign --force --sign "$(SIGN_IDENTITY)" --timestamp $(BUILD_DIR)/$(DMG_NAME)
	@echo "  ✔  signed $(BUILD_DIR)/$(DMG_NAME)"

notarize: dmg-signed
	@echo "Submitting to Apple notary service (this can take a few minutes)..."
	xcrun notarytool submit $(BUILD_DIR)/$(DMG_NAME) \
		--keychain-profile "$(NOTARY_PROFILE)" --wait
	@echo "Stapling ticket..."
	xcrun stapler staple $(BUILD_DIR)/$(DMG_NAME)
	xcrun stapler validate $(BUILD_DIR)/$(DMG_NAME)
	@echo "  ✔  notarized & stapled $(BUILD_DIR)/$(DMG_NAME)"

# Alias
staple: notarize

# --------------------------------------------------------------------------
install: app
	@echo "Installing to /Applications/..."
	@# rm first: cp -r into an existing .app would nest the bundle inside it
	@rm -rf /Applications/$(APP_NAME)
	@cp -R $(APP_DIR) /Applications/$(APP_NAME)
	@echo "  ✔  /Applications/$(APP_NAME)"

# --------------------------------------------------------------------------
build-windows:
	@echo "Building $(BINARY_NAME) for Windows..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
		go build -ldflags="-s -w -H windowsgui" -o $(BUILD_DIR)/$(BINARY_NAME).exe .
	@echo "  ✔  $(BUILD_DIR)/$(BINARY_NAME).exe"

# --------------------------------------------------------------------------
# Unsigned release (current)
release: dmg build-windows
	@echo "Release artifacts:"
	@ls -lh $(BUILD_DIR)/$(DMG_NAME) $(BUILD_DIR)/$(BINARY_NAME).exe

# Signed + notarized macOS release + Windows exe
release-signed: notarize build-windows
	@echo "Signed release artifacts:"
	@ls -lh $(BUILD_DIR)/$(DMG_NAME) $(BUILD_DIR)/$(BINARY_NAME).exe

# --------------------------------------------------------------------------
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)/$(BINARY_NAME) $(APP_DIR) $(BUILD_DIR)/$(DMG_NAME) $(BUILD_DIR)/$(BINARY_NAME).exe 
	@echo "  ✔  clean"
