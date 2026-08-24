FROM zenika/alpine-chrome:latest
ARG TARGETPLATFORM
ENV TERM xterm-256color
COPY $TARGETPLATFORM/chrome-protocol-proxy chrome-protocol-proxy
COPY docker/run.sh run.sh
EXPOSE 9222 9223
ENTRYPOINT ["sh", "run.sh"]
