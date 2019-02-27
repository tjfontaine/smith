FROM oraclelinux:7-slim

RUN yum install -y oracle-golang-release-el7 && yum install -y git golang make

WORKDIR /src

ADD . .

RUN make install

FROM oraclelinux:7-slim

RUN yum install -y oracle-epel-release-el7 && yum install -y pigz mock && yum clean all

ADD etc /etc

copy --from=0 /usr/bin/smith /usr/bin/smith

VOLUME /write

WORKDIR /write

ENTRYPOINT ["/usr/bin/smith"]
