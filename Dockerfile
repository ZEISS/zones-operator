# https://goreleaser.com/docker/

FROM gcr.io/distroless/static:nonroot

ARG TARGETPLATFORM

WORKDIR /
COPY $TARGETPLATFORM/operator /main

USER 65532:65532

CMD ["/main"]
