FROM golang:1.14 AS builder
ARG BIN_OUTPUT_DIRECTORY="/app/samba-config-kube-pvc"
WORKDIR /app
ENV GO111MODULE=on

COPY go.sum go.mod ./
RUN go mod download
COPY . .

# Build the Go src
RUN mkdir -p $BIN_OUTPUT_DIRECTORY/bin
RUN CGO_ENABLED=0 GOOS=linux go build -o $BIN_OUTPUT_DIRECTORY/bin/samba-config-kube-pvc .
RUN ls -altr $BIN_OUTPUT_DIRECTORY

FROM scratch
ARG BIN_OUTPUT_DIRECTORY="/app/samba-config-kube-pvc"

WORKDIR /app/
COPY resources/template-samba-config/ resources/template-samba-config/
COPY --from=builder $BIN_OUTPUT_DIRECTORY/bin/samba-config-kube-pvc .

VOLUME /etc/samba/
VOLUME /root/.kube/

ENTRYPOINT ["/app/samba-config-kube-pvc"]

