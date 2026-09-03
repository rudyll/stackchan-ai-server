#!/bin/sh
# Collect notices for the actual Go dependencies, not just direct go.mod entries.
set -eu
OUTPUT_DIR=${1:?Usage: collect-licenses.sh OUTPUT_DIR}
mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR=$(cd "$OUTPUT_DIR" && pwd)
GO_ROOT=$(go env GOROOT)
GO_LICENSE="$GO_ROOT/LICENSE"
[ -f "$GO_LICENSE" ] || GO_LICENSE="$(dirname "$GO_ROOT")/LICENSE"
cp "$GO_LICENSE" "$OUTPUT_DIR/Go.txt"
MODULE_LIST=$(mktemp)
trap 'rm -f "$MODULE_LIST"' EXIT
go list -mod=readonly -deps -f '{{if .Module}}{{.Module.Path}}|{{.Module.Dir}}{{end}}' . > "$MODULE_LIST"
sort -u -o "$MODULE_LIST" "$MODULE_LIST"
while IFS='|' read -r module_path module_dir; do
    [ -n "$module_dir" ] || continue
    module_name=$(printf '%s' "$module_path" | tr '/:' '__')
    find "$module_dir" -maxdepth 1 -type f \( -iname 'license*' -o -iname 'copying*' -o -iname 'notice*' \) -print | while IFS= read -r license_file; do
        cp "$license_file" "$OUTPUT_DIR/$module_name-$(basename "$license_file")"
    done
done < "$MODULE_LIST"
