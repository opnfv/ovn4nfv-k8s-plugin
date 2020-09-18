#!/bin/bash
# SPDX-license-identifier: Apache-2.0
##############################################################################
# Copyright (c) 2018
# All rights reserved. This program and the accompanying materials
# are made available under the terms of the Apache License, Version 2.0
# which accompanies this distribution, and is available at
# http://www.apache.org/licenses/LICENSE-2.0
##############################################################################

set -o nounset
set -o pipefail

# usage() - Prints the usage of the program
function usage {
    cat <<EOF
usage: $0 [-v volumes]
Optional Argument:
    -v List of key pair values for volumes and mount points ( e. g. sda=/var/lib/docker/,sdb=/var/lib/libvirt/ )
EOF
}

# mount_external_partition() - Create partition and mount the external volume
function mount_external_partition {
    local dev_name="/dev/$1"
    local mount_dir=$2

    sfdisk $dev_name --no-reread << EOF
;
EOF
    mkfs -t ext4 ${dev_name}1
    mkdir -p $mount_dir
    mount ${dev_name}1 $mount_dir
    echo "${dev_name}1 $mount_dir           ext4    errors=remount-ro,noatime,barrier=0 0       1" >> /etc/fstab
}

while getopts "h?v:" opt; do
    case $opt in
        v)
            dict_volumes="$OPTARG"
            ;;
        h|\?)
            usage
            exit
            ;;
    esac
done

swapoff -a
if [[ -n "${dict_volumes+x}" ]]; then
    for kv in ${dict_volumes//,/ } ;do
        mount_external_partition ${kv%=*} ${kv#*=}
    done
fi

vendor_id=$(lscpu|grep "Vendor ID")
if [[ $vendor_id == *GenuineIntel* ]]; then
    kvm_ok=$(cat /sys/module/kvm_intel/parameters/nested)
    if [[ $kvm_ok == 'N' ]]; then
        echo "Enable Intel Nested-Virtualization"
        rmmod kvm-intel
        echo 'options kvm-intel nested=y' >> /etc/modprobe.d/dist.conf
        modprobe kvm-intel
        echo kvm-intel >> /etc/modules
    fi
else
    kvm_ok=$(cat /sys/module/kvm_amd/parameters/nested)
    if [[ $kvm_ok == '0' ]]; then
        echo "Enable AMD Nested-Virtualization"
        rmmod kvm-amd
        sh -c "echo 'options kvm-amd nested=1' >> /etc/modprobe.d/dist.conf"
        modprobe kvm-amd
        echo kvm-amd >> /etc/modules
    fi
fi
modprobe vhost_net
echo vhost_net >> /etc/modules
source /etc/os-release || source /usr/lib/os-release
case ${ID,,} in
    *suse)
    ;;
    ubuntu|debian)
        apt-get install -y cpu-checker
        kvm-ok
    ;;
    rhel|centos|fedora)
    ;;
esac
