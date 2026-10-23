#!/usr/bin/env bash
# check-docs-version.sh checks that the docs in a source tree are versioned for a release.
# Usage: check-docs-version.sh <vX.Y.Z> <source dir>
set -euo pipefail

TAG="${1#v}"
SRC="$2"
echo "Release version: ${TAG}"

MAJOR=$(echo "$TAG" | cut -d. -f1)
MINOR=$(echo "$TAG" | cut -d. -f2)
MINOR_VERSION="${MAJOR}.${MINOR}.0"
echo "Expected docs version: ${MINOR_VERSION}"

if ! grep -q "\"${MINOR_VERSION}\"" "${SRC}/docs/versions.json"; then
  echo "::error::Documentation version ${MINOR_VERSION} is NOT in docs/versions.json"
  echo "Before releasing, version the docs:"
  echo "  cd docs && npm run docusaurus docs:version ${MINOR_VERSION}"
  exit 1
fi

VERSIONED_DIR="docs/versioned_docs/version-${MINOR_VERSION}"
if [ ! -d "${SRC}/${VERSIONED_DIR}" ]; then
  echo "::error::${VERSIONED_DIR}/ directory is missing"
  exit 1
fi

DIFF_OUTPUT=$(diff -r "${SRC}/docs/docs/" "${SRC}/${VERSIONED_DIR}/" 2>&1) || true
if [ -n "$DIFF_OUTPUT" ]; then
  echo "::error::Versioned docs (${VERSIONED_DIR}) are out of sync with docs/docs/"
  echo "The following files differ:"
  echo "$DIFF_OUTPUT"
  echo "Re-version the docs to pick up the latest changes:"
  echo "  cd docs && rm -rf versioned_docs/version-${MINOR_VERSION} versioned_sidebars/version-${MINOR_VERSION}-sidebars.json"
  echo "  npm run docusaurus docs:version ${MINOR_VERSION}"
  exit 1
fi

echo "Documentation version ${MINOR_VERSION} is present and up to date."
