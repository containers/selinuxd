# Copyright © 2020 Red Hat, Inc.
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

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest as build

WORKDIR /work

RUN microdnf install -y \
    container-selinux \
    go-toolset \
    make \
    findutils \
    git-core && microdnf clean all

COPY . /work

RUN SEMODULE_BACKEND=policycoreutils make

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

# TODO(jaosorior): See if we can run this without root
USER root

LABEL name="selinuxd" \
      description="selinuxd is a daemon that listens for files in /etc/selinux.d/ and installs the relevant policies."

# TODO(jaosorior): Remove once we use static linking
RUN microdnf install -y \
    policycoreutils && microdnf clean all

RUN mkdir -p /usr/share/selinuxd/templates
COPY --from=build /usr/share/udica/templates/* /usr/share/selinuxd/templates/
COPY --from=build /work/bin/selinuxdctl /usr/bin/

ENTRYPOINT ["/usr/bin/selinuxdctl"]
