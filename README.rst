.. Copyright 2018 Intel Corporation.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
        http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.

=================
OVN4NFVK8s Plugin/nfn-operator (Network Function Networking operator)
=================

Problem statement
-----------------

Networking applications are of three types - Management applications,
Control plane applications and data plane applications. Management
and control plane applications are similar to Enterprise applications,
but data plane applications different in following aspects:

- Multiple virtual network interfaces
- Multiple IP addresses
- SRIOV networking support
- Programmable virtual switch (for service function chaining, to tap
  the traffic for visibility etc..)

Kubernetes (Simply K8S) is the most popular container orchestrator.
K8S is supported by GCE, AZURE and AWS and will be supported by
Akraino Edge stack that enable edge clouds.

K8S has being enhanced to support VM workload types, this helps
cloud providers that need to migrate legacy workloads to microservices
architecture. Cloud providers may continue to support VM workload
types for security reasons and hence there is need for VIM that
support both VMs and containers. Since same K8S instance can
orchestrate both VM and container workload types, same compute nodes
can be leveraged for both VMs and containers. Telco and CSPs are
seeing similar need to deploy networking applications as containers.

Since, both VMs and container workloads are used for networking
applications, there would be need for

- Sharing the networks across VMs and containers.
- Sharing the volumes across VMs and containers.

**Network Function Virtualization Requirements**

NFV workloads can be,

- Management plane workloads
- Control plane work loads
- User plane (data plane workloads)
- User plane workloads normally have
- Multiple interfaces, Multiple subnets, Multiple virtual networks
- NFV workloads typically have its own management network.
- Some data plane workloads require SR-IOV NIC support for data
  interfaces and virtual NIC for other interfaces (for performance
  reasons)
- Need for multiple CNIs.
- NFV workloads require dynamic creation of virtual networks. Dynamic
  configuration of subnets.

ovn4nfvk8s Plugin
-----------------

This plugin addresses the below requirements, for networking
workloads as well typical application workloads
- Multi-interface support
- Multi-IP address support
- Dynamic creation of virtual networks
- Co-existing with SRIOV and other CNIs.
- Route management across virtual networks and external networks

**OVN Background**

OVN, the Open Virtual Network, is a system to support virtual network
abstraction. OVN complements the existing capabilities of OVS to add
native support for virtual network abstractions, such as virtual L2
and L3 overlays and security groups. Services such as DHCP are also
desirable features. Just like OVS, OVN’s design goal is to have a
production quality implementation that can operate at significant
scale.

**OVN4NFVK8s Plugin/ NFN-Operator development**

ovn-kubernetes_ plugin is part of OVN project which provides OVN
integration with Kubernetes but doesn't address the requirements
as given above. To meet those requirements like multiple interfaces,
IPs, dynamic creation of virtual networks, etc., OVN4NFVK8s plugin/
nfn-operator is created. It assumes that it will be used in
conjuction with Multus_or other similar CNI which allows for the
co-existance of multipleCNI plugins in runtime. This plugin assumes
that the first interface in a Pod is provided by some other Plugin/CNI
like Flannel or even OVN-Kubernetes. It is only responsible to add
multiple interfacesbased on the Pod annotations. The code is based on
ovn-kubernetes_.

NFN-Operator has following functionalities:
1) It watches pods for annotations (see below)
2) It is a CRD Controller for dynamic networks, provider networks and
   dynamic route creation.
3) It is a gRPC server for nfn-agent running on all nodes in the cluster

nfn-operator uses operator-sdk and controller runtime for watching for
pods and also other CR's. For creating dynamic logical networks Network
CRD controller creates OVN logical switch as defined in CR. For provider
network creation Provider Network CRD controller works with nfn-agent
running on nodes to create provider network. nfn-operator communicates
with nfn agent using gRPC. Currently only VLAN based provider networks
are supported.

.. note::

 This plugin is currently tested to work with Multus and Flannel
 providing the first network interface.

To meet the requirement of multiple interfaces and IP's per pod,
a Pod annotation like below is required when working with Multus:


.. code-block:: yaml


  annotations:
     k8s.v1.cni.cncf.io/networks: '[{ "name": "ovn-networkobj"}]'
     k8s.plugin.opnfv.org/nfn-network: '{ "type": "ovn4nfv", "interface": [
         { "name": <name of OVN Logical Switch>, "interfaceRequest": "eth1" },
         { "name":  <name of OVN Logical Switch>, "interfaceRequest": "eth2" }
  ]}'

