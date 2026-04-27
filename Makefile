.PHONY: build app dmg clean install

BINARY_NAME = Bzz
APP_NAME    = Bzz.app
DMG_NAME    = Bzz.dmg
BUILD_DIR   = build
APP_DIR     = $(BUILD_DIR)/$(APP_NAME)
RESOURCES   = $(APP_DIR)/Contents/Resources
MACOS_DIR   = $(APP_DIR)/Contents/MacOS
ICONSET     = /tmp/Bzz.iconset
VERSION     = 1.0.0

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
	@echo "  ✔  $(APP_DIR)"

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
dmg: app
	@echo "Creating DMG..."
	@rm -f $(BUILD_DIR)/$(DMG_NAME)
	@# Create a temp dir for DMG contents
	@rm -rf /tmp/bzz_dmg
	@mkdir /tmp/bzz_dmg
	@cp -r $(APP_DIR) /tmp/bzz_dmg/
	@ln -s /Applications /tmp/bzz_dmg/Applications
	@hdiutil create \
		-volname "$(BINARY_NAME) $(VERSION)" \
		-srcfolder /tmp/bzz_dmg \
		-ov -format UDZO \
		$(BUILD_DIR)/$(DMG_NAME)
	@rm -rf /tmp/bzz_dmg
	@echo "  ✔  $(BUILD_DIR)/$(DMG_NAME)"

# --------------------------------------------------------------------------
install: app
	@echo "Installing to ~/Applications/..."
	@cp -r $(APP_DIR) ~/Applications/$(APP_NAME)
	@echo "  ✔  ~/Applications/$(APP_NAME)"

# --------------------------------------------------------------------------
build-windows:
	@echo "Building $(BINARY_NAME) for Windows..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
		go build -ldflags="-s -w -H windowsgui" -o $(BUILD_DIR)/$(BINARY_NAME).exe .
	@echo "  ✔  $(BUILD_DIR)/$(BINARY_NAME).exe"

# --------------------------------------------------------------------------
release: dmg build-windows
	@echo "Release artifacts:"
	@ls -lh $(BUILD_DIR)/$(DMG_NAME) $(BUILD_DIR)/$(BINARY_NAME).exe

# --------------------------------------------------------------------------
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)/$(BINARY_NAME) $(APP_DIR) $(BUILD_DIR)/$(DMG_NAME)
	@echo "  ✔  clean"
