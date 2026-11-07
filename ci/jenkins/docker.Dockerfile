# Build agent for the Dockerfile stages of the Jenkinsfile (the tool versions of aqaa's runner, checksums verified).
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl unzip xz-utils git && rm -rf /var/lib/apt/lists/*
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN T="$(mktemp -d)" \
    && cd "$T" \
    && curl -fsSL -o hadolint "https://github.com/hadolint/hadolint/releases/download/v2.15.1/hadolint-linux-x86_64" && echo "c7187db94eeeeca956519a6af171adc31453941a1e777961f6e680f697c8c507  hadolint" | sha256sum -c - \
    && install -m 0755 hadolint /usr/local/bin/hadolint \
    && cd - > /dev/null \
    && rm -rf "$T"
