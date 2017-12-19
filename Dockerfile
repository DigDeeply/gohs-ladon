FROM digdeeply/intel-hyperscan-centos7:latest
RUN mkdir -vp /go/src && yum install -y golang 
ENV GOPATH=/go/ \
   PKG_CONFIG_PATH=/usr/local/include/hs/ \
   CGO_CFLAGS="-I/usr/local/include/hyperscan/src" \
   LIBRARY_PATH=/usr/local/include/hs/lib \
   GOROOT=/usr/lib/golang/
WORKDIR /go/src
