FROM golang:alpine as builder

RUN apk add --no-cache git
RUN go get github.com/golang/dep/cmd/dep

COPY Gopkg.lock Gopkg.toml /go/src/github.com/grieshaber/generic-autoscaler-controller/
WORKDIR /go/src/github.com/grieshaber/generic-autoscaler-controller/
# Install library dependencies
RUN dep ensure -vendor-only

COPY . /go/src/github.com/grieshaber/generic-autoscaler-controller/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /bin/generic-autoscaler-controller .


FROM scratch
COPY --from=builder /bin/generic-autoscaler-controller /bin/generic-autoscaler-controller
ENTRYPOINT ["/bin/generic-autoscaler-controller"]