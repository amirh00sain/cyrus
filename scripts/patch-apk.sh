#!/usr/bin/env bash
# Adds the Java VpnService (android/java) and its manifest entries to an APK
# produced by fyne-cross, then aligns and re-signs it.
#
#   scripts/patch-apk.sh <in.apk> <out.apk>
#
# Needs: JDK 17, Android SDK (platform + build-tools), python3, zip, curl.
set -euo pipefail

IN="$(readlink -f "$1")"
OUT="$(readlink -f -m "$2")"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SDK="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-/usr/local/lib/android/sdk}}"
PLATFORM="$(ls -d "$SDK"/platforms/android-* | sort -V | tail -n1)"
BT="$(ls -d "$SDK"/build-tools/* | sort -V | tail -n1)"
APKTOOL_VERSION="${APKTOOL_VERSION:-2.10.0}"
W="$(mktemp -d)"

echo "SDK=$SDK platform=$PLATFORM build-tools=$BT"

# 1) Java -> dex
mkdir -p "$W/classes" "$W/dex"
javac --release 8 -classpath "$PLATFORM/android.jar" -d "$W/classes" \
  $(find "$ROOT/android/java" -name '*.java')
"$BT/d8" --release --min-api 21 --lib "$PLATFORM/android.jar" --output "$W/dex" \
  $(find "$W/classes" -name '*.class')

# 2) manifest: declare the activity and the service
curl -fsSL -o "$W/apktool.jar" \
  "https://github.com/iBotPeaches/Apktool/releases/download/v${APKTOOL_VERSION}/apktool_${APKTOOL_VERSION}.jar"
java -jar "$W/apktool.jar" d -s -f "$IN" -o "$W/app"

python3 - "$W/app/AndroidManifest.xml" "$ROOT/android/AndroidManifest.snippet.xml" <<'PY'
import re, sys
path, snippet_path = sys.argv[1], sys.argv[2]
manifest = open(path, encoding="utf-8").read()
snippet = re.sub(r"<!--.*?-->", "", open(snippet_path, encoding="utf-8").read(), flags=re.S)
manifest = manifest.replace('android:hasCode="false"', 'android:hasCode="true"')
if "</application>" in manifest:
    manifest = manifest.replace("</application>", snippet + "\n</application>", 1)
else:  # self-closing <application ... />
    manifest = re.sub(r"(<application\b[^>]*?)/>", lambda m: m.group(1) + ">" + snippet + "</application>", manifest, count=1)
if "OperaVpnService" not in manifest:
    sys.exit("could not patch AndroidManifest.xml")
open(path, "w", encoding="utf-8").write(manifest)
PY

java -jar "$W/apktool.jar" b "$W/app" -o "$W/patched.apk"

# 3) add our classes as an extra dex file
DEXN=2
while unzip -l "$W/patched.apk" | grep -q " classes${DEXN}.dex"; do DEXN=$((DEXN+1)); done
cp "$W/dex/classes.dex" "$W/classes${DEXN}.dex"
(cd "$W" && zip -q patched.apk "classes${DEXN}.dex")

# 4) align + sign (debug key; replace with your own keystore for releases)
"$BT/zipalign" -f -p 4 "$W/patched.apk" "$W/aligned.apk"
KS="${KEYSTORE:-$W/debug.keystore}"
if [ ! -f "$KS" ]; then
  keytool -genkeypair -keystore "$KS" -storepass android -keypass android \
    -alias androiddebugkey -dname "CN=Android Debug,O=Android,C=US" \
    -keyalg RSA -keysize 2048 -validity 10000 >/dev/null
fi
"$BT/apksigner" sign --ks "$KS" --ks-pass "pass:${KS_PASS:-android}" \
  --key-pass "pass:${KEY_PASS:-android}" --out "$OUT" "$W/aligned.apk"
"$BT/apksigner" verify "$OUT"
echo "Wrote $OUT"
