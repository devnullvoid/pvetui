#!/bin/bash

# Automated Release Script for proxmox-tui
# This script automates the complete release process:
# 1. Updates changelog (moves Unreleased to new version)
# 2. Commits changelog changes
# 3. Merges develop to master
# 4. Creates and pushes release tag
# 5. Creates GitHub release
#
# Usage: ./scripts/create-release.sh v0.6.0 [--dry-run] [--github|--no-github]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
DRY_RUN=false
NO_GITHUB=true
CURRENT_BRANCH=""

# Parse arguments
VERSION=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --no-github)
            NO_GITHUB=true
            shift
            ;;
        --github)
            NO_GITHUB=false
            shift
            ;;
        -h|--help)
            echo "Usage: $0 <version> [--dry-run] [--github|--no-github]"
            echo ""
            echo "Arguments:"
            echo "  version     Release version (e.g., v0.6.0)"
            echo ""
            echo "Options:"
            echo "  --dry-run   Show what would be done without making changes"
            echo "  --github    Create GitHub release (requires GitHub CLI)"
            echo "  --no-github Skip GitHub release creation (default)"
            echo "  --help      Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0 v0.6.0                    # Tag and merge only"
            echo "  $0 v0.6.0 --github          # Tag, merge, and create GitHub release"
            echo "  $0 v0.6.0 --dry-run         # Preview changes"
            exit 0
            ;;
        v[0-9]*)
            if [[ -z "$VERSION" ]]; then
                VERSION="$1"
            else
                echo -e "${RED}Error: Multiple version arguments provided${NC}"
                exit 1
            fi
            shift
            ;;
        *)
            echo -e "${RED}Error: Unknown argument '$1'${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

if [[ -z "$VERSION" ]]; then
    echo -e "${RED}Error: Version argument required${NC}"
    echo "Usage: $0 <version> [--dry-run] [--github|--no-github]"
    echo "Example: $0 v0.6.0"
    exit 1
fi

VERSION_NO_V="${VERSION#v}"
RELEASE_DATE=$(date +%Y-%m-%d)

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

run_command() {
    local cmd="$1"
    local description="$2"

    if [[ "$DRY_RUN" == "true" ]]; then
        echo -e "${YELLOW}[DRY RUN] Would run: $cmd${NC}"
        return 0
    fi

    log_info "$description"
    if eval "$cmd"; then
        log_success "$description completed"
    else
        log_error "$description failed"
        exit 1
    fi
}

# Validation functions
check_git_status() {
    if [[ -n $(git status --porcelain) ]]; then
        log_error "Working directory is not clean. Please commit or stash changes."
        git status --short
        exit 1
    fi
}

check_branch() {
    CURRENT_BRANCH=$(git branch --show-current)
    if [[ "$CURRENT_BRANCH" != "develop" ]]; then
        log_error "Must be on 'develop' branch to create release. Currently on '$CURRENT_BRANCH'"
        exit 1
    fi
}

check_unreleased_content() {
    if ! grep -q "## \[Unreleased\]" CHANGELOG.md; then
        log_error "No [Unreleased] section found in CHANGELOG.md"
        exit 1
    fi

    # Check if there's actual content in Unreleased section
    local unreleased_content
    unreleased_content=$(sed -n '/## \[Unreleased\]/,/^## \[/p' CHANGELOG.md | sed '/^## \[/d' | grep -v '^[[:space:]]*$' | wc -l)

    if [[ $unreleased_content -eq 0 ]]; then
        log_error "No content found in [Unreleased] section of CHANGELOG.md"
        exit 1
    fi
}

check_version_not_exists() {
    if grep -q "## \[$VERSION_NO_V\]" CHANGELOG.md; then
        log_error "Version $VERSION_NO_V already exists in CHANGELOG.md"
        exit 1
    fi

    if git tag | grep -q "^$VERSION$"; then
        log_error "Tag $VERSION already exists"
        exit 1
    fi
}

check_github_cli() {
    if [[ "$NO_GITHUB" == "false" ]] && ! command -v gh &> /dev/null; then
        log_error "GitHub CLI (gh) is required for GitHub release creation"
        log_info "Install with: https://cli.github.com/"
        log_info "Or use --no-github flag to skip GitHub release"
        exit 1
    fi
}

# Main functions
update_changelog() {
    log_info "Updating CHANGELOG.md..."

    local temp_file
    temp_file=$(mktemp)
    trap 'rm -f "$temp_file"' EXIT

    # 1. Capture the header
    sed -n '1,/## \[Unreleased\]/p' CHANGELOG.md | head -n -1 > "$temp_file"

    # 2. Add the new [Unreleased] section
    echo "## [Unreleased]" >> "$temp_file"
    echo "" >> "$temp_file"

    # 3. Add the new version header
    echo "## [$VERSION_NO_V] - $RELEASE_DATE" >> "$temp_file"

    # 4. Extract and append the content from the old [Unreleased] section
    sed -n '/## \[Unreleased\]/,/^## \[/p' CHANGELOG.md | sed '1d;$d' >> "$temp_file"

    # 5. Append the rest of the file (old versions)
    # Find the line number of the second `## [` tag (the start of the previous version)
    local next_version_line
    next_version_line=$(grep -n -m 2 '^## \[' CHANGELOG.md | tail -n 1 | cut -d: -f1)

    if [[ -n "$next_version_line" ]]; then
        tail -n +"$next_version_line" CHANGELOG.md >> "$temp_file"
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        log_warning "[DRY RUN] Would update CHANGELOG.md with version $VERSION_NO_V"
        echo "New changelog structure:"
        cat "$temp_file"
        return 0
    fi

    # Replace original file
    mv "$temp_file" CHANGELOG.md
    log_success "CHANGELOG.md updated with version $VERSION_NO_V"
}

