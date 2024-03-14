FROM scratch

LABEL org.opencontainers.image.source https://github.com/jamesread/SickRock

EXPOSE 8080

COPY SickRock /SickRock

ENTRYPOINT [ "/SickRock" ]
