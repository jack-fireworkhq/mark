FROM golang:latest
ENV GOPATH="/go"
WORKDIR /go/src/github.com/jack-fireworkhq/mark
COPY / .
RUN make get
RUN make build

FROM alpine:latest
RUN apk --no-cache add ca-certificates bash git
COPY --from=0 /go/src/github.com/jack-fireworkhq/mark /bin/
RUN mkdir -p /docs
WORKDIR /docs
