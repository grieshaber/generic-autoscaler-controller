FROM scratch

ADD bin/generic-autoscaler-controller /generic-autoscaler-controller

ENTRYPOINT ["/generic-autoscaler-controller"]