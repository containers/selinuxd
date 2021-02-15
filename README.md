selinuxd
========

This a daemon that has the purpose of installing and removing policies as they are
laid in a specific directory. This directory is `/etc/selinux.d` by default.

The intent is to follow a infrastructure-as-code approach for installing SELinux
policies. With this, installing policies is a matter of persisting policy files
in a specific directory, which the daemon will immediately pick up and try to
install them.

Building
========

Golang 1.15 and GNU make are required. In Fedora 33, the installation is a matter of doing:

```
$ sudo dnf install golang make libsemanage-devel policycoreutils
```

With this, you can build the daemon's binary with `make build`, or simply
`make`. the binary will be persisted to the `bin/` directory.

Running
=======

Once you have built the binary, simply do:

```
$ sudo ./bin/selinuxdctl daemon
```

or

```
$ make run
```

Note that `sudo` is needed as it'll attempt to install SELinux policies, which
requires root. Also note that the `run` target will attempt to create
`/etc/selinux.d`.

This will:

* Listen for file changes in the `/etc/selinux.d` directory

  - When a file is added or modified, it'll attempt to install the policy

  - When a file is removed, it'll uninstall the policy

Testing (for demo purposes)
===========================

With the daemon running, do:

```
$ sudo cp tests/data/testport.cil /etc/selinux.d/
```

Notice that the policy will be installed in the system shortly:

```
$ sudo semodule -l | grep testport
```

Now, remove the policy:

```
$ sudo rm /etc/selinux.d/testport.cil
```

Notice that the policy will no longer be there:

```
$ sudo semodule -l | grep testport
```

Why?
====

This enables an easy way to install policies by establishing intent, as opposed to
having to tell a system how to do things. This way, all we need to do is tell a system
that we want a file in a specific path in the file system, and the rest will be taken care of.

OpenShift/Machine Config Operator
---------------------------------

The [Machine Config Operator](https://github.com/openshift/machine-config-operator)
is an [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that
ensures that the nodes belonging to an OpenShift cluster are in a certain state.

If this daemon would be running on a node in the cluster, all we would need to do
to install a policy is:

```
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  labels:
    machineconfiguration.openshift.io/role: worker
  name: 50-example-sepolicy
spec:
  config:
    ignition:
      version: 2.2.0
    storage:
      files:
      - contents:
          source: data:,%3B%20Declare%20a%20test_port_t%20type%0A%28type%20test_port_t%29%0A%3B%20Assign%20the%20type%20to%20the%20object_r%20role%0A%28roletype%20object_r%20test_port_t%29%0A%0A%3B%20Assign%20the%20right%20set%20of%20attributes%20to%20the%20port%0A%28typeattributeset%20defined_port_type%20test_port_t%29%0A%28typeattributeset%20port_type%20test_port_t%29%0A%0A%3B%20Declare%20tcp%3A1440%20as%20test_port_t%0A%28portcon%20tcp%201440%20%28system_u%20object_r%20test_port_t%20%28%28s0%29%20%28s0%29%29%29%29
        filesystem: root
        mode: 0600
        path: /etc/selinux.d/testport.cil
```

This `MachineConfig` object tells the operator to put the policy in the specified path, with
the specified permissions. Note that the policy is URL encoded due
to what the [ignition format](https://github.com/coreos/ignition) requires.

Without this daemon, each policy installation would require us to persist the file
on the node, then run a one-off systemd unit to install the policy. As policies
get added to the system, the number of systemd units increases, which is neither scalable
nor user-friendly.

Uses
====

This daemon is currently being used [in the security-profiles-operator](
https://github.com/kubernetes-sigs/security-profiles-operator) in order to do
the heavy lifting of installing SELinux policies. The operator itself manages the policies
as Kubernetes objects, and the daemon makes sure that they are actually installed in
the nodes of the cluster.

Looking for a home
==================

While this daemon is currently being developed in **JAORMX/selinuxd**, it would be better
for this project to live elsewhere. If you have ideas on where would be an appropriate
place for this. We are open to suggestions!
