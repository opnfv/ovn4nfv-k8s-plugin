FROM centos:8 as base

USER root
RUN yum update -y && yum install -y iproute curl nc ipset iptables jq wget unbound unbound-devel

RUN mkdir -p /opt/ovn4nfv-k8s-plugin/ovs/rpm/rpmbuild/RPMS/x86_64
RUN bash -xc "\
pushd /opt/ovn4nfv-k8s-plugin/ovs/rpm/rpmbuild/RPMS/x86_64; \
wget -q -nv -O- https://api.github.com/repos/akraino-icn/ovs/releases/tags/v2.14.0 2>/dev/null | jq -r '.assets[] | select(.browser_download_url | contains("\""rpm"\"")) | .browser_download_url' | wget -i -; \
popd; \
"
RUN rpm -ivh --nodeps /opt/ovn4nfv-k8s-plugin/ovs/rpm/rpmbuild/RPMS/x86_64/*.rpm

RUN mkdir -p /opt/ovn4nfv-k8s-plugin/ovn/rpm/rpmbuild/RPMS/x86_64
RUN bash -xc "\
pushd /opt/ovn4nfv-k8s-plugin/ovn/rpm/rpmbuild/RPMS/x86_64; \
wget -q -nv -O- https://api.github.com/repos/akraino-icn/ovn/releases/tags/v20.06.0 2>/dev/null | jq -r '.assets[] | select(.browser_download_url | contains("\""rpm"\"")) | .browser_download_url' | wget -i -; \
popd; \
"
RUN rpm -ivh --nodeps /opt/ovn4nfv-k8s-plugin/ovn/rpm/rpmbuild/RPMS/x86_64/*.rpm

RUN mkdir -p /var/run/openvswitch && \
    mkdir -p /var/run/ovn

WORKDIR /opt/ovn4nfv-k8s-plugin/utilities/docker/
COPY ./ ./
RUN cp /opt/ovn4nfv-k8s-plugin/utilities/docker/ovn4nfv-k8s.sh /usr/local/bin/ovn4nfv-k8s
RUN echo $PATH
ENTRYPOINT ["ovn4nfv-k8s"]