Based on these annotations watcher service in OVN4NFVK8s plugin/
nfn-operator assumes logical switch is already present. Dynamic IP
addresses are assigned (static IP's also supported) and annotations
are updated.

When the Pod is initialized on a node, OVN4NFVK8s CNI creates multiple
interfaces and assigns IP addresses for the pod based on the annotations.

**Multus Configuration**
Multus CRD definition for OVN:

.. code-block:: yaml

  apiVersion: "k8s.cni.cncf.io/v1"
  kind: NetworkAttachmentDefinition
  metadata:
    name: ovn-networkobj
  spec:
    config: '{
        "cniVersion": "0.3.1",
        "name": "ovn4nfv-k8s-plugin",
        "type": "ovn4nfvk8s-cni"
      }'

Please refer to Multus_ for details about how this configuration is used

CNI configuration file for Multus with Flannel:

.. code-block:: yaml

 {
  "type": "multus",
  "name": "multus-cni",
  "cniVersion": "0.3.1",
  "kubeconfig": "/etc/kubernetes/admin.conf",
  "delegates": [
    {
      "type": "flannel",
      "cniVersion": "0.3.1",
      "masterplugin": true,
      "delegate": {
        "isDefaultGateway": false
      }
    }
  ]
 }

Refer Kubernetes_ documentation for the order in which CNI configurations
are applied.


**Build**

For building the project:

.. code-block:: bash

  cd ovn4nfv-k8s-plugin
  make


This will output two files nfn-operator, nfn-agent and ovn4nfvk8s-cni which are the plugin/
 operator, gRPC client and CNI binaries respectively.

ovn4nfvk8s-cni requires some configuration at start up.

Example configuration file (default location/etc/openvswitch/ovn4nfv_k8s.conf)

.. code-block:: yaml

  [logging]
  loglevel=5
  logfile=/var/log/openvswitch/ovn4k8s.log

  [cni]
  conf-dir=/etc/cni/net.d
  plugin=ovn4nfvk8s-cni

  [kubernetes]
  kubeconfig=/etc/kubernetes/admin.conf



**CRD Controllers**


nfn-operator includes controllers for 3 types of CRDs:

1) Network CRD - To create logical networks.

2) Provider Network CRD - To Create Provider networks. This works along with nfn-agent
   to create provider networks on nodes in cluster as needed.

3) Chaining operator - To provision routes in Pods as per CR definition.



**Network CR Example**


.. code-block:: yaml

  apiVersion: k8s.plugin.opnfv.org/v1alpha1
  kind: Network
  metadata:
    name: example-network
  spec:
    # Add fields here
    cniType: ovn4nfv
    ipv4Subnets:
    - subnet: 172.16.44.0/24
      name: subnet1
      gateway: 172.16.44.1/24
      excludeIps: 172.16.44.2 172.16.44.5..172.16.44.10



**Provider Network CR Example**


.. code-block:: yaml

  apiVersion: k8s.plugin.opnfv.org/v1alpha1
  kind: ProviderNetwork
  metadata:
    name: pnetwork
  spec:
    cniType: ovn4nfv
    ipv4Subnets:
    - subnet: 172.16.33.0/24
      name: subnet1
      excludeIps: 172.16.33.2 172.16.33.5..172.16.33.10
    providerNetType: VLAN
    vlan:
      vlanId: "100"
      providerInterfaceName: eth1
      logicalInterfaceName: eth1.100
      vlanNodeSelector: specific
      nodeLabelList:
      - kubernetes.io/hostname=testnode1

**Chaining CR Example**

TODO


**Figure**


.. code-block:: raw

    +-----------------+
    |                 |
    |                 |   Program OVN Switch
    |ovn4nfvk8s Plugin|                      +------------------+
    |                 +--------------------->|                  |
    |                 |                      | OVN Switch       |
    |                 |                      |                  |
    |                 |                      +------------------+
    +----+----------+-+
         ^          |
         |          |
         |On Event  |Annotate Pod
         |          |
         |          v
    +----+--------------+        +------------------+           +-----------+
    |                   |        |                  |           | Pod       |
    |  Kube API         +-------->  Kube Scheduler  +---------->|           |
    |                   |        |                  |           +--------+--+
    |                   |        +--------+---------+                    |
    +-------------------+                 |                              |
                                          |                              |
                                          |                              |Assign IP & MAC
                                 +--------v-----------+                  |
                                 |                    |                  |
                                 | ovn4nfvk8s-cni     |                  |
                                 |                    +------------------+
                                 +--------------------+




**References**

.. _ovn-kubernetes: https://github.com/openvswitch/ovn-kubernetes
.. _Multus: https://github.com/intel/multus-cni
.. _Kubernetes: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/

**Authors/Contributors**

Addepalli, Srinivasa R <srinivasa.r.addepalli@intel.com>
Sood, Ritu <ritu.sood@intel.com>

