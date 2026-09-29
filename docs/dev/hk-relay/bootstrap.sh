#!/bin/bash
# hk-relay first-boot: hostname, BBR, official Caddy, sshd, ufw.
# Idempotent. Run as root. Does not install Docker or Sub2API.
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

CADDYFILE_SRC="${1:-/tmp/hk-relay.Caddyfile}"
if [ ! -f "$CADDYFILE_SRC" ]; then
	echo "missing Caddyfile at $CADDYFILE_SRC" >&2
	exit 1
fi

echo "== hostname =="
hostnamectl set-hostname hk-relay
if grep -q '^preserve_hostname:' /etc/cloud/cloud.cfg 2>/dev/null; then
	sed -i 's/^preserve_hostname:.*/preserve_hostname: true/' /etc/cloud/cloud.cfg
else
	printf '\npreserve_hostname: true\n' >> /etc/cloud/cloud.cfg
fi
if [ -f /etc/hosts ]; then
	sed -i 's/vm20403-cur20297/hk-relay/g' /etc/hosts
fi
if [ -f /etc/cloud/templates/hosts.debian.tmpl ]; then
	sed -i 's/vm20403-cur20297/hk-relay/g' /etc/cloud/templates/hosts.debian.tmpl
fi

echo "== BBR =="
cat > /etc/sysctl.d/99-bbr.conf << 'EOF'
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
EOF
sysctl --system >/dev/null

echo "== packages =="
apt-get update -qq
apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl gnupg ca-certificates ufw

echo "== caddy repo =="
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null
chmod o+r /usr/share/keyrings/caddy-stable-archive-keyring.gpg /etc/apt/sources.list.d/caddy-stable.list
apt-get update -qq
apt-get install -y caddy

echo "== caddyfile =="
install -d -o caddy -g caddy -m 0750 /var/log/caddy
install -m 0644 "$CADDYFILE_SRC" /etc/caddy/Caddyfile
caddy fmt --overwrite /etc/caddy/Caddyfile
caddy validate --config /etc/caddy/Caddyfile
# validate runs as root and may create the log file as root:root 0600
chown -R caddy:caddy /var/log/caddy
chmod 0750 /var/log/caddy
touch /var/log/caddy/hk-relay.log
chown caddy:caddy /var/log/caddy/hk-relay.log
chmod 0640 /var/log/caddy/hk-relay.log
systemctl enable caddy
systemctl restart caddy
systemctl --no-pager --full status caddy | head -20 || true

echo "== sshd =="
cat > /etc/ssh/sshd_config.d/99-hk-relay.conf << 'EOF'
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
EOF
sshd -t
systemctl reload ssh

echo "== ufw =="
ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
ufw status verbose

echo "== done =="
hostname
sysctl net.ipv4.tcp_congestion_control net.core.default_qdisc
caddy version
ss -lnt
systemctl is-active caddy
