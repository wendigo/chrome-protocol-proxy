FROM zenika/alpine-chrome:latest
ENV TERM xterm-256color
COPY chrome-protocol-proxy chrome-protocol-proxy
COPY docker/run.sh run.sh
EXPOSE 9222 9223
ENTRYPOINT ["sh", "run.sh"]
