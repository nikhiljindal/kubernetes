#!/bin/bash

# Copyright 2014 Google Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# A library of helper functions that each provider hosting Kubernetes must implement to use cluster/kube-*.sh scripts.

readonly KUBE_ROOT=$(dirname "${BASH_SOURCE}")/../..
readonly ROOT=$(dirname "${BASH_SOURCE}")
source $ROOT/${KUBE_CONFIG_FILE:-"config-default.sh"}

export LIBVIRT_DEFAULT_URI=qemu:///system

readonly POOL=kubernetes
readonly POOL_PATH="$(cd $ROOT && pwd)/libvirt_storage_pool"

# join <delim> <list...>
# Concatenates the list elements with the delimiter passed as first parameter
#
# Ex: join , a b c
#  -> a,b,c
function join {
  local IFS="$1"
  shift
  echo "$*"
}

# Must ensure that the following ENV vars are set
function detect-master {
  KUBE_MASTER_IP=192.168.10.1
  KUBE_MASTER=kubernetes-master
  export KUBERNETES_MASTER=http://$KUBE_MASTER_IP:8080
  echo "KUBE_MASTER_IP: $KUBE_MASTER_IP"
  echo "KUBE_MASTER: $KUBE_MASTER"
}

# Get minion IP addresses and store in KUBE_MINION_IP_ADDRESSES[]
function detect-minions {
  for (( i = 0 ; i < $NUM_MINIONS ; i++ )); do
    KUBE_MINION_IP_ADDRESSES[$i]=192.168.10.$(($i+2))
  done
  echo "KUBE_MINION_IP_ADDRESSES=[${KUBE_MINION_IP_ADDRESSES[@]}]"
}

# Verify prereqs on host machine
function verify-prereqs {
  if ! which virsh >/dev/null; then
      echo "Can't find virsh in PATH, please fix and retry." >&2
      exit 1
  fi
  if ! virsh nodeinfo >/dev/null; then
      exit 1
  fi
  if [[ "$(</sys/kernel/mm/ksm/run)" -ne "1" ]]; then
      echo "KSM is not enabled" >&2
      echo "Enabling it would reduce the memory footprint of large clusters" >&2
      if [[ -t 0 ]]; then
          read -t 5 -n 1 -p "Do you want to enable KSM (requires root password) (y/n)? " answer
          echo ""
          if [[ "$answer" == 'y' ]]; then
              su -c 'echo 1 > /sys/kernel/mm/ksm/run'
          fi
      else
        echo "You can enable it with (as root):" >&2
        echo "" >&2
        echo "  echo 1 > /sys/kernel/mm/ksm/run" >&2
        echo "" >&2
      fi
  fi
}

# Destroy the libvirt storage pool and all the images inside
#
# If 'keep_base_image' is passed as first parameter,
# the base image is kept, as well as the storage pool.
# All the other images are deleted.
function destroy-pool {
  virsh pool-info $POOL >/dev/null 2>&1 || return

  rm -rf "$POOL_PATH"/kubernetes/*
  rm -rf "$POOL_PATH"/kubernetes_config*/*
  local vol
  virsh vol-list $POOL | awk 'NR>2 && !/^$/ && $1 ~ /^kubernetes/ {print $1}' | \
      while read vol; do
        virsh vol-delete $vol --pool $POOL
      done

  [[ "$1" == 'keep_base_image' ]] && return

  set +e
  virsh vol-delete coreos_base.img --pool $POOL
  virsh pool-destroy $POOL
  rmdir "$POOL_PATH"
  set -e
}

# Creates the libvirt storage pool and populate it with
# - the CoreOS base image
# - the kubernetes binaries
function initialize-pool {
  mkdir -p "$POOL_PATH"
  if ! virsh pool-info $POOL >/dev/null 2>&1; then
      virsh pool-create-as $POOL dir --target "$POOL_PATH"
  fi

  wget -N -P "$ROOT" http://${COREOS_CHANNEL:-alpha}.release.core-os.net/amd64-usr/current/coreos_production_qemu_image.img.bz2
  if [ "$ROOT/coreos_production_qemu_image.img.bz2" -nt "$POOL_PATH/coreos_base.img" ]; then
      bunzip2 -f -k "$ROOT/coreos_production_qemu_image.img.bz2"
      virsh vol-delete coreos_base.img --pool $POOL 2> /dev/null || true
      mv "$ROOT/coreos_production_qemu_image.img" "$POOL_PATH/coreos_base.img"
  fi
  # if ! virsh vol-list $POOL | grep -q coreos_base.img; then
  #     virsh vol-create-as $POOL coreos_base.img 10G --format qcow2
  #     virsh vol-upload coreos_base.img "$ROOT/coreos_production_qemu_image.img" --pool $POOL
  # fi

  mkdir -p "$POOL_PATH/kubernetes"
  kube-push
  virsh pool-refresh $POOL
}

