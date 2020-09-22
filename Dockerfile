FROM ubuntu:18.04
ADD build/bin/p4dbeat-linux-amd64 /p4dbeat
ENTRYPOINT [ "/p4dbeat" ]