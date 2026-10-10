FROM golang:1.25.0-alpine AS builder
WORKDIR /src
COPY operator/go.mod operator/go.sum ./
RUN go mod download
COPY operator/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /manager ./cmd
COPY ansible/collections/ansible_collections/homelab /usr/share/infrastructure-gitops/ansible_collections/homelab

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /manager /manager
COPY --from=builder /usr/share/infrastructure-gitops/ansible_collections /usr/share/infrastructure-gitops/ansible_collections
USER 65532:65532
ENTRYPOINT ["/manager"]
