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

set -x

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

# Install selinuxd policy
# FIXME(jaosorior): Replace this call with oneshot command
semodule -i security/selinuxd.cil

SECCOMP_FLAG=""
if [ "$OS" == "fedora" ]; then
    SECCOMP_FLAG="--security-opt seccomp=$PWD/security/selinuxd-seccomp-fedora-35.json"
fi

# run daemon
podman run \
    --name "$CONTAINER_NAME" \
    -d \
    $SECCOMP_FLAG \
    --security-opt label=type:selinuxd.process \
    -v /sys/fs/selinux:/sys/fs/selinux \
    -v /var/lib/selinux:/var/lib/selinux \
    -v /etc/selinux:/etc/selinux \
    -v /etc/selinux.d:/etc/selinux.d \
    $IMG daemon
