#!/bin/bash
#
# Brings up a VM in QEMU using the disk with device-name guest-vm.
# The guest VM will appear to GCE network infra like a native GCE VM (i.e. the
# metadata server will work, DHCP will work, etc).

set -o errexit
set -o pipefail

show_status() {
  $@ 2>&1 | sed -e 's/^/HostStatus: /' > /dev/ttyS1
  if [[ "${PIPESTATUS[0]}" != 0 ]]; then
    echo "HostFailed: see logs for errors" > /dev/ttyS1
  fi
}

i=0
errcode=0
while (( i < 10 )); do
  show_status sudo apt update && errcode=$? || errcode=$?
  if [[ $errcode != 0 ]]; then
    (( i++ ))
    continue
  fi
  show_status sudo apt install -y qemu-system-aarch64 && errcode=$? || errcode=$?
  if [[ $errcode != 0 ]]; then
    (( i++ ))
    continue
  fi
  break
done
if [[ $errcode != 0 ]]; then
  show_status echo "Failed to install QEMU"
  exit 1
fi
show_status echo "Creating new netns"
show_status sudo ip netns add vmnetwork
show_status sudo ip link set ens4 netns vmnetwork
cat <<EOF | show_status sudo ip netns exec vmnetwork bash
ip link set ens4 up
ip link add link ens4 name macvtap0 type macvtap mode passthru
ip link set macvtap0 up
echo "Running QEMU"
touch /tmp/kvm.monitor
qemu-system-aarch64 -nographic \
        -m 16G -smp 6 -M virt -cpu cortex-a57 \
        -bios /usr/share/qemu-efi-aarch64/QEMU_EFI.fd \
        -chardev pipe,id=control_pipe,path=/tmp/kvm.monitor -mon chardev=control_pipe \
        -serial /dev/ttyS2 \
        -device virtio-rng \
        -device virtio-scsi-pci,id=scsi \
        -device scsi-hd,drive=hd,bootindex=0 \
        -drive if=none,id=hd,file=/dev/disk/by-id/google-guest-vm,cache=unsafe,format=raw \
        -device scsi-hd,drive=cidata \
        -drive if=none,id=cidata,file=/dev/disk/by-id/google-cidata,cache=unsafe,format=raw \
        -device virtio-net,netdev=eth0,mac=\$(cat /sys/class/net/ens4/address) \
        -netdev tap,id=eth0,fd=3 \
        3<>/dev/tap\$(cat /sys/class/net/macvtap0/ifindex)
EOF
show_status echo "Done running QEMU, shutting down"
# `shutdown -h now` seems to hang sometimes. Nothing important is mounted, so
# completely kill everything.
sudo bash -c 'echo o > /proc/sysrq-trigger'
