#!/bin/sh
# Install a dictionary-embedded GitHub release binary without sudo/build tools.
set -eu
repo=https://github.com/takeshy/skk-popup
api=https://api.github.com/repos/takeshy/skk-popup
version=${1:-latest}
if [ "$#" -gt 1 ]; then
  echo 'Usage: sh install_macos.sh [latest|vVERSION]' >&2
  exit 1
fi
if [ "$(uname -s)" != Darwin ]; then
  echo 'This installer is for macOS.' >&2
  exit 1
fi
case $(uname -m) in
arm64) asset=skk-popup-darwin-arm64 ;;
*)
  echo 'Only Apple Silicon macOS releases are currently available.' >&2
  exit 1
  ;;
esac
case "$version" in
latest) endpoint=$api/releases/latest ;;
'' | *[!A-Za-z0-9._-]* | .*)
  echo 'Invalid release version.' >&2
  exit 1
  ;;
v[0-9]*) endpoint=$api/releases/tags/$version ;;
*)
  echo 'Specify latest or a release tag such as v0.10.3.' >&2
  exit 1
  ;;
esac
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/skk-popup-install.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM
curl -fsSL --proto '=https' --proto-redir '=https' "$endpoint" -o "$work_dir/release.json"
version=$(plutil -extract tag_name raw -o - "$work_dir/release.json")
case "$version" in
'' | *[!A-Za-z0-9._-]* | .*)
  echo 'Invalid release tag returned by GitHub.' >&2
  exit 1
  ;;
v[0-9]*) ;;
*)
  echo 'Invalid release tag returned by GitHub.' >&2
  exit 1
  ;;
esac
# macOS's built-in JSON parser avoids a dependency on jq or Python.
index=0
digest=
while name=$(plutil -extract "assets.$index.name" raw -o - "$work_dir/release.json" 2>/dev/null); do
  if [ "$name" = "$asset" ]; then
    digest=$(plutil -extract "assets.$index.digest" raw -o - "$work_dir/release.json")
    break
  fi
  index=$((index + 1))
done
case "$digest" in
sha256:*) expected=${digest#sha256:} ;;
*)
  echo 'This release has no macOS binary with a SHA-256 digest.' >&2
  exit 1
  ;;
esac
if [ "${#expected}" != 64 ]; then
  echo 'Invalid SHA-256 digest.' >&2
  exit 1
fi
case "$expected" in
*[!0-9a-f]*)
  echo 'Invalid SHA-256 digest.' >&2
  exit 1
  ;;
esac
echo "Downloading skk-popup $version (Apple Silicon, dictionary included)..."
curl -fsSL --proto '=https' --proto-redir '=https' "$repo/releases/download/$version/$asset" -o "$work_dir/skk-popup"
actual=$(shasum -a 256 "$work_dir/skk-popup" | awk '{print $1}')
if [ "$actual" != "$expected" ]; then
  echo 'Checksum mismatch; nothing was installed.' >&2
  exit 1
fi
chmod 755 "$work_dir/skk-popup"
"$work_dir/skk-popup" version
install_dir=${SKK_POPUP_INSTALL_DIR:-"$HOME/.local/bin"}
mkdir -p "$install_dir"
if [ -e "$install_dir/skk-popup" ] || [ -L "$install_dir/skk-popup" ]; then
  backup=$(mktemp "$install_dir/skk-popup.backup.XXXXXX")
  cp -p "$install_dir/skk-popup" "$backup"
  echo "Previous binary saved at $backup"
fi
# Rename a complete file to avoid modifying a running executable in place.
staged=$(mktemp "$install_dir/skk-popup.install.XXXXXX")
install -m 755 "$work_dir/skk-popup" "$staged"
mv -f "$staged" "$install_dir/skk-popup"
echo "Installed $version at $install_dir/skk-popup"
echo "Start: $install_dir/skk-popup"
echo "Show:  $install_dir/skk-popup show"
echo 'If an older daemon is running, quit and restart it to use this version.'
