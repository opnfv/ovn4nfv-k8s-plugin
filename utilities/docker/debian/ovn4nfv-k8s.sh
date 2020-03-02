#!/usr/bin/env bash
OVS_RUNDIR=/var/run/openvswitch
OVS_LOGDIR=/var/log/openvswitch

DB_NB_ADDR=${DB_NB_ADDR:-::}
DB_NB_PORT=${DB_NB_PORT:-6641}
DB_SB_ADDR=${DB_SB_ADDR:-::}
DB_SB_PORT=${DB_SB_PORT:-6642}
cmd=${1:-""}

if [[ -f /usr/bin/ovn-appctl ]] ; then
    # ovn-appctl is present. Use new ovn run dir path.
    OVN_RUNDIR=/var/run/ovn
    OVNCTL_PATH=/usr/share/ovn/scripts/ovn-ctl
    OVN_LOGDIR=/var/log/ovn
    OVN_ETCDIR=/etc/ovn
else
    # ovn-appctl is not present. Use openvswitch run dir path.
    OVN_RUNDIR=/var/run/openvswitch
    OVNCTL_PATH=/usr/share/openvswitch/scripts/ovn-ctl
    OVN_LOGDIR=/var/log/openvswitch
    OVN_ETCDIR=/etc/openvswitch
fi

check_ovn_control_plane() {
    /usr/share/ovn/scripts/ovn-ctl status_northd
    /usr/share/ovn/scripts/ovn-ctl status_ovnnb
    /usr/share/ovn/scripts/ovn-ctl status_ovnsb
}

check_ovn_controller() {
    /usr/share/ovn/scripts/ovn-ctl status_controller
}

# wait for ovn-sb ready
wait_ovn_sb() {
    if [[ -z "${OVN_SB_TCP_SERVICE_HOST}" ]]; then
        echo "env OVN_SB_SERVICE_HOST not exists"
        exit 1
    fi
    if [[ -z "${OVN_SB_TCP_SERVICE_PORT}" ]]; then
        echo "env OVN_SB_SERVICE_PORT not exists"
        exit 1
    fi
    while ! nc -z "${OVN_SB_TCP_SERVICE_HOST}" "${OVN_SB_TCP_SERVICE_PORT}" </dev/null;
    do
        echo "sleep 10 seconds, waiting for ovn-sb ${OVN_SB_TCP_SERVICE_HOST}:${OVN_SB_TCP_SERVICE_PORT} ready "
        sleep 10;
    done
}

start_ovs_vswitch() {
    wait_ovn_sb
    function quit {
	/usr/share/openvswitch/scripts/ovs-ctl stop
	/usr/share/openvswitch/scripts/ovn-ctl stop_controller
	exit 0
    }
    trap quit EXIT
    /usr/share/openvswitch/scripts/ovs-ctl restart --no-ovs-vswitchd --system-id=random
    # Restrict the number of pthreads ovs-vswitchd creates to reduce the
    # amount of RSS it uses on hosts with many cores
    # https://bugzilla.redhat.com/show_bug.cgi?id=1571379
    # https://bugzilla.redhat.com/show_bug.cgi?id=1572797
    if [[ `nproc` -gt 12 ]]; then
        ovs-vsctl --no-wait set Open_vSwitch . other_config:n-revalidator-threads=4
        ovs-vsctl --no-wait set Open_vSwitch . other_config:n-handler-threads=10
    fi

    # Start ovsdb
    /usr/share/openvswitch/scripts/ovs-ctl restart --no-ovsdb-server  --system-id=random
    /usr/share/openvswitch/scripts/ovs-ctl --protocol=udp --dport=6081 enable-protocol
    
}

#cleanup_ovs_server() {
#}

#cleanup_ovs_controller() {
#}

function get_default_inteface_ipaddress {
    local _ip=$1
    local _default_interface=$(awk '$2 == 00000000 { print $1 }' /proc/net/route)
    local _ipv4address=$(ip addr show dev $_default_interface | awk '$1 == "inet" { sub("/.*", "", $2); print $2 }')
    eval $_ip="'$_ipv4address'"
}

start_ovn_control_plane() {
    function quit {
        /usr/share/openvswitch/scripts/ovn-ctl stop_northd
         exit 0
    }
    trap quit EXIT
    /usr/share/openvswitch/scripts/ovn-ctl restart_northd
    ovn-nbctl set-connection ptcp:"${DB_NB_PORT}":["${DB_NB_ADDR}"]
    ovn-nbctl set Connection . inactivity_probe=0
    ovn-sbctl set-connection ptcp:"${DB_SB_PORT}":["${DB_SB_ADDR}"]
    ovn-sbctl set Connection . inactivity_probe=0
    tail -f /var/log/openvswitch/ovn-northd.log
}

start_ovn_controller() {
    function quit {
	/usr/share/openvswitch/scripts/ovn-ctl stop_controller
	exit 0
    }
    trap quit EXIT
    wait_ovn_sb
    get_default_inteface_ipaddress node_ipv4_address
    /usr/share/openvswitch/scripts/ovn-ctl restart_controller
    # Set remote ovn-sb for ovn-controller to connect to
    ovs-vsctl set open . external-ids:ovn-remote=tcp:"${OVN_SB_TCP_SERVICE_HOST}":"${OVN_SB_TCP_SERVICE_PORT}"
    ovs-vsctl set open . external-ids:ovn-remote-probe-interval=10000
    ovs-vsctl set open . external-ids:ovn-openflow-probe-interval=180
    ovs-vsctl set open . external-ids:ovn-encap-type=geneve
    ovs-vsctl set open . external-ids:ovn-encap-ip=$node_ipv4_address
    tail -f /var/log/openvswitch/ovn-controller.log
}

set_nbclt() {
    wait_ovn_sb
    ovn-nbctl --db=tcp:["${OVN_NB_TCP_SERVICE_HOST}"]:"${OVN_NB_TCP_SERVICE_PORT}" --pidfile --detach --overwrite-pidfile
}

check_ovs_vswitch() {
    /usr/share/openvswitch/scripts/ovs-ctl status
}

case ${cmd} in
  "start_ovn_control_plane")
        start_ovn_control_plane
    ;;
  "check_ovn_control_plane")
        check_ovn_control_plane
    ;;
  "start_ovn_controller")
        start_ovs_vswitch
        set_nbclt
        start_ovn_controller 
    ;;
  "check_ovs_vswitch")
        check_ovs_vswitch
    ;;
  "check_ovn_controller")
        check_ovs_vswitch
        check_ovn_controller
    ;;
  "cleanup_ovs_controller")
        cleanup_ovs_controller
    ;;
  *)
    echo "invalid command ${cmd}"
    echo "valid commands: start-ovn-control-plane check_ovn_control_plane start-ovs-vswitch"
    exit 0
esac

exit 0
