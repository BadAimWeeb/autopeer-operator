#!/usr/bin/env sh
set -euo pipefail

: "${BIRD_PEERS_DIR:=/etc/bird/peers}"
: "${WIREGUARD_DIR:=/etc/wireguard}"

if [ "$TASK" = "peer" ]; then
    echo "Adding peer..."

    echo "$BIRD_CONF" > "$NODE_ROOT$BIRD_PEERS_DIR/$ASN.conf"
    echo "$WG_CONF" > "$NODE_ROOT$WIREGUARD_DIR/wgbgp$ASN.conf"

    nsenter -m/$NODE_ROOT/proc/1/ns/mnt -- systemctl enable --now wg-quick@wgbgp$ASN
    nsenter -m/$NODE_ROOT/proc/1/ns/mnt -- birdc configure

    echo "Peer added successfully."
elif [ "$TASK" = "rmpeer" ]; then
    echo "Removing peer..."

    rm -f "$NODE_ROOT$BIRD_PEERS_DIR/$ASN.conf"

    nsenter -m/$NODE_ROOT/proc/1/ns/mnt -- systemctl disable --now wg-quick@wgbgp$ASN || true
    # in case systemd fails to stop the interface, try manually bringing it down
    nsenter -m/$NODE_ROOT/proc/1/ns/mnt -- wg-quick down wgbgp$ASN || true
    rm -f "$NODE_ROOT$WIREGUARD_DIR/wgbgp$ASN.conf"

    nsenter -m/$NODE_ROOT/proc/1/ns/mnt -- birdc configure

    echo "Peer removed successfully."
else 
    echo "Unknown task: $TASK"
    exit 1
fi
