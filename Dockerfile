FROM golang:1.14 AS build
ADD ./ /build/
WORKDIR /build
RUN go get
RUN rm -rf p4dbeat && make ES_BEATS=$GOPATH/pkg/mod/github.com/elastic/beats/v7@v7.9.1 p4dbeat

FROM ubuntu:18.04
COPY --from=build /build/p4dbeat /p4dbeat
ENTRYPOINT [ "/p4dbeat" ]