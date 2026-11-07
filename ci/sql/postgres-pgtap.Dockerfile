# PostgreSQL 16 with pgTAP and pg-semver, the test database of the pipelines (written by aqaa: the image its runs use).
FROM postgres:16.14-alpine3.23
RUN apk add --no-cache --virtual .build git make perl patch gcc musl-dev \
    && git clone -q --branch v1.3.4 --depth 1 https://github.com/theory/pgtap /tmp/pgtap \
    && test "$(git -C /tmp/pgtap rev-parse HEAD)" = 968eb53a33114e83042b3bdb0c664b5b80cf8bdf \
    && make -C /tmp/pgtap >/dev/null && make -C /tmp/pgtap install >/dev/null \
    && git clone -q --branch v0.41.0 --depth 1 https://github.com/theory/pg-semver /tmp/semver \
    && test "$(git -C /tmp/semver rev-parse HEAD)" = f8c003fae3365fbe96fe6f4944578680f55e6d63 \
    && make -C /tmp/semver with_llvm=no >/dev/null && make -C /tmp/semver with_llvm=no install >/dev/null \
    && rm -rf /tmp/pgtap /tmp/semver && apk del .build \
    && ls /usr/local/share/postgresql/extension/pgtap.control /usr/local/share/postgresql/extension/semver.control
