FROM scratch
MAINTAINER chamila@apache.org

ADD server /docker-ui
ADD tmpl /tmpl
ADD ca.crt /ca.crt

EXPOSE 8080

CMD ["/docker-ui"]