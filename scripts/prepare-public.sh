#!/usr/bin/env bash
# Stage the static SPA files Wrangler uploads (index.html, data.js, assets/).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PUB="$ROOT/public"
mkdir -p "$PUB"
ln -sfn ../index.html "$PUB/index.html"
ln -sfn ../data.js "$PUB/data.js"
ln -sfn ../assets "$PUB/assets"
cat > "$PUB/.assetsignore" <<'EOF'
**/.DS_Store
**/Thumbs.db
**/.git
**/node_modules
EOF
echo "Prepared $PUB"
