# Display available commands
[private]
default:
    @just --list

app_name := "SloppyPaste"
bundle_id := "dev.sglavoie.SloppyPaste"
version := `sed -n 's/.*static let version = "\(.*\)".*/\1/p' Sources/SloppyCore/SloppyCore.swift`
bundle_dir := ".build/bundle"
app_path := bundle_dir / app_name + ".app"
install_path := home_directory() / "Applications" / app_name + ".app"

# Self-signed identity kept in its own keychain, so the designated requirement
# (and with it the Accessibility grant) survives rebuilds.
signing_identity := "Sloppy Paste Local Signing"
signing_keychain := home_directory() / "Library/Keychains/sloppy-paste-signing.keychain-db"
signing_keychain_password := "sloppy-paste"

# Build all targets
build:
    swift build

# Run the test suite
test:
    swift test

# Remove build artifacts
clean:
    swift package clean
    rm -rf .build

# Build the app icon (AppIcon.icns) from Resources/icon.png
icon:
    #!/usr/bin/env bash
    set -euo pipefail
    iconset="{{bundle_dir}}/AppIcon.iconset"
    rm -rf "$iconset"
    mkdir -p "$iconset"
    for size in 16 32 128 256 512; do
        sips -z $size $size Resources/icon.png --out "$iconset/icon_${size}x${size}.png" >/dev/null
        double=$((size * 2))
        sips -z $double $double Resources/icon.png --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
    done
    iconutil -c icns -o "{{bundle_dir}}/AppIcon.icns" "$iconset"

# Assemble an unsigned SloppyPaste.app in .build/bundle
bundle: icon
    #!/usr/bin/env bash
    set -euo pipefail
    swift build -c release --product {{app_name}}
    bin_path="$(swift build -c release --show-bin-path)"
    build_number="$(git rev-list --count HEAD 2>/dev/null || echo 1)"
    rm -rf "{{app_path}}"
    mkdir -p "{{app_path}}/Contents/MacOS" "{{app_path}}/Contents/Resources"
    cp "$bin_path/{{app_name}}" "{{app_path}}/Contents/MacOS/{{app_name}}"
    cp "{{bundle_dir}}/AppIcon.icns" "{{app_path}}/Contents/Resources/AppIcon.icns"
    sed -e "s/@EXECUTABLE@/{{app_name}}/" \
        -e "s/@BUNDLE_ID@/{{bundle_id}}/" \
        -e "s/@VERSION@/{{version}}/" \
        -e "s/@BUILD@/$build_number/" \
        Resources/Info.plist.in > "{{app_path}}/Contents/Info.plist"
    printf 'APPL????' > "{{app_path}}/Contents/PkgInfo"
    plutil -lint "{{app_path}}/Contents/Info.plist"

# Build and sign SloppyPaste.app with the local signing identity
sign: bundle
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! -f "{{signing_keychain}}" ]]; then
        echo "error: signing keychain not found; run 'just setup-signing' once first" >&2
        exit 1
    fi
    security unlock-keychain -p "{{signing_keychain_password}}" "{{signing_keychain}}"
    codesign --force --timestamp=none \
        --sign "{{signing_identity}}" \
        --keychain "{{signing_keychain}}" \
        --identifier "{{bundle_id}}" \
        "{{app_path}}"
    codesign --verify --strict --verbose=2 "{{app_path}}"

# Sign, then replace the copy in ~/Applications and launch it
install: sign
    #!/usr/bin/env bash
    set -euo pipefail
    if pkill -x {{app_name}}; then
        while pgrep -x {{app_name}} >/dev/null; do sleep 0.1; done
    fi
    mkdir -p "$(dirname "{{install_path}}")"
    rm -rf "{{install_path}}"
    ditto "{{app_path}}" "{{install_path}}"
    open "{{install_path}}"

# Sign and launch the app straight from .build/bundle
run: sign
    open "{{app_path}}"

# Quit the app and remove it from ~/Applications
uninstall:
    pkill -x {{app_name}} || true
    rm -rf "{{install_path}}"

# Print the designated requirement of the built app
show-dr:
    @codesign -d -r- "{{app_path}}" 2>&1 | sed -n 's/^designated => //p'

# One-off: create the self-signed code-signing identity and trust it
setup-signing:
    #!/usr/bin/env bash
    set -euo pipefail
    keychain="{{signing_keychain}}"
    if [[ -f "$keychain" ]]; then
        echo "Signing keychain already exists at $keychain; leaving it as is."
        echo "Replacing it would change the designated requirement and void the Accessibility grant."
        exit 0
    fi
    workdir="$(mktemp -d)"
    trap 'rm -rf "$workdir"' EXIT
    cat > "$workdir/cert.cnf" <<'CNF'
    [req]
    distinguished_name = dn
    x509_extensions = ext
    prompt = no
    [dn]
    CN = {{signing_identity}}
    [ext]
    basicConstraints = critical,CA:false
    keyUsage = critical,digitalSignature
    extendedKeyUsage = critical,codeSigning
    CNF
    openssl req -x509 -quiet -newkey rsa:2048 -nodes -days 7300 \
        -keyout "$workdir/key.pem" -out "$workdir/cert.pem" -config "$workdir/cert.cnf"
    # macOS `security` only reads PKCS#12 files that use the legacy algorithms.
    openssl pkcs12 -export -legacy -passout pass:import \
        -inkey "$workdir/key.pem" -in "$workdir/cert.pem" \
        -name "{{signing_identity}}" -out "$workdir/identity.p12"
    security create-keychain -p "{{signing_keychain_password}}" "$keychain"
    # No auto-lock timeout; `just sign` unlocks it after a reboot.
    security set-keychain-settings "$keychain"
    security unlock-keychain -p "{{signing_keychain_password}}" "$keychain"
    security import "$workdir/identity.p12" -k "$keychain" -P import -T /usr/bin/codesign
    security set-key-partition-list -S apple-tool:,apple:,codesign: -s \
        -k "{{signing_keychain_password}}" "$keychain" >/dev/null
    security add-trusted-cert -r trustRoot -p codeSign -k "$keychain" "$workdir/cert.pem"
    security find-identity -p codesigning -v "$keychain"
