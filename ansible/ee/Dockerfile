FROM python:3.12.8-slim-bookworm

ARG ANSIBLE_CORE_VERSION=2.18.6
ARG ANSIBLE_RUNNER_VERSION=2.4.1
ENV ANSIBLE_COLLECTIONS_PATH=/usr/share/ansible/collections \
    HOME=/home/runner

RUN apt-get update \
    && apt-get install --no-install-recommends -y openssh-client ca-certificates \
    && python -m pip install --no-cache-dir "ansible-core==${ANSIBLE_CORE_VERSION}" "ansible-runner==${ANSIBLE_RUNNER_VERSION}" \
    && useradd --create-home --uid 1000 runner \
    && mkdir -p /usr/share/ansible/collections \
    && chown -R runner:runner /home/runner \
    && rm -rf /var/lib/apt/lists/*

COPY ansible/collections/dist/homelab-web-1.0.0.tar.gz /tmp/homelab-web-1.0.0.tar.gz
COPY ansible/collections/dist/homelab-system-1.0.0.tar.gz /tmp/homelab-system-1.0.0.tar.gz
RUN ansible-galaxy collection install \
      /tmp/homelab-web-1.0.0.tar.gz \
      /tmp/homelab-system-1.0.0.tar.gz \
      --collections-path /usr/share/ansible/collections \
    && rm /tmp/homelab-web-1.0.0.tar.gz /tmp/homelab-system-1.0.0.tar.gz \
    && chown -R runner:runner /home/runner

USER 1000:1000
WORKDIR /runner
ENTRYPOINT ["ansible-runner"]
