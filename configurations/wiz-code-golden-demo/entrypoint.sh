#!/bin/bash
set -e

echo "========================================================================="
echo "⚡ Starting code-server Cloud Run environment..."
echo "========================================================================="

export HOME="/home/coder"
export PATH="${HOME}/.local/bin:${PATH}"

# Clone Demo Repository if not present
if [ ! -d "${HOME}/project/sorcery-solutions-frontend/.git" ]; then
  echo "📦 Cloning sorcery-solutions-frontend repository..."
  mkdir -p "${HOME}/project"
  git clone https://github.com/wiz-demo/sorcery-solutions-frontend.git "${HOME}/project/sorcery-solutions-frontend" 2>/dev/null || true
fi

PORT_NUMBER="${PORT:-8080}"
echo "🚀 Launching code-server on port ${PORT_NUMBER}..."

# Execute code-server directly as user coder
exec code-server \
  --auth none \
  --bind-addr "0.0.0.0:${PORT_NUMBER}" \
  "${HOME}/project/sorcery-solutions-frontend"
