!#/bin/bash

apt-get install apt-transport-https dpkg-dev -y

mkdir -p /opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb
pushd /opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/libopenvswitch-dev_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/libopenvswitch_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-common_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-datapath-dkms_2.12.0-1_all.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-datapath-source_2.12.0-1_all.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-dbg_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-ipsec_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-pki_2.12.0-1_all.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-switch_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-testcontroller_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-test_2.12.0-1_all.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-vtep_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/ovn-central_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/ovn-common_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/ovn-controller-vtep_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/ovn-docker_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/ovn-host_2.12.0-1_amd64.deb
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/python-openvswitch_2.12.0-1_all.deb
dpkg-scanpackages . | gzip -c9  > Packages.gz
popd

sudo apt-get install apt-transport-https
echo "deb [trusted=yes] file:///opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb ./" | tee -a /etc/apt/sources.list > /dev/null
cp /etc/apt/sources.list /etc/apt/sources.list~
sed -Ei 's/^# deb-src /deb-src /' /etc/apt/sources.list
apt-get update && apt-get build-dep dkms -y && apt-get install openvswitch-datapath-dkms=2.12.0-1 -y
