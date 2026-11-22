#!/usr/bin/env bash
# Casbin Gateway one-step install for Linux and macOS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.sh | bash
#
# This downloads the nightly build, which is an automated build of master and
# not an official release. Use it for testing and development only.
#
# Optional environment variables:
#   INSTALL_DIR   where the executable and its data live
#                 (default: $HOME/.local/share/casbin-gateway)
#   BIN_DIR       where the "casbin-gateway" command is placed
#                 (default: $HOME/.local/bin)
#   SERVER_ONLY   set to any value to install the server and CLI without desktop
#                 integration or starting or stopping a process
#   NO_START      set to any value to install without starting Gateway; one that
#                 was already running is stopped for the update and started again
#   NO_AUTOSTART  set to any value to skip the login-time startup entry
#   NO_SHORTCUT   set to any value to skip the macOS bundle or the desktop entry

set -euo pipefail

REPO="apache/casbin-gateway"
TAG="nightly"
BASENAME="casbin-gateway-nightly"

INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/share/casbin-gateway}"
BIN_DIR="${BIN_DIR:-${HOME}/.local/bin}"
SERVER_ONLY="${SERVER_ONLY:-}"
NO_START="${NO_START:-}"
NO_AUTOSTART="${NO_AUTOSTART:-}"
NO_SHORTCUT="${NO_SHORTCUT:-}"

info() { printf '%s\n' "$*"; }
die() { printf 'casbin-gateway: %s\n' "$*" >&2; exit 1; }

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

need_cmd curl
need_cmd tar

# ── pick the archive for this machine ─────────────────────────────────────────
os="$(uname -s)"
case "${os}" in
	Linux)  osName="linux" ;;
	Darwin) osName="darwin" ;;
	*) die "unsupported operating system \"${os}\", build from source instead: https://github.com/${REPO}" ;;
esac

# Only x86_64 archives are published. macOS runs them under Rosetta 2, so an
# Apple Silicon machine still gets a working install; on Linux arm64 there is
# nothing to fall back to.
arch="$(uname -m)"
case "${arch}" in
	x86_64|amd64) archName="x86_64" ;;
	aarch64|arm64)
		[[ "${osName}" == "darwin" ]] || die "unsupported architecture \"${arch}\", build from source instead: https://github.com/${REPO}"
		archName="x86_64"
		info "No arm64 build is published, installing the x86_64 one to run under Rosetta 2"
		;;
	*) die "unsupported architecture \"${arch}\", build from source instead: https://github.com/${REPO}" ;;
esac

archive="${BASENAME}-${osName}-${archName}.tar.gz"
url="https://github.com/${REPO}/releases/download/${TAG}/${archive}"

if [[ -n "${SERVER_ONLY}" ]]; then
	if [[ -e "${INSTALL_DIR}/casbin-gateway-desktop" || -L "${INSTALL_DIR}/casbin-gateway-desktop" ]]; then
		die "SERVER_ONLY needs an install directory without a desktop launcher. Use the desktop installer to update this installation, or choose a separate INSTALL_DIR."
	fi
elif [[ "${osName}" == "linux" ]]; then
	need_cmd ldd
fi

# ── download ──────────────────────────────────────────────────────────────────
tmpDir="$(mktemp -d)"
stage=""
cleanup() {
	rm -rf "${tmpDir}"
	if [[ -n "${stage}" ]]; then
		rm -rf "${stage}"
	fi
}
trap cleanup EXIT

info "Downloading ${url}"
curl -fsSL -o "${tmpDir}/${archive}" "${url}"

# ── install ───────────────────────────────────────────────────────────────────
info "Installing to ${INSTALL_DIR}"
mkdir -p "${INSTALL_DIR}" || die "cannot create ${INSTALL_DIR}, set INSTALL_DIR to a directory you can write to"

