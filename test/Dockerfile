FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git && \
    rm -rf /var/lib/apt/lists/*

# Copy pre-built binary
ARG TARGETARCH=amd64
COPY bin/koda-linux-${TARGETARCH} /usr/local/bin/koda
RUN chmod +x /usr/local/bin/koda

# Simulate a fresh user
RUN useradd -m -s /bin/bash testuser
USER testuser
WORKDIR /home/testuser

COPY --chown=testuser test/smoke.sh /home/testuser/smoke.sh
RUN chmod +x /home/testuser/smoke.sh

CMD ["/home/testuser/smoke.sh"]
