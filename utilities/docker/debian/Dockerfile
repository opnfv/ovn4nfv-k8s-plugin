FROM ubuntu:18.04 as base

USER root

RUN apt-get update && apt-get install -y iproute2 curl software-properties-common setpriv dpkg-dev netcat jq wget

RUN mkdir -p /opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb
RUN bash -xc "\
pushd /opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb; \
wget -q -nv -O- https://api.github.com/repos/akraino-icn/ovs/releases/tags/v2.12.0 2>/dev/null | jq -r '.assets[] | select(.browser_download_url | contains("\""deb"\"")) | .browser_download_url' | wget -i -; \
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
