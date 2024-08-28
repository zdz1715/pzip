ARG JDK_VERSION=11
FROM eclipse-temurin:${JDK_VERSION}

# golang
ENV GOROOT=/usr/local/go \
    GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct

# maven
ENV MAVEN_VERSION=3.9.9 \
    MAVEN_HOME=/usr/share/maven

ENV PATH=$MAVEN_HOME/bin:$GOROOT/bin:$PATH

ARG GO_VERSION

# install go and maven
RUN set -eux; \
    apt-get -qqy update; \
    apt-get install -qqy --no-install-recommends --no-install-suggests \
      git  \
      curl \
      zip \
      unzip \
    ; \
    arch="$(dpkg --print-architecture)"; arch="${arch##*-}"; \
    url=; \
    case "$arch" in \
        'amd64') \
            url=https://go.dev/dl/go${GO_VERSION}.linux-arm64.tar.gz; \
            ;; \
        'arm64') \
            url=https://go.dev/dl/go${GO_VERSION}.linux-arm64.tar.gz; \
            ;; \
        *) echo >&2 "error: unsupported architecture '$arch' (likely packaging update needed)"; exit 1 ;; \
    esac; \
    curl -fSsL -o /tmp/go.tar.gz $url; \
    tar -xzvf /tmp/go.tar.gz -C /usr/local/; \
    wget https://archive.apache.org/dist/maven/maven-3/$MAVEN_VERSION/binaries/apache-maven-$MAVEN_VERSION-bin.zip \
        -O /tmp/apache-maven-$MAVEN_VERSION-bin.zip; \
    unzip /tmp/apache-maven-$MAVEN_VERSION-bin.zip -d /usr/share; \
    mv /usr/share/apache-maven-$MAVEN_VERSION $MAVEN_HOME; \
    rm /tmp/apache-maven-$MAVEN_VERSION-bin.zip
