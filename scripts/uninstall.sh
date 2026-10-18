#!/bin/sh
# Обратное действие для scripts/install.sh: удаляет установленный бинарь и
# PATH-блок, который install.sh мог дописать в rc-файлы шелла.
#
# Если install.sh запускался с TRUNKLINE_INSTALL_DIR, передайте ту же
# переменную сюда, иначе бинарь не найдётся по умолчанному пути.
set -eu

INSTALL_DIR="${TRUNKLINE_INSTALL_DIR:-$HOME/.trunkline/bin}"
BIN_NAME="trunkline"
MARK_BEGIN="# >>> trunkline installer >>>"
MARK_END="# <<< trunkline installer <<<"

if [ -f "$INSTALL_DIR/$BIN_NAME" ]; then
    rm -f "$INSTALL_DIR/$BIN_NAME"
    echo "Removed $INSTALL_DIR/$BIN_NAME"
    rmdir "$INSTALL_DIR" 2>/dev/null || true
else
    echo "$INSTALL_DIR/$BIN_NAME not found — nothing to remove."
fi

for rc_file in "$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile" "$HOME/.config/fish/config.fish"; do
    [ -f "$rc_file" ] || continue
    if grep -qF "$MARK_BEGIN" "$rc_file" 2>/dev/null; then
        tmp="$(mktemp)"
        awk -v b="$MARK_BEGIN" -v e="$MARK_END" '
            $0 == b { skip = 1 }
            skip == 0 { print }
            $0 == e { skip = 0; next }
        ' "$rc_file" > "$tmp"
        mv "$tmp" "$rc_file"
        echo "Removed trunkline PATH block from $rc_file"
    fi
done

echo "Done. Restart your shell to pick up the PATH change."
