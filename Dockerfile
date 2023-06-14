FROM alpine

RUN apk --no-cache add build-base git curl jq bash
RUN curl -s -k https://api.github.com/repos/JamesWoolfenden/ghat/releases/latest | jq '.assets[] | select(.name | contains("linux_386")) | select(.content_type | contains("gzip")) | .browser_download_url' -r | awk '{print "curl -L -k " $0 " -o ./ghat.tar.gz"}' | sh
RUN tar -xf ./ghat.tar.gz -C /usr/bin/ && rm ./ghat.tar.gz && chmod +x /usr/bin/ghat && echo 'alias ghat="/usr/bin/ghat"' >> ~/.bashrc
COPY entrypoint.sh /entrypoint.sh

# Code file to execute when the docker container starts up (`entrypoint.sh`)
ENTRYPOINT ["/entrypoint.sh"]
