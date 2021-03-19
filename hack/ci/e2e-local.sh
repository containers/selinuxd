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

export E2E_SECURE=${E2E_SECURE:-""}

make fedora-image

# Ensure image.tar isn't there
rm -f image.tar

podman save -o image.tar quay.io/jaosorior/selinuxd-fedora:latest

RUN=./hack/ci/run.sh

echo "Spawning VM"
make vagrant-up


if [ -z "$E2E_SECURE" ]; then
    echo "Spawning selinuxd in VM with tracing"
    $RUN hack/ci/daemon-and-trace.sh
else
    echo "Spawning selinuxd in VM with security features enabled"
    $RUN hack/ci/daemon-secure.sh
fi

echo "Running e2e tests"
$RUN hack/ci/e2e.sh

echo "Getting logs"
$RUN hack/ci/logs.sh
