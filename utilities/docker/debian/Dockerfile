FROM ubuntu:18.04 as base

USER root

RUN apt-get update && apt-get install -y iproute2 curl software-properties-common setpriv dpkg-dev netcat

RUN mkdir -p /opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb
RUN bash -xc "\
pushd /opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/libopenvswitch-dev_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/libopenvswitch_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-common_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-datapath-dkms_2.12.0-1_all.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-datapath-source_2.12.0-1_all.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-dbg_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-ipsec_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-pki_2.12.0-1_all.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-switch_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-testcontroller_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-test_2.12.0-1_all.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/openvswitch-vtep_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/ovn-central_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/ovn-common_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/ovn-controller-vtep_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/ovn-docker_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/ovn-host_2.12.0-1_amd64.deb; \
curl --insecure --compressed -O -L https://github.com/akraino-icn/ovs/releases/download/v2.12.0/python-openvswitch_2.12.0-1_all.deb; \
dpkg-scanpackages . | gzip -c9  > Packages.gz; \
popd; \
"
RUN ls -lt /opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb
RUN echo "deb [trusted=yes] file:///opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb ./" | tee -a /etc/apt/sources.list > /dev/null
RUN apt-get update
RUN apt-get install -y openvswitch-switch=2.12.0-1 openvswitch-common=2.12.0-1 ovn-central=2.12.0-1 ovn-common=2.12.0-1 ovn-host=2.12.0-1
RUN mkdir -p /var/run/openvswitch && \
    mkdir -p /var/run/ovn

COPY ovn4nfv-k8s.sh /usr/local/bin/ovn4nfv-k8s

ENTRYPOINT ["ovn4nfv-k8s"]
