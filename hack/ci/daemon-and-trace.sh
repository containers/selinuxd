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

source /etc/profile.d/selinuxd-env.sh

mkdir -p /etc/selinux.d

# Initialize base policies
podman run \
    --name policy-copy \
    --privileged \
    -v /etc/selinux:/etc/selinux \
    -v /etc/selinux.d:/etc/selinux.d \
    --entrypoint /bin/bash \
    $IMG -c 'cp /usr/share/selinuxd/templates/* /etc/selinux.d/'

# run daemon
podman run \
    --name "$CONTAINER_NAME" \
    -d \
    --annotation io.containers.trace-syscall="of:/tmp/selinuxd-seccomp.json" \
    --privileged \
    -v /sys/fs/selinux:/sys/fs/selinux \
    -v /var/lib/selinux:/var/lib/selinux \
    -v /etc/selinux:/etc/selinux \
    -v /etc/selinux.d:/etc/selinux.d \
    $IMG daemon

# Should create selinuxd.cil
podman inspect "$CONTAINER_NAME" | udica selinuxd || /bin/true
