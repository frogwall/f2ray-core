#!/usr/bin/env bash

# F2Ray installation script
# Original source is located at github.com/frogwall/f2ray-core/release/install-release.sh

# If not specify, default meaning of return value:
# 0: Success
# 1: System error
# 2: Application error
# 3: Network error

#######color code########
RED="31m"      # Error message
YELLOW="33m"   # Warning message
GREEN="32m"    # Success message
colorEcho(){
    echo -e "\033[${1}${@:2}\033[0m" 1>& 2
}

colorEcho ${YELLOW} "F2Ray Installation Script"
colorEcho ${YELLOW} "Please use the user-package.sh script to build release packages."
colorEcho ${GREEN} "For manual installation, run: go build -o f2ray ./main"
exit 0
