FROM gcr.io/distroless/static:nonroot

COPY scm-controller-linux-amd64 /manager
USER 65532:65532

ENTRYPOINT ["/manager"]
