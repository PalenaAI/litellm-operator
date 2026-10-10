#!/usr/bin/env bash
# Generate a file-based catalog for one already-built OLM bundle.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: generate-catalog.sh --opm <path> --bundle-dir <path> --bundle-image <image@sha256:...> \
  --output-dir <path> --package <name> --channel <name>
EOF
}

opm=''
bundle_dir=''
bundle_image=''
output_dir=''
package=''
channel=''

while [ "$#" -gt 0 ]; do
  case "$1" in
    --opm)
      opm="$2"
      shift 2
      ;;
    --bundle-dir)
      bundle_dir="$2"
      shift 2
      ;;
    --bundle-image)
      bundle_image="$2"
      shift 2
      ;;
    --output-dir)
      output_dir="$2"
      shift 2
      ;;
    --package)
      package="$2"
      shift 2
      ;;
    --channel)
      channel="$2"
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

if [ -z "$opm" ] || [ -z "$bundle_dir" ] || [ -z "$bundle_image" ] || [ -z "$output_dir" ] || [ -z "$package" ] || [ -z "$channel" ]; then
  usage >&2
  exit 2
fi

case "$bundle_image" in
  *@sha256:*) ;;
  *)
    echo "bundle image must be pinned by SHA256 digest: $bundle_image" >&2
    exit 2
    ;;
esac

mkdir -p "$output_dir"

"$opm" render "$bundle_dir" --output yaml | awk -v image="$bundle_image" '
  $0 == "image: \"\"" && !replaced {
    print "image: " image
    replaced = 1
    next
  }
  { print }
  END {
    if (!replaced) {
      print "rendered bundle does not contain an empty image field" > "/dev/stderr"
      exit 1
    }
  }
' > "$output_dir/bundle.yaml"

cat > "$output_dir/package.yaml" <<EOF
---
defaultChannel: $channel
name: $package
schema: olm.package
EOF

bundle_name="$(awk -F ': ' '$1 == "name" { print $2; exit }' "$output_dir/bundle.yaml")"
if [ -z "$bundle_name" ]; then
  echo "rendered bundle does not contain a name" >&2
  exit 1
fi

bundle_package="$(awk -F ': ' '$1 == "package" { print $2; exit }' "$output_dir/bundle.yaml")"
if [ "$bundle_package" != "$package" ]; then
  echo "bundle package $bundle_package does not match requested catalog package $package" >&2
  exit 1
fi

cat > "$output_dir/channel.yaml" <<EOF
---
entries:
  - name: $bundle_name
name: $channel
package: $package
schema: olm.channel
EOF
