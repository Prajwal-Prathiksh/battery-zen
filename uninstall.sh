#!/bin/bash
# Battery Zen Uninstallation Script

set -e

remove_data=true
if [[ "${1:-}" == "--keep-data" ]]; then
    remove_data=false
elif [[ $# -gt 0 ]]; then
    echo "Usage: $0 [--keep-data]"
    exit 2
elif [[ -t 0 ]]; then
    read -r -p "Remove Battery Zen configuration and logs? [Y/n] " response
    case "$response" in
        n|N|no|NO|No) remove_data=false ;;
    esac
fi

echo "Uninstalling Battery Zen..."
make uninstall
make clean
rm -f "$HOME/.local/share/applications/battery-zen.desktop"
rm -f "$HOME/.local/share/icons/battery-zen.png"

if [[ "$remove_data" == true ]]; then
    rm -rf "$HOME/.config/battery-zen"
    rm -rf "$HOME/.local/state/battery-zen"
    echo "Removed Battery Zen configuration and logs."
else
    echo "Preserved Battery Zen configuration and logs."
fi

echo "Battery Zen has been uninstalled."
