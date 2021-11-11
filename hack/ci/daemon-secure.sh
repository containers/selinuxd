#!/usr/bin/env bash
# Copyright 2021 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

set -x

source hack/ci/env.sh

mkdir -p /etc/selinux.d

# Initialize base policies
podman run \
    --name policy-copy \
    --privileged \
    -v /etc/selinux:/etc/selinux \
    -v /etc/selinux.d:/etc/selinux.d \
    --entrypoint /bin/bash \
    $IMG -c 'cp /usr/share/selinuxd/templates/* /etc/selinux.d/'

# Install base policies
podman run \
    --name "$CONTAINER_NAME-oneshot" \
    --privileged \
    -v /sys/fs/selinux:/sys/fs/selinux \
    -v /var/lib/selinux:/var/lib/selinux \
    -v /etc/selinux:/etc/selinux \
    -v /etc/selinux.d:/etc/selinux.d \
    $IMG oneshot

cp security/selinuxd.cil /etc/selinux.d/

# Install selinuxd policy
# FIXME(jaosorior): Replace this call with oneshot command
semodule -i security/selinuxd.cil

# run daemon
podman run \
    --name "$CONTAINER_NAME" \
    -d \
    --security-opt seccomp=$PWD/security/selinuxd-seccomp-fedora-35.json \
    --security-opt label=type:selinuxd.process \
    -v /sys/fs/selinux:/sys/fs/selinux \
    -v /var/lib/selinux:/var/lib/selinux \
    -v /etc/selinux:/etc/selinux \
    -v /etc/selinux.d:/etc/selinux.d \
    $IMG daemon
