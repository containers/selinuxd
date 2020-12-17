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

FROM registry.centos.org/centos:8 AS build
USER root
WORKDIR /work

# Speed up build by leveraging docker layer caching
COPY go.mod go.sum vendor/ ./
RUN mkdir -p bin

RUN dnf install -y --disableplugin=subscription-manager \
    --enablerepo=powertools \
    golang make libsemanage-devel

ADD . /work

RUN make

FROM registry.centos.org/centos:8 AS build
# TODO(jaosorior): Switch to UBI once we use static linking
#FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

# TODO(jaosorior): See if we can run this without root
USER root

LABEL name="selinuxd" \
      description="selinuxd is a daemon that listens for files in /etc/selinux.d/ and installs the relevant policies."

# TODO(jaosorior): Remove once we use static linking
RUN dnf install -y --disableplugin=subscription-manager \
    --enablerepo=powertools \
    policycoreutils

COPY --from=build /work/bin/selinuxdctl /usr/bin/

ENTRYPOINT ["/usr/bin/selinuxdctl"]
