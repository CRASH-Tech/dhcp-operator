FROM golang:alpine3.17 as builder

COPY cmd/ /app/cmd
COPY main.go /app/main.go
COPY pxe.go /app/pxe.go
COPY utils.go /app/utils.go
COPY leaderElection.go /app/leaderElection.go
COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
WORKDIR /app
RUN go build -o dhcp-operator

FROM alpine:3.17
COPY --from=builder /app/dhcp-operator /app/dhcp-operator
COPY static/ /app/static
WORKDIR /app
CMD /app/dhcp-operator
