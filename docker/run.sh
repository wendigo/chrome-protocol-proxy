#!/bin/sh
ENV TERM xterm-256color

chromium-browser --headless --disable-gpu --disable-software-rasterizer --disable-dev-shm-usage --no-sandbox --remote-debugging-address=0.0.0.0 --remote-debugging-port=9223 --no-sandbox &

./chrome-protocol-proxy -l 0.0.0.0:9222 -r localhost:9223 "$@"
