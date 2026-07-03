# Consumed by GoReleaser: it copies the already cross-compiled binary out of the
# build context rather than compiling, so the image build is fast and uses the
# same static binary every other artifact ships.
#
# This image is tiny on purpose. unagi is a pure-Go static binary; the compiler
# and its Python runtime are compiled straight into it. Compiling the Go it
# emits needs a Go toolchain on the host, so the image is for the compile step
# itself (parse, check, emit), not for building the final binaries.
#
# GoReleaser builds one multi-platform image with buildx and stages each
# platform's binary under a $TARGETPLATFORM directory (e.g. linux/amd64/) in the
# build context, so the COPY line selects the right one through the automatic
# TARGETPLATFORM build arg.
FROM alpine:3.21

ARG TARGETPLATFORM

RUN apk add --no-cache ca-certificates tzdata

COPY $TARGETPLATFORM/unagi /usr/bin/unagi

WORKDIR /work

# Mount your project and inspect what your Python becomes:
#
#   docker run -v "$PWD:/work" ghcr.io/tamnd/unagi build app.py --emit-go gen/
ENTRYPOINT ["/usr/bin/unagi"]
