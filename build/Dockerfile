FROM ubuntu:18.04

ARG HTTP_PROXY=${HTTP_PROXY}
ARG HTTPS_PROXY=${HTTPS_PROXY}

ENV http_proxy $HTTP_PROXY
ENV https_proxy $HTTPS_PROXY
ENV no_proxy $NO_PROXY

RUN apt-get update && apt-get install -y -qq apt-transport-https make curl net-tools iproute2 \
    wget software-properties-common setpriv dpkg-dev netcat jq

RUN mkdir -p /opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb
RUN bash -xc "\
pushd /opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb; \
wget -q -nv -O- https://api.github.com/repos/akraino-icn/ovs/releases/tags/v2.12.0 2>/dev/null | jq -r '.assets[] | select(.browser_download_url | contains("\""deb"\"")) | .browser_download_url' | wget -i -; \
dpkg-scanpackages . | gzip -c9  > Packages.gz; \
popd; \
"
RUN echo "deb [trusted=yes] file:///opt/ovn4nfv-k8s-plugin/dist/ubuntu/deb ./" | tee -a /etc/apt/sources.list > /dev/null
RUN apt-get update && apt-get install -y -qq ovn-common=2.12.0-1 openvswitch-common=2.12.0-1 openvswitch-switch=2.12.0-1

ENV GOLANG_VERSION 1.14.1
RUN curl -sSL https://storage.googleapis.com/golang/go$GOLANG_VERSION.linux-amd64.tar.gz \
        | tar -v -C /usr/local -xz

ENV PATH /usr/local/go/bin:$PATH
RUN mkdir -p /go/src /go/bin && chmod -R 777 /go
ENV GOROOT /usr/local/go
ENV GOPATH /go
ENV PATH /go/bin:$PATH

WORKDIR /go/src/github.com/opnfv/ovn4nfv-k8s-plugin
COPY . .
RUN make all

ENV OPERATOR=/usr/local/bin/nfn-operator \
    AGENT=/usr/local/bin/nfn-agent \
    USER_UID=1001 \
    USER_NAME=nfn-operator

RUN cp -r build/bin/* /usr/local/bin/
ENTRYPOINT ["entrypoint"]
