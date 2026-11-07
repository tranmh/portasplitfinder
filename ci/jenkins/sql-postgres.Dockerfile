# Build agent for the database code stages of the Jenkinsfile: psql, sqlfluff, squawk, pymssql and tSQLt (the
# versions of aqaa's runner).
FROM python:3.13-slim
RUN apt-get update && apt-get install -y --no-install-recommends postgresql-client curl unzip ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && pip install --no-cache-dir sqlfluff==4.4.0 squawk-cli==2.67.0 pymssql==2.4.3 \
    && curl -fsSL -o /tmp/tsqlt.zip https://tsqlt.org/wp-content/uploads/dlm_uploads/2015/07/tSQLt_V1.0.8083.3529.zip \
    && echo "af841f357dd9189f8b197c34c5014fe0dbabd3be34b024ed46967dafe13d4f50  /tmp/tsqlt.zip" | sha256sum -c - \
    && mkdir -p /opt/tsqlt && unzip -q -j /tmp/tsqlt.zip PrepareServer.sql tSQLt.class.sql -d /opt/tsqlt && rm /tmp/tsqlt.zip
ENV AQAA_TSQLT_DIR=/opt/tsqlt