function destroy-network {
  set +e
  virsh net-destroy kubernetes_global
  virsh net-destroy kubernetes_pods
  set -e
}

function initialize-network {
  virsh net-create "$ROOT/network_kubernetes_global.xml"
  virsh net-create "$ROOT/network_kubernetes_pods.xml"
}

function render-template {
  eval "echo \"$(cat $1)\""
}

# Instantiate a kubernetes cluster
function kube-up {
  detect-master
  detect-minions
  initialize-pool keep_base_image
  initialize-network

  readonly ssh_keys="$(cat ~/.ssh/id_*.pub | sed 's/^/  - /')"
  readonly kubernetes_dir="$POOL_PATH/kubernetes"
  readonly discovery=$(curl -s https://discovery.etcd.io/new)

  readonly machines=$(join , "${KUBE_MINION_IP_ADDRESSES[@]}")

  local i
  for (( i = 0 ; i <= $NUM_MINIONS ; i++ )); do
    if [[ $i -eq 0 ]]; then
        type=master
    else
      type=minion-$(printf "%02d" $i)
    fi
    name=kubernetes_$type
    image=$name.img
    config=kubernetes_config_$type

    virsh vol-create-as $POOL $image 10G --format qcow2 --backing-vol coreos_base.img --backing-vol-format qcow2

    mkdir -p "$POOL_PATH/$config/openstack/latest"
    render-template "$ROOT/user_data.yml" > "$POOL_PATH/$config/openstack/latest/user_data"
    virsh pool-refresh $POOL

    domain_xml=$(mktemp)
    render-template $ROOT/coreos.xml > $domain_xml
    virsh create $domain_xml
    rm $domain_xml
  done
}

# Delete a kubernetes cluster
function kube-down {
  virsh list | awk 'NR>2 && !/^$/ && $2 ~ /^kubernetes/ {print $2}' | \
      while read dom; do
        virsh destroy $dom
      done
  destroy-pool keep_base_image
  destroy-network
}

function find-release-tars {
  SERVER_BINARY_TAR="${KUBE_ROOT}/server/kubernetes-server-linux-amd64.tar.gz"
  if [[ ! -f "$SERVER_BINARY_TAR" ]]; then
    SERVER_BINARY_TAR="${KUBE_ROOT}/_output/release-tars/kubernetes-server-linux-amd64.tar.gz"
  fi
  if [[ ! -f "$SERVER_BINARY_TAR" ]]; then
    echo "!!! Cannot find kubernetes-server-linux-amd64.tar.gz"
    exit 1
  fi
}

# The kubernetes binaries are pushed to a host directory which is exposed to the VM
function upload-server-tars {
  tar -x -C "$POOL_PATH/kubernetes" -f "$SERVER_BINARY_TAR" kubernetes
  rm -rf "$POOL_PATH/kubernetes/bin"
  mv "$POOL_PATH/kubernetes/kubernetes/server/bin" "$POOL_PATH/kubernetes/bin"
  rmdir "$POOL_PATH/kubernetes/kubernetes/server" "$POOL_PATH/kubernetes/kubernetes"
}

# Update a kubernetes cluster with latest source
function kube-push {
  find-release-tars
  upload-server-tars
}

# Execute prior to running tests to build a release if required for env
function test-build-release {
  echo "TODO"
}

# Execute prior to running tests to initialize required structure
function test-setup {
  echo "TODO"
}

# Execute after running tests to perform any required clean-up
function test-teardown {
  echo "TODO"
}

# Set the {KUBE_USER} and {KUBE_PASSWORD} environment values required to interact with provider
function get-password {
  export KUBE_USER=core
  echo "TODO get-password"
}

function setup-monitoring-firewall {
  echo "TODO" 1>&2
}

function teardown-monitoring-firewall {
  echo "TODO" 1>&2
}

function setup-logging-firewall {
  echo "TODO: setup logging"
}

function teardown-logging-firewall {
  echo "TODO: teardown logging"
}
