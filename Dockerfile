FROM scratch

EXPOSE 8080

COPY SickRock /SickRock

ENTRYPOINT [ "/SickRock" ]