# Unpacking inside INSTALL_DIR keeps the final move on one filesystem, so it is
# a rename: an already running Gateway holds the old executable open, and
# writing over it in place would fail with "text file busy".
stage="$(mktemp -d "${INSTALL_DIR}/.install.XXXXXXXX")"
tar -xzf "${tmpDir}/${archive}" -C "${stage}" --strip-components=1

executables=(casbin-gateway)
if [[ -z "${SERVER_ONLY}" ]]; then
	executables+=(casbin-gateway-desktop)
fi
for file in "${executables[@]}" LICENSE NOTICE DISCLAIMER; do
	[[ -f "${stage}/${file}" ]] || die "the downloaded archive is missing ${file}"
done
for executable in "${executables[@]}"; do
	chmod 755 "${stage}/${executable}"
done

if [[ -z "${SERVER_ONLY}" && "${osName}" == "linux" ]]; then
	# ldd can exit successfully even when a shared library is missing.
	if ! dependencies="$(LC_ALL=C ldd "${stage}/casbin-gateway-desktop" 2>&1)" ||
		[[ "${dependencies}" == *"not found"* ]]; then
		printf '%s\n' "${dependencies}" >&2
		if [[ "${dependencies}" == *"libwebkit2gtk-4.1.so.0 => not found"* ]]; then
			printf '%s\n' 'Install WebKitGTK 4.1 (webkit2gtk-4.1 on Arch Linux, libwebkit2gtk-4.1-0 on Debian or Ubuntu where available).' >&2
		fi
		die "the Linux desktop launcher needs working shared libraries. Fix the dependencies above and rerun the installer. For a server-only install, use SERVER_ONLY=1 with a directory without a desktop launcher."
	fi
fi

# ── stop the Gateway that is running ──────────────────────────────────────────
# Without this an install over a running Gateway put the new executables in
# place and left the old process serving, so nothing appeared to change until
# that process was stopped by hand. The launcher from the archive is what stops
# it, because an installation old enough to have no "quit" command cannot stop
# itself. CASBIN_GATEWAY_HOME points that launcher at the installation rather
# than at the staging directory it runs from.
wasRunning=""
if [[ -z "${SERVER_ONLY}" && -e "${INSTALL_DIR}/casbin-gateway" ]] &&
	CASBIN_GATEWAY_HOME="${INSTALL_DIR}" "${stage}/casbin-gateway-desktop" quit; then
	wasRunning="yes"
	info "Stopped the running Gateway, and will start it again once it is updated"
fi

for executable in "${executables[@]}"; do
	mv -f "${stage}/${executable}" "${INSTALL_DIR}/${executable}"
done
for legalFile in LICENSE NOTICE DISCLAIMER; do
	mv -f "${stage}/${legalFile}" "${INSTALL_DIR}/${legalFile}"
done
# ── put a "casbin-gateway" command on PATH ────────────────────────────────────
# Gateway keeps its database, logs and temporary files in the working
# directory, so the command is a wrapper that always starts it in INSTALL_DIR.
# Without it, running "casbin-gateway" from somewhere else would quietly start
# a second, empty installation.
if [[ "${BIN_DIR}" == "${INSTALL_DIR}" ]]; then
	info "BIN_DIR is the install directory, so no wrapper was created"
else
	mkdir -p "${BIN_DIR}" || die "cannot create ${BIN_DIR}, set BIN_DIR to a directory you can write to"
	cat > "${BIN_DIR}/casbin-gateway" <<EOF
