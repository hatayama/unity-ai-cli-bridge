#!/bin/sh

set -eu

DEFAULT_GO_CACHE="/tmp/unity-ai-cli-bridge-go-cache"
DEFAULT_GOLANGCI_LINT_CACHE="/tmp/unity-ai-cli-bridge-golangci-lint-cache"
GOLANGCI_VERSION_FILE=".golangci-lint-version"
GOVULNCHECK_VERSION_FILE=".govulncheck-version"

project_root() {
    CDPATH= cd -- "$(dirname -- "$0")/.." && pwd
}

require_command() {
    command_name="$1"
    if command -v "$command_name" >/dev/null 2>&1; then
        return 0
    fi

    printf '%s\n' "missing required command: $command_name" >&2
    return 1
}

go_cache_dir() {
    printf '%s\n' "${GOCACHE:-$DEFAULT_GO_CACHE}"
}

golangci_lint_cache_dir() {
    printf '%s\n' "${GOLANGCI_LINT_CACHE:-$DEFAULT_GOLANGCI_LINT_CACHE}"
}

go_files() {
    git ls-files '*.go'
}

run_with_go_cache() {
    GOCACHE="$(go_cache_dir)" "$@"
}

format_go_files() {
    files="$(go_files)"
    if [ -z "$files" ]; then
        return 0
    fi

    gofmt -w $files
}

check_go_format() {
    files="$(go_files)"
    if [ -z "$files" ]; then
        return 0
    fi

    unformatted_files="$(gofmt -l $files)"
    if [ -z "$unformatted_files" ]; then
        return 0
    fi

    printf '%s\n' "gofmt is required for the following files:" >&2
    printf '%s\n' "$unformatted_files" >&2
    return 1
}

print_install_help() {
    printf '%s\n' "Install golangci-lint with Homebrew or the official binary installer:" >&2
    printf '%s\n' "  https://golangci-lint.run/docs/welcome/install/local/" >&2
    printf '%s\n' "Suggested version file: $GOLANGCI_VERSION_FILE ($(cat "$GOLANGCI_VERSION_FILE"))" >&2
    printf '%s\n' "Install govulncheck with:" >&2
    printf '%s\n' "  go install golang.org/x/vuln/cmd/govulncheck@$(cat "$GOVULNCHECK_VERSION_FILE")" >&2
}

run_tests() {
    require_command go
    run_with_go_cache go test ./...
}

run_vet() {
    require_command go
    run_with_go_cache go vet ./...
}

run_lint() {
    require_command golangci-lint || {
        print_install_help
        return 1
    }

    GOCACHE="$(go_cache_dir)" GOLANGCI_LINT_CACHE="$(golangci_lint_cache_dir)" golangci-lint run ./...
}

run_vulncheck() {
    require_command govulncheck || {
        print_install_help
        return 1
    }

    GOCACHE="$(go_cache_dir)" govulncheck ./...
}

run_doctor() {
    missing_tools=0

    require_command go || missing_tools=1
    if ! command -v golangci-lint >/dev/null 2>&1; then
        printf '%s\n' "missing required command: golangci-lint" >&2
        missing_tools=1
    fi

    if ! command -v govulncheck >/dev/null 2>&1; then
        printf '%s\n' "missing required command: govulncheck" >&2
        missing_tools=1
    fi

    if [ "$missing_tools" -ne 0 ]; then
        print_install_help
        return 1
    fi

    printf '%s\n' "Go guardrail tools are available."
}

run_all_checks() {
    check_go_format
    run_tests
    run_vet
    run_lint
    run_vulncheck
}

main() {
    command_name="${1:-check}"

    cd "$(project_root)"

    case "$command_name" in
        fmt)
            format_go_files
            ;;
        fmt-check)
            check_go_format
            ;;
        test)
            run_tests
            ;;
        vet)
            run_vet
            ;;
        lint)
            run_lint
            ;;
        vuln)
            run_vulncheck
            ;;
        doctor)
            run_doctor
            ;;
        check)
            run_all_checks
            ;;
        *)
            printf '%s\n' "Usage: sh scripts/go-guardrails.sh [fmt|fmt-check|test|vet|lint|vuln|doctor|check]" >&2
            return 1
            ;;
    esac
}

main "$@"
