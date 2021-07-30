FROM gcr.io/distroless/static:nonroot

COPY scm-controller-linux-amd64 /scm-controller
USER 65532:65532

ENTRYPOINT ["/scm-controller"]
