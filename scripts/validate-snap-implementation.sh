#!/bin/bash

# Snap package implementation validation script
# Verifies that all necessary files and configurations are in place

set -e

echo "=== Gthulhu Snap Implementation Validation ==="

ERRORS=0

# Function to check if file exists
check_file() {
    if [ -f "$1" ]; then
        echo "✓ $1"
    else
        echo "✗ Missing: $1"
        ERRORS=$((ERRORS + 1))
    fi
}

# Function to check if file is executable
check_executable() {
    if [ -x "$1" ]; then
        echo "✓ $1 (executable)"
    else
        echo "✗ Not executable: $1"
        ERRORS=$((ERRORS + 1))
    fi
}

# Function to validate YAML syntax
check_yaml() {
    if python3 -c "import yaml; yaml.safe_load(open('$1'))" 2>/dev/null; then
        echo "✓ $1 (valid YAML)"
    else
        echo "✗ Invalid YAML: $1"
        ERRORS=$((ERRORS + 1))
    fi
}

echo "Checking core snap files..."
check_file "snapcraft.yaml"
check_file ".snapcraft.yaml"
check_file "snap/local/launcher.sh"
check_executable "snap/local/launcher.sh"

echo ""
echo "Checking GitHub workflow..."
check_file ".github/workflows/snap.yaml"

echo ""
echo "Checking documentation..."
check_file "docs/SNAP.md"
check_file "docs/SNAP_STORE_LISTING.md"

echo ""
echo "Checking build scripts..."
check_file "scripts/test-snap-build.sh"
check_executable "scripts/test-snap-build.sh"

echo ""
echo "Validating YAML syntax..."
check_yaml "snapcraft.yaml"
check_yaml ".github/workflows/snap.yaml"

echo ""
echo "Checking Makefile snap targets..."
if grep -q "^snap:" Makefile; then
    echo "✓ Makefile contains snap targets"
else
    echo "✗ Makefile missing snap targets"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "Checking README updates..."
if grep -q "snap install" README.md; then
    echo "✓ README.md contains snap installation instructions"
else
    echo "✗ README.md missing snap installation instructions"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "Checking .gitignore updates..."
if grep -q "*.snap" .gitignore; then
    echo "✓ .gitignore includes snap artifacts"
else
    echo "✗ .gitignore missing snap artifacts"
    ERRORS=$((ERRORS + 1))
fi

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "🎉 All snap implementation checks passed!"
    echo ""
    echo "Next steps:"
    echo "1. Test snap build with: make snap-test"
    echo "2. Create snap store account and register name"
    echo "3. Set up SNAPCRAFT_TOKEN secret in GitHub"
    echo "4. Create a release tag to trigger automated publishing"
else
    echo "❌ Found $ERRORS issues that need to be fixed"
    exit 1
fi