FROM ubuntu:18.04

ARG HTTP_PROXY=${HTTP_PROXY}
ARG HTTPS_PROXY=${HTTPS_PROXY}

ENV http_proxy $HTTP_PROXY
ENV https_proxy $HTTPS_PROXY
ENV no_proxy $NO_PROXY

RUN apt-get update && apt-get install -y -qq apt-transport-https make curl net-tools iproute2 \
 && echo "deb https://packages.wand.net.nz bionic ovs-2.10" >  /etc/apt/sources.list.d/ovs.list \
 && curl https://packages.wand.net.nz/keyring.gpg -o /etc/apt/trusted.gpg.d/wand.gpg \
 && apt-get update && apt install -y -qq ovn-common openvswitch-common openvswitch-switch

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
