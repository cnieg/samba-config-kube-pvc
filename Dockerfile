FROM golang:1.19 AS builder
WORKDIR /app
ENV GO111MODULE=on

COPY go.sum go.mod ./
RUN go mod download
COPY . .

# Build the Go src
RUN CGO_ENABLED=0 GOOS=linux go build -o ./samba-config-kube-pvc .

FROM scratch

WORKDIR /app/
COPY resources/template-samba-config/ resources/template-samba-config/
COPY --from=builder /app/samba-config-kube-pvc .

VOLUME /etc/samba/
VOLUME /root/.kube/

ENTRYPOINT ["/app/samba-config-kube-pvc"]

