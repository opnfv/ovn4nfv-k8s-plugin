#OVN4NFV Configuration Reference

ovn4nfv is not having any cni configuration. All the configuration are encapsulated within ovn4nfv-cni - `build/bin/entrypoint`

ovn4nfv-cni daemonset creates following cni configuration file `/etc/cni/net.d/00-network.conf` in each node
```
{
  "name": "ovn4nfv-k8s-plugin",
  "type": "ovn4nfvk8s-cni",
  "cniVersion": "0.3.1"
}
```
ovn4nfv cni-server use incluster-communication and cni shim uses the out-of-cluster
communication using the auto generated kubeconfig in each node.

#logging

Log is enabled by default and log file - `/var/log/openvswitch/ovn4k8s.log`

ovn log and openvswitch log can be find in the `/var/log/openvswitch` & `/var/log/ovn`
