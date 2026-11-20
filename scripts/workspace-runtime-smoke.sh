#!/usr/bin/env bash
set -euo pipefail

usage() {
	printf 'Usage: bash scripts/workspace-runtime-smoke.sh [--require-rootless] IMAGE@sha256:<digest>\n'
}

fail() {
	printf 'workspace-runtime-smoke: %s\n' "$*" >&2
	exit 1
}

require_rootless=false
case "${1:-}" in
	-h|--help) usage; exit 0 ;;
	--require-rootless) require_rootless=true; shift ;;
esac

if [[ $# -ne 1 || ! $1 =~ ^[[:alnum:]][[:alnum:]._:/-]*@sha256:[0-9a-f]{64}$ ]]; then
	usage >&2
	exit 2
fi
image=$1

engine=$(docker info --format '{{.OSType}} {{.CgroupVersion}} {{json .SecurityOptions}}')
[[ $engine == 'linux 2 '* ]] || fail "expected a Linux Docker engine with cgroup v2: $engine"

if [[ $engine == *'"name=rootless"'* ]]; then
	printf 'Docker engine: rootless\n'
elif $require_rootless; then
	fail 'Docker engine is not rootless'
else
	printf 'Docker engine: rootful (rootless verification skipped)\n'
fi

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/workspace-smoke.XXXXXXXX")
probe="${tmp_dir##*/}"
container="$probe-run"
volume="$probe-state"
cleanup() {
	docker rm -f "$container" >/dev/null 2>&1 || true
	docker volume rm "$volume" >/dev/null 2>&1 || true
	rmdir "$tmp_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker volume create "$volume" >/dev/null
runtime_args=(
	--init
	--user 65532:65532
	--network none
	--read-only
	--cap-drop ALL
	--security-opt no-new-privileges
	--cpus 0.25
	--memory 64m
	--pids-limit 32
	--stop-signal SIGTERM
	--mount "type=volume,source=$volume,target=/tmp"
	--entrypoint /bin/sh
)

docker run --detach --name "$container" "${runtime_args[@]}" "$image" -eu -c '
	record() {
		printf "%s\n" "$1" >> /tmp/run.log
		printf "%s\n" "$1"
	}
	trap "record cancelled; exit 143" TERM
	tick=0
	record "$tick"
	while :; do
		sleep 1
		tick=$((tick + 1))
		record "$tick"
	done
' >/dev/null

docker exec "$container" /bin/sh -eu -c '
	test "$(id -u)" = 65532
	for flags_file in /sys/class/net/*/flags; do
		[ "$flags_file" = /sys/class/net/lo/flags ] && continue
		flags=$(cat "$flags_file")
		test "$((flags & 1))" = 0
	done
	awk '"'"'
		$1 == "CapEff:" { no_caps = ($2 == "0000000000000000") }
		$1 == "NoNewPrivs:" { no_new_privs = ($2 == 1) }
		$1 == "Seccomp:" { seccomp = ($2 == 2) }
		END { exit !(no_caps && no_new_privs && seccomp) }
	'"'"' /proc/self/status
	test "$(cat /sys/fs/cgroup/memory.max)" = 67108864
	test "$(cat /sys/fs/cgroup/pids.max)" = 32
	awk '"'"'NF == 2 && $1 ~ /^[0-9]+$/ && $2 > 0 && $1 * 4 == $2 { matched = 1 }
		END { exit !matched }'"'"' /sys/fs/cgroup/cpu.max
' || fail 'container isolation or cgroup settings did not match'

read_tick() {
	docker exec "$container" /bin/sh -ec 'tail -n 1 /tmp/run.log'
}

for ((attempt = 0; attempt < 10; attempt++)); do
	first_tick=$(read_tick 2>/dev/null) || true
	[[ $first_tick =~ ^[0-9]+$ ]] && break
	sleep 1
done
[[ $first_tick =~ ^[0-9]+$ ]] || fail 'fixture did not write its first tick'

# The start and status clients have exited. A new client must observe progress.
advanced=false
for ((attempt = 0; attempt < 10; attempt++)); do
	sleep 1
	current_tick=$(read_tick)
	if [[ $current_tick =~ ^[0-9]+$ ]] && (( current_tick > first_tick )); then
		advanced=true
		break
	fi
done
$advanced || fail 'fixture did not advance after the client disconnected'

docker stop --time 5 "$container" >/dev/null
[[ $(docker inspect -f '{{.State.ExitCode}}' "$container") == 143 ]] || fail 'expected SIGTERM exit status'
logs=$(docker logs "$container")
[[ ${logs##*$'\n'} == cancelled ]] || fail 'fixture did not record cancellation'

docker rm "$container" >/dev/null

# A fresh container can read the state after the original container is gone.
persisted=$(docker run --rm --name "$container" "${runtime_args[@]}" "$image" -ec 'cat /tmp/run.log')
[[ $persisted == "$logs" ]] || fail 'reopened volume did not contain the complete run log'
printf 'PASS: isolated execution, detached progress, cancellation and persistent log recovery\n'
