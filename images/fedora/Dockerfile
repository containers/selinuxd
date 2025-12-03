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

FROM registry.fedoraproject.org/fedora-minimal:38 AS build
ARG GO_VERSION=go1.24.9
ENV GOPATH="/go"
ENV PATH="$GOPATH/bin:$PATH"
USER root
WORKDIR /work

RUN mkdir -p bin
RUN mkdir -p /go

RUN microdnf install -y golang make libsemanage-devel container-selinux

# NOTE(jaosorior): This allows us to use a specific golang version in CentOS as
# opposed to the older one that comes with the distro.
RUN go install golang.org/dl/${GO_VERSION}@latest
RUN ${GO_VERSION} download

COPY . /work

RUN GO=${GO_VERSION} make

FROM registry.fedoraproject.org/fedora-minimal:38

# TODO(jaosorior): See if we can run this without root
USER root

LABEL name="selinuxd" \
      description="selinuxd is a daemon that listens for files in /etc/selinux.d/ and installs the relevant policies."

# TODO(jaosorior): Remove once we use static linking
RUN microdnf install -y policycoreutils

RUN mkdir -p /usr/share/selinuxd/templates
COPY --from=build /usr/share/udica/templates/* /usr/share/selinuxd/templates/
COPY --from=build /work/bin/selinuxdctl /usr/bin/

ENTRYPOINT ["/usr/bin/selinuxdctl"]
