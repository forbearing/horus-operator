#FROM hybfkuf/findpvdir:latest
FROM debian:stable-slim


#ADD ./restic_0.13.1_linux_amd64 /usr/local/bin/restic
ADD ./restic_0.14.0_linux_amd64 /usr/local/bin/restic
ADD ./restic-init.sh /usr/local/bin/

ENTRYPOINT ["sh", "-c", "sleep infinity"]
#ENTRYPOINT ["cat"]
