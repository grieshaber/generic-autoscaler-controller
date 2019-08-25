FROM golang:alpine as builder

RUN apk add --no-cache git
RUN go get github.com/golang/dep/cmd/dep

COPY Gopkg.lock Gopkg.toml /go/src/generic-autsocaler-controller/
WORKDIR /go/src/generic-autsocaler-controller/
# Install library dependencies
RUN dep ensure -vendor-only

COPY . /go/src/generic-autsocaler-controller/
RUN go build -o /bin/generic-autsocaler-controller


FROM scratch
COPY --from=builder /bin/generic-autsocaler-controller /bin/generic-autsocaler-controller
ENTRYPOINT ["/bin/generic-autsocaler-controller"]