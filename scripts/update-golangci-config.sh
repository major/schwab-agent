#!/bin/zsh
set -eu

VERSION="${1:-v2.12.2}"
CONFIG_URL="https://raw.githubusercontent.com/maratori/golangci-lint-config/${VERSION}/.golangci.yml"
MODULE_PATH="github.com/major/schwab-agent"

curl -fsSL "${CONFIG_URL}" -o .golangci.yml
python3 - <<PYTHON_INNER
from pathlib import Path

path = Path(".golangci.yml")
config = path.read_text()
config = config.replace("        - github.com/my/project", "        - ${MODULE_PATH}")
config = config.replace(
    "          - noctx\n          - wrapcheck\n",
    "          - noctx\n          - paralleltest\n          - testpackage\n          - wrapcheck\n",
)
path.write_text(config)
PYTHON_INNER
