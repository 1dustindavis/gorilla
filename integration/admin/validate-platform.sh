#!/usr/bin/env bash
# Native OS/filesystem validation for the existing repository-admin command.
set -euo pipefail

gorilla="${1:?usage: smoke.sh <path-to-gorilla-binary>}"
gorilla="$(cd "$(dirname "$gorilla")" && pwd)/$(basename "$gorilla")"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT

mkdir -p "$fixture"/{manifests,packages-info,packages/live,packages/abandoned}
cat > "$fixture/manifests/main.yaml" <<'EOF'
name: smoke
managed_installs:
  - Live
EOF
for version in 1 2 3; do
  cat > "$fixture/packages-info/live-$version.yaml" <<EOF
item_name: Live
catalog: base
version: $version.0
installer:
  location: packages/live/live-$version.exe
EOF
  printf 'fixture %s\n' "$version" > "$fixture/packages/live/live-$version.exe"
done
cat > "$fixture/packages-info/abandoned.yaml" <<'EOF'
item_name: Abandoned
catalog: base
version: 1.0
installer:
  location: packages/abandoned/abandoned.exe
EOF
printf 'abandoned\n' > "$fixture/packages/abandoned/abandoned.exe"

"$gorilla" admin build --repo "$fixture"
test -s "$fixture/catalogs/base.yaml"
grep -q 'Live:' "$fixture/catalogs/base.yaml"
grep -q 'version: "3.0"' "$fixture/catalogs/base.yaml" || grep -q 'version: 3.0' "$fixture/catalogs/base.yaml"

"$gorilla" admin cleanup --repo "$fixture" --keep 1
# Dry-run must not mutate the fixture.
test -f "$fixture/packages-info/live-1.yaml"
test -f "$fixture/packages-info/abandoned.yaml"
test -f "$fixture/packages/abandoned/abandoned.exe"

"$gorilla" admin cleanup --repo "$fixture" --keep 1 --apply
test -f "$fixture/packages-info/live-3.yaml"
test ! -e "$fixture/packages-info/live-2.yaml"
test ! -e "$fixture/packages-info/live-1.yaml"
test ! -e "$fixture/packages-info/abandoned.yaml"
test -f "$fixture/packages/live/live-3.exe"
test ! -e "$fixture/packages/live/live-1.exe"
test ! -e "$fixture/packages/live/live-2.exe"
test ! -e "$fixture/packages/abandoned/abandoned.exe"

"$gorilla" admin build --repo "$fixture"
test -s "$fixture/catalogs/base.yaml"
grep -q 'Live:' "$fixture/catalogs/base.yaml"
if grep -q 'Abandoned:' "$fixture/catalogs/base.yaml"; then
  echo "Abandoned item survived catalog regeneration" >&2
  exit 1
fi
echo "Native repository-admin smoke test passed"