#!/usr/bin/env bash
# Written by the Casbin Gateway installer. Gateway reads and writes ./data,
# ./logs and ./tmp, so it always has to start in its own directory.
cd "${INSTALL_DIR}" || exit 1
exec "${INSTALL_DIR}/casbin-gateway" "\$@"
EOF
	chmod 755 "${BIN_DIR}/casbin-gateway"

	case ":${PATH}:" in
		*":${BIN_DIR}:"*) ;;
		*)
			shellRc=""
			case "${SHELL:-}" in
				*/zsh)  shellRc="${HOME}/.zshrc" ;;
				*/bash) shellRc="${HOME}/.bashrc" ;;
			esac

			if [[ -n "${shellRc}" ]]; then
				printf '\nexport PATH="%s:$PATH"\n' "${BIN_DIR}" >> "${shellRc}"
				info "Added ${BIN_DIR} to PATH in ${shellRc}, which takes effect in your next shell"
			else
				info "${BIN_DIR} is not on your PATH, add it to run Gateway by name"
			fi
			;;
	esac
fi

if [[ -n "${SERVER_ONLY}" ]]; then
	info "Casbin Gateway server is installed in ${INSTALL_DIR}."
	info "Use casbin-gateway start to start it, or stop and start it to run the updated version."
	exit 0
fi

# ── the desktop application ───────────────────────────────────────────────────
# A macOS bundle or a Linux desktop entry, created by the launcher so that an
# install and an archive unpacked by hand end up with the same one. Either
# points at the launcher rather than at the server: it is what shows the window,
# and it starts the server itself. Telling it "off" is what keeps a reinstall
# with NO_SHORTCUT from getting one back on the next start.
desktopExe="${INSTALL_DIR}/casbin-gateway-desktop"
appNote=""
shortcutState="on"
if [[ -n "${NO_SHORTCUT}" ]]; then
	shortcutState="off"
fi

if ! appNote="$("${desktopExe}" shortcut "${shortcutState}")"; then
	appNote=""
	info "Could not create the desktop application"
fi

# The macOS bundle holds its own copy of the launcher, and that copy is the one
# the login item and the first start should use: a launcher running from outside
# the bundle gets neither the Dock icon nor the name.
if [[ "${appNote}" == *.app ]]; then
	desktopExe="${appNote}/Contents/MacOS/casbin-gateway-desktop"
fi

# ── start at login ────────────────────────────────────────────────────────────
# The launcher owns this entry so that the tray's "Start at Login" checkbox and
# the installer are never out of step. Older installs started the server on its
# own; those entries would now bring up a second copy.
rm -f "${HOME}/Library/LaunchAgents/org.apache.casbin-gateway.plist"
if command -v systemctl >/dev/null 2>&1 && [[ -f "${HOME}/.config/systemd/user/casbin-gateway.service" ]]; then
	systemctl --user disable --now casbin-gateway.service >/dev/null 2>&1 || true
	rm -f "${HOME}/.config/systemd/user/casbin-gateway.service"
	systemctl --user daemon-reload >/dev/null 2>&1 || true
fi

autostartNote=""
if [[ -z "${NO_AUTOSTART}" ]]; then
	if "${desktopExe}" autostart on; then
		autostartNote="yes"
	fi
fi

info ""
info "Casbin Gateway is installed in ${INSTALL_DIR}"
info "Its database, logs and temporary files stay in that directory."
info "It serves this machine only, and signs you in there as admin without a password."
info "Closing its window leaves it running in the tray; quit it from there."
info "Without the window: \"casbin-gateway start\", \"casbin-gateway stop\", \"casbin-gateway status\"."
if [[ -n "${appNote}" ]]; then
	info "The application is ${appNote}"
fi
if [[ -n "${autostartNote}" ]]; then
	info "It will start when you log in; turn that off from the tray menu."
fi
info ""

# A Gateway that was running when this started is started again even with
# NO_START: the install is what stopped it, and leaving the machine without one
# is not what "install without starting" asked for.
if [[ -n "${NO_START}" && -z "${wasRunning}" ]]; then
	info "Start it from the Casbin Gateway application, or with: casbin-gateway start"
	exit 0
fi

# The launcher forks the server and the window, so installing does not occupy
# this terminal beyond the startup it waits for.
cd "${INSTALL_DIR}"
"${desktopExe}" >/dev/null 2>&1 &
disown 2>/dev/null || true
