FROM golang:1.14 AS builder
ARG BIN_OUTPUT_DIRECTORY="/app/samba-config-kube-pvc"
WORKDIR /app
ENV GO111MODULE=on

COPY go.sum go.mod ./
RUN go mod download
COPY . .

# Build the Go src
RUN CGO_ENABLED=0 GOOS=linux go build -o $BIN_OUTPUT_DIRECTORY/samba-config-kube-pvc .

FROM scratch
ARG BIN_OUTPUT_DIRECTORY="/app/samba-config-kube-pvc"

WORKDIR /app/
COPY resources/template-samba-config/ resources/template-samba-config/
COPY --from=builder $BIN_OUTPUT_DIRECTORY/samba-config-kube-pvc .

VOLUME /etc/samba/
VOLUME /root/.kube/

ENTRYPOINT ["/app/samba-config-kube-pvc"]