commit_changelog() {
    run_command \
        "git add CHANGELOG.md && git commit --no-verify -m '📝 Prepare $VERSION release

- Update changelog with $VERSION release notes
- Ready for release tagging and merge to master'" \
        "Committing changelog update"
}

merge_to_master() {
    run_command "git checkout master" "Switching to master branch"
    run_command "git merge develop" "Merging develop into master"
}

create_and_push_tag() {
    local tag_message="Release $VERSION

🚀 Release $VERSION

See CHANGELOG.md for full details of changes and improvements."

    run_command "git tag -a $VERSION -m '$tag_message'" "Creating release tag $VERSION"
    run_command "git push origin master" "Pushing master branch"
    run_command "git push origin $VERSION" "Pushing release tag"
}

create_github_release() {
    if [[ "$NO_GITHUB" == "true" ]]; then
        log_info "Skipping GitHub release creation (--no-github flag)"
        return 0
    fi

    log_info "Creating GitHub release..."

    # Create temporary file with release notes
    local temp_file
    temp_file=$(mktemp)
    trap 'rm -f "$temp_file"' EXIT

    # Extract changelog content for this version
    sed -n "/## \[$VERSION_NO_V\]/,/^## \[/p" CHANGELOG.md | sed '/^## \[/d' | tail -n +2 > "$temp_file"

    # Add installation instructions
    cat >> "$temp_file" << 'EOF'

## 📦 Downloads

Choose the appropriate binary for your platform:

- **Linux AMD64**: `proxmox-tui-linux-amd64.tar.gz`
- **Linux ARM64**: `proxmox-tui-linux-arm64.tar.gz`
- **macOS Intel**: `proxmox-tui-darwin-amd64.tar.gz`
- **macOS Apple Silicon**: `proxmox-tui-darwin-arm64.tar.gz`
- **Windows**: `proxmox-tui-windows-amd64.zip`

## 🔐 Verification

Verify your download with the provided `checksums.txt` file:
```bash
shasum -a 256 -c checksums.txt
```

## 📋 Installation

1. Download the appropriate archive for your platform
2. Extract the binary: `tar -xzf proxmox-tui-*.tar.gz` (or unzip for Windows)
3. Make executable (Unix): `chmod +x proxmox-tui-*`
4. Run: `./proxmox-tui-* --help`
EOF

    if [[ "$DRY_RUN" == "true" ]]; then
        log_warning "[DRY RUN] Would create GitHub release $VERSION"
        echo "Release notes preview:"
        head -10 "$temp_file"
        echo "..."
        return 0
    fi

    # Create the release
    gh release create "$VERSION" \
        --title "Release $VERSION" \
        --notes-file "$temp_file" \
        --draft

    log_success "GitHub release created successfully!"
    local repo_url
    repo_url=$(gh repo view --json url -q '.url')
    echo "   📋 Review at: $repo_url/releases"
    echo "   🚀 Publish when ready with: gh release edit $VERSION --draft=false"
}

cleanup_and_return() {
    run_command "git checkout develop" "Returning to develop branch"
    run_command "git push origin develop" "Pushing updated develop branch"
}

# Main execution
main() {
    echo -e "${BLUE}🚀 Starting automated release process for $VERSION${NC}"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        log_warning "DRY RUN MODE - No changes will be made"
        echo ""
    fi

    # Pre-flight checks
    log_info "Running pre-flight checks..."
    check_git_status
    check_branch
    check_unreleased_content
    check_version_not_exists
    check_github_cli
    log_success "All pre-flight checks passed"
    echo ""

    # Show what will be released
    log_info "Release summary:"
    echo "  📋 Version: $VERSION"
    echo "  📅 Date: $RELEASE_DATE"
    echo "  🌿 Current branch: $CURRENT_BRANCH"
    echo "  🎯 Target: master"

    if [[ "$NO_GITHUB" == "true" ]]; then
        echo "  🚫 GitHub release: Skipped"
    else
        echo "  🐙 GitHub release: Yes (draft)"
    fi
    echo ""

    # Confirm unless dry run
    if [[ "$DRY_RUN" == "false" ]]; then
        read -r -p "Proceed with release? (y/N): " REPLY
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Release cancelled by user"
            exit 0
        fi
        echo ""
    fi

    # Execute release steps
    update_changelog
    commit_changelog
    merge_to_master
    create_and_push_tag
    create_github_release
    cleanup_and_return

    echo ""
    log_success "🎉 Release $VERSION completed successfully!"
    echo ""
    echo "Next steps:"
    echo "  1. 🔍 Review the draft release on GitHub"
    echo "  2. 🚀 Publish the release when ready"
    echo "  3. 📢 Announce the release to users"
    echo ""
    echo "GitHub Actions will automatically:"
    echo "  • 🔨 Build binaries for all platforms"
    echo "  • 📦 Create release archives"
    echo "  • 🔐 Generate checksums"
    echo "  • 📤 Upload assets to the draft release"
}

# Run main function
main "$@"
