FROM registry.fedoraproject.org/fedora-minimal:44

LABEL org.opencontainers.image.source https://github.com/jamesread/SickRock

EXPOSE 8080

ENV GIN_MODE=release

COPY frontend/dist/ ./www/

COPY SickRock /SickRock

VOLUME [ "/config" ]

ENTRYPOINT [ "/SickRock" ]
