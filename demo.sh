#!/bin/bash

. $(dirname ${BASH_SOURCE})/util.sh

FEDKUBECTL="kubectl --context=federation --namespace=myns"
KUBECTL1a="kubectl --context=gce-us-central1-a --namespace=myns"
KUBECTL1f="kubectl --context=gce-us-central1-f --namespace=myns"
desc "List all clusters"
run "kubectl --context=federation get clusters"

desc "List foo"
run "${FEDKUBECTL} get foos"
run "${KUBECTL1a} get foos"
run "${KUBECTL1f} get foos"

desc "Create an example foo in federation"
run "${FEDKUBECTL} create -f examples/foo.yaml"
run "cat examples/foo.yaml"

desc "List foo"
run "${FEDKUBECTL} get foos"
run "${KUBECTL1a} get foos"
run "${KUBECTL1f} get foos"

desc "Delete kubecon-foo from cluster 1a"
run "${KUBECTL1a} delete foos kubecon-foo"
run "${KUBECTL1a} get foos"
run "${KUBECTL1a} get foos"

desc "Delete kubecon-foo from federation"
run "${FEDKUBECTL} delete foos kubecon-foo"
run "${KUBECTL1a} get foos"
run "${KUBECTL1f} get foos"

