#!/bin/sh
# Собирает trunkline из этого чекаута и кладёт в свою папку — не в
# $(go env GOBIN)/GOPATH/bin, чтобы не зависеть от того, настроен ли у
# пользователя Go-специфичный PATH. Дальше интерактивно предлагает добавить
# эту папку в PATH текущего логин-шелла.
#
# Переопределить папку установки: TRUNKLINE_INSTALL_DIR=/custom/path sh scripts/install.sh
#
# Обратное действие: scripts/uninstall.sh (или `make uninstall`).
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALL_DIR="${TRUNKLINE_INSTALL_DIR:-$HOME/.trunkline/bin}"
BIN_NAME="trunkline"
MARK_BEGIN="# >>> trunkline installer >>>"
MARK_END="# <<< trunkline installer <<<"

mkdir -p "$INSTALL_DIR"

echo "Building $BIN_NAME into $INSTALL_DIR ..."
( cd "$ROOT_DIR" && go build -trimpath -o "$INSTALL_DIR/$BIN_NAME" ./cmd/trunkline )

VERSION="$(cat "$ROOT_DIR/VERSION" 2>/dev/null || echo unknown)"
echo "Installed $BIN_NAME $VERSION -> $INSTALL_DIR/$BIN_NAME"

case ":$PATH:" in
    *":$INSTALL_DIR:"*)
        echo "$INSTALL_DIR is already on PATH — nothing else to do."
        exit 0
        ;;
esac

# Определяем rc-файл по текущему логин-шеллу пользователя.
shell_name="$(basename "${SHELL:-sh}")"
case "$shell_name" in
    zsh)
        rc_file="$HOME/.zshrc"
        export_line="export PATH=\"$INSTALL_DIR:\$PATH\""
        ;;
    fish)
        rc_file="$HOME/.config/fish/config.fish"
        export_line="set -gx PATH $INSTALL_DIR \$PATH"
        ;;
    bash)
        if [ "$(uname)" = "Darwin" ]; then
            rc_file="$HOME/.bash_profile"
        else
            rc_file="$HOME/.bashrc"
        fi
        export_line="export PATH=\"$INSTALL_DIR:\$PATH\""
        ;;
    *)
        rc_file="$HOME/.profile"
        export_line="export PATH=\"$INSTALL_DIR:\$PATH\""
        ;;
esac

echo ""
echo "$INSTALL_DIR is not on your PATH."

# Неинтерактивный запуск (curl | sh, CI, pipe) — читать ответ не у кого,
# просто печатаем команду и выходим.
if [ ! -t 0 ]; then
    echo "Add it manually, e.g.:"
    echo "  $export_line"
    exit 0
fi

printf 'Add %s to PATH via %s? [Y/n] ' "$INSTALL_DIR" "$rc_file"
read -r answer

case "$answer" in
    [nN]*)
        echo "Skipped. Add it yourself later, or re-run this script."
        ;;
    *)
        mkdir -p "$(dirname "$rc_file")"
        if [ -f "$rc_file" ] && grep -qF "$MARK_BEGIN" "$rc_file" 2>/dev/null; then
            echo "$rc_file already has a trunkline PATH block — leaving it as is."
        else
            {
                echo ""
                echo "$MARK_BEGIN"
                echo "$export_line"
                echo "$MARK_END"
            } >> "$rc_file"
            echo "Added to $rc_file. Restart your shell or run: . $rc_file"
        fi
        ;;
esac
