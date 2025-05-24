#!/usr/bin/env bash

# Color definitions - shared across all scripts
GRAY='\033[90m'
GREEN='\033[32m'
YELLOW='\033[33m'
RED='\033[31m'
BLUE='\033[34m'
RESET='\033[0m'

# Alternative name for compatibility
NC="$RESET"  # No Color (used in some scripts)

# Basic logging functions
log() { 
    echo -e "${GRAY}git-undo:${RESET} $1"
}

log_info() {
    echo -e "${BLUE}[INFO]${RESET} $*"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${RESET} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${RESET} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${RESET} $*"
} 