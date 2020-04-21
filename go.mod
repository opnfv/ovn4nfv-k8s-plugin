module ovn4nfv-k8s-plugin

require (
	dmitri.shuralyov.com/gpu/mtl v0.0.0-20191203043605-d42048ed14fd // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/ajstarks/svgo v0.0.0-20191124160048-bd5c74aaa11c // indirect
	github.com/containernetworking/cni v0.7.1
	github.com/containernetworking/plugins v0.8.1
	github.com/coreos/go-iptables v0.4.2 // indirect
	github.com/creack/pty v1.1.9 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/emicklei/go-restful v2.11.1+incompatible // indirect
	github.com/fogleman/gg v1.3.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.5
	github.com/go-openapi/swag v0.19.6 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/golang/groupcache v0.0.0-20191227052852-215e87163ea7 // indirect
	github.com/golang/protobuf v1.3.5
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/gophercloud/gophercloud v0.2.0 // indirect
	github.com/gorilla/mux v1.7.4
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.8.5 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/jung-kurt/gofpdf v1.16.2 // indirect
	github.com/kr/pty v1.1.8 // indirect
	github.com/mitchellh/mapstructure v1.1.2
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/phpdave11/gofpdi v1.0.8 // indirect
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829 // indirect
	github.com/rogpeppe/go-charset v0.0.0-20190617161244-0dc95cdf6f31 // indirect
	github.com/safchain/ethtool v0.0.0-20190326074333-42ed695e3de8 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.5
	github.com/sqs/goreturns v0.0.0-20181028201513-538ac6014518 // indirect
	github.com/urfave/cli v1.20.0
	github.com/vishvananda/netlink v1.0.0
	github.com/vishvananda/netns v0.0.0-20190625233234-7109fa855b0f // indirect
	github.com/zmb3/gogetdoc v0.0.0-20190228002656-b37376c5da6a // indirect
	golang.org/x/crypto v0.0.0-20191227163750-53104e6ec876 // indirect
	golang.org/x/exp v0.0.0-20191227195350-da58074b4299 // indirect
	golang.org/x/image v0.0.0-20191214001246-9130b4cfad52 // indirect
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422 // indirect
	golang.org/x/mobile v0.0.0-20191210151939-1a1fef82734d // indirect
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e // indirect
	golang.org/x/tools v0.0.0-20191227053925-7b8e75db28f4 // indirect
	golang.org/x/tools/gopls v0.1.3 // indirect
	golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543 // indirect
	gonum.org/v1/gonum v0.6.2 // indirect
	gonum.org/v1/netlib v0.0.0-20191031114514-eccb95939662 // indirect
	gonum.org/v1/plot v0.0.0-20191107103940-ca91d9d40d0a // indirect
	google.golang.org/genproto v0.0.0-20200325114520-5b2d0af7952b // indirect
	google.golang.org/grpc v1.28.0
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
	honnef.co/go/tools v0.0.0-20190523083050-ea95bdfd59fc // indirect
	k8s.io/api v0.0.0-20190918155943-95b840bb6a1f
	k8s.io/apimachinery v0.17.0
	//k8s.io/apimachinery v0.0.0-20190612125636-6a5db36e93ad
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.17.0 // indirect
	k8s.io/gengo v0.0.0-20191120174120-e74f70b9b27e // indirect
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	k8s.io/utils v0.0.0-20190801114015-581e00157fb1
	sigs.k8s.io/controller-runtime v0.2.0-beta.4
	sigs.k8s.io/controller-tools v0.1.10
	sigs.k8s.io/structured-merge-diff v1.0.1 // indirect
)

// Pinned to kubernetes-1.13.4
replace (
	k8s.io/api => k8s.io/api v0.0.0-20190918155943-95b840bb6a1f
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190918161926-8f644eb6e783
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/controller-tools => sigs.k8s.io/controller-tools v0.2.4
)

go 1.13
