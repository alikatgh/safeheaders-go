#!/bin/bash
# Release automation script for SafeHeaders-Go

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Check if version argument is provided
if [ -z "$1" ]; then
    echo -e "${RED}Error: Version number required${NC}"
    echo "Usage: $0 <version>"
    echo "Example: $0 v0.5.1"
    exit 1
fi

VERSION="$1"

# Validate version format
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z0-9]+)?$ ]]; then
    echo -e "${RED}Error: Invalid version format${NC}"
    echo "Version must be in format: vX.Y.Z or vX.Y.Z-suffix"
    echo "Examples: v0.5.1, v1.0.0, v1.0.0-beta1"
    exit 1
fi

echo -e "${BLUE}SafeHeaders-Go Release Automation${NC}"
echo -e "${BLUE}===================================${NC}"
echo ""
echo "Preparing release: $VERSION"
echo ""

# Pre-flight checks
echo -e "${YELLOW}Running pre-flight checks...${NC}"

# Check if on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo -e "${RED}Error: Not on main branch (current: $CURRENT_BRANCH)${NC}"
    echo "Switch to main branch before releasing"
    exit 1
fi

# Check if working directory is clean
if [ -n "$(git status --porcelain)" ]; then
    echo -e "${RED}Error: Working directory is not clean${NC}"
    git status --short
    exit 1
fi

# Check if tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo -e "${RED}Error: Tag $VERSION already exists${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Pre-flight checks passed${NC}"
echo ""

# Run tests
echo -e "${YELLOW}Running tests...${NC}"
make test || {
    echo -e "${RED}Tests failed! Fix issues before releasing.${NC}"
    exit 1
}
echo -e "${GREEN}✓ Tests passed${NC}"
echo ""

# Run linter
echo -e "${YELLOW}Running linter...${NC}"
make lint || {
    echo -e "${RED}Linting failed! Fix issues before releasing.${NC}"
    exit 1
}
echo -e "${GREEN}✓ Linting passed${NC}"
echo ""

# Update CHANGELOG.md
echo -e "${YELLOW}Updating CHANGELOG.md...${NC}"
TODAY=$(date +%Y-%m-%d)
VERSION_NO_V=${VERSION#v}

# Check if version already in CHANGELOG
if grep -q "## \[$VERSION_NO_V\]" CHANGELOG.md; then
    echo -e "${YELLOW}Version $VERSION_NO_V already in CHANGELOG.md${NC}"
else
    # Add new version section
    sed -i "/## \[Unreleased\]/a \\
\\
## [$VERSION_NO_V] - $TODAY" CHANGELOG.md
    echo -e "${GREEN}✓ CHANGELOG.md updated${NC}"
fi
echo ""

# Commit CHANGELOG update
if [ -n "$(git status --porcelain CHANGELOG.md)" ]; then
    echo -e "${YELLOW}Committing CHANGELOG update...${NC}"
    git add CHANGELOG.md
    git commit -m "chore: update CHANGELOG for $VERSION"
    echo -e "${GREEN}✓ CHANGELOG committed${NC}"
    echo ""
fi

# Create git tag
echo -e "${YELLOW}Creating git tag $VERSION...${NC}"
git tag -a "$VERSION" -m "Release $VERSION"
echo -e "${GREEN}✓ Tag created${NC}"
echo ""

# Push to remote
echo -e "${YELLOW}Pushing to remote...${NC}"
echo "This will push:"
echo "  - Commits to main branch"
echo "  - Tag $VERSION"
echo ""
read -p "Continue? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Aborted. Tag created locally but not pushed.${NC}"
    echo "To push later: git push origin main --tags"
    exit 1
fi

git push origin main
git push origin "$VERSION"
echo -e "${GREEN}✓ Pushed to remote${NC}"
echo ""

# Summary
echo -e "${GREEN}=================================${NC}"
echo -e "${GREEN}Release $VERSION Complete!${NC}"
echo -e "${GREEN}=================================${NC}"
echo ""
echo "The GitHub Actions workflow will now:"
echo "  1. Run tests and linting"
echo "  2. Build binaries for Linux, macOS, Windows"
echo "  3. Create GitHub release with binaries"
echo "  4. Build and push Docker image"
echo ""
echo "Monitor progress at:"
echo "  https://github.com/alikatgh/safeheaders-go/actions"
echo ""
echo "View release at:"
echo "  https://github.com/alikatgh/safeheaders-go/releases/tag/$VERSION"
echo ""
