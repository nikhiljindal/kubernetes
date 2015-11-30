---
layout: docwithnav
title: "Running Kubernetes locally via Docker"
---
<!-- BEGIN MUNGE: UNVERSIONED_WARNING -->


<!-- END MUNGE: UNVERSIONED_WARNING -->
Running Kubernetes locally via Docker
-------------------------------------

**Table of Contents**

- [Overview](#setting-up-a-cluster)
- [Prerequisites](#prerequisites)
- [Step One: Run etcd](#step-one-run-etcd)
- [Step Two: Run the master](#step-two-run-the-master)
- [Step Three: Run the service proxy](#step-three-run-the-service-proxy)
- [Test it out](#test-it-out)
- [Run an application](#run-an-application)
- [Expose it as a service](#expose-it-as-a-service)
- [A note on turning down your cluster](#a-note-on-turning-down-your-cluster)

### Overview

The following instructions show you how to set up a simple, single node Kubernetes cluster using Docker.

Here's a diagram of what the final result will look like:
![Kubernetes Single Node on Docker](k8s-singlenode-docker.png)

### Prerequisites

1. You need to have docker installed on one machine.
2. Your kernel should support memory and swap accounting. Ensure that the
following configs are turned on in your linux kernel:

{% highlight console %}
{% raw %}
    CONFIG_RESOURCE_COUNTERS=y
    CONFIG_MEMCG=y
    CONFIG_MEMCG_SWAP=y
    CONFIG_MEMCG_SWAP_ENABLED=y
    CONFIG_MEMCG_KMEM=y
{% endraw %}
{% endhighlight %}

3. Enable the memory and swap accounting in the kernel, at boot, as command line
parameters as follows:

{% highlight console %}
{% raw %}
    GRUB_CMDLINE_LINUX="cgroup_enable=memory swapaccount=1"
{% endraw %}
{% endhighlight %}

    NOTE: The above is specifically for GRUB2.
    You can check the command line parameters passed to your kernel by looking at the
    output of /proc/cmdline:

{% highlight console %}
{% raw %}
    $cat /proc/cmdline
    BOOT_IMAGE=/boot/vmlinuz-3.18.4-aufs root=/dev/sda5 ro cgroup_enable=memory
    swapaccount=1
{% endraw %}
{% endhighlight %}

### Step One: Run etcd

{% highlight sh %}
{% raw %}
docker run --net=host -d gcr.io/google_containers/etcd:2.0.12 /usr/local/bin/etcd --addr=127.0.0.1:4001 --bind-addr=0.0.0.0:4001 --data-dir=/var/etcd/data
{% endraw %}
{% endhighlight %}

### Step Two: Run the master

{% highlight sh %}
{% raw %}
docker run \
    --volume=/:/rootfs:ro \
    --volume=/sys:/sys:ro \
    --volume=/dev:/dev \
    --volume=/var/lib/docker/:/var/lib/docker:ro \
    --volume=/var/lib/kubelet/:/var/lib/kubelet:rw \
    --volume=/var/run:/var/run:rw \
    --net=host \
    --pid=host \
    --privileged=true \
    -d \
    gcr.io/google_containers/hyperkube:v1.0.1 \
    /hyperkube kubelet --containerized --hostname-override="127.0.0.1" --address="0.0.0.0" --api-servers=http://localhost:8080 --config=/etc/kubernetes/manifests
{% endraw %}
{% endhighlight %}

This actually runs the kubelet, which in turn runs a [pod](../user-guide/pods.html) that contains the other master components.

### Step Three: Run the service proxy

{% highlight sh %}
{% raw %}
docker run -d --net=host --privileged gcr.io/google_containers/hyperkube:v1.0.1 /hyperkube proxy --master=http://127.0.0.1:8080 --v=2
{% endraw %}
{% endhighlight %}

### Test it out

At this point you should have a running Kubernetes cluster.  You can test this by downloading the kubectl
binary
([OS X](https://storage.googleapis.com/kubernetes-release/release/v1.0.1/bin/darwin/amd64/kubectl))
([linux](https://storage.googleapis.com/kubernetes-release/release/v1.0.1/bin/linux/amd64/kubectl))

*Note:*
On OS/X you will need to set up port forwarding via ssh:

{% highlight sh %}
{% raw %}
boot2docker ssh -L8080:localhost:8080
{% endraw %}
{% endhighlight %}

List the nodes in your cluster by running:

{% highlight sh %}
{% raw %}
kubectl get nodes
{% endraw %}
{% endhighlight %}

This should print:

{% highlight console %}
{% raw %}
NAME        LABELS    STATUS
127.0.0.1   <none>    Ready
{% endraw %}
{% endhighlight %}

If you are running different Kubernetes clusters, you may need to specify `-s http://localhost:8080` to select the local cluster.

### Run an application

{% highlight sh %}
{% raw %}
kubectl -s http://localhost:8080 run nginx --image=nginx --port=80
{% endraw %}
{% endhighlight %}

Now run `docker ps` you should see nginx running.  You may need to wait a few minutes for the image to get pulled.

### Expose it as a service

{% highlight sh %}
{% raw %}
kubectl expose rc nginx --port=80
{% endraw %}
{% endhighlight %}

Run the following command to obtain the IP of this service we just created. There are two IPs, the first one is internal (CLUSTER_IP), and the second one is the external load-balanced IP.

{% highlight sh %}
{% raw %}
kubectl get svc nginx
{% endraw %}
{% endhighlight %}

Alternatively, you can obtain only the first IP (CLUSTER_IP) by running:

{% highlight sh %}
{% raw %}
kubectl get svc nginx --template={{.spec.clusterIP}}
{% endraw %}
{% endhighlight %}

Hit the webserver with the first IP (CLUSTER_IP):

{% highlight sh %}
{% raw %}
curl <insert-cluster-ip-here>
{% endraw %}
{% endhighlight %}

Note that you will need run this curl command on your boot2docker VM if you are running on OS X.

### A note on turning down your cluster

Many of these containers run under the management of the `kubelet` binary, which attempts to keep containers running, even if they fail.  So, in order to turn down
the cluster, you need to first kill the kubelet container, and then any other containers.

You may use `docker kill $(docker ps -aq)`, note this removes _all_ containers running under Docker, so use with caution.



<!-- BEGIN MUNGE: IS_VERSIONED -->
<!-- TAG IS_VERSIONED -->
<!-- END MUNGE: IS_VERSIONED -->


<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/docs/getting-started-guides/docker.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->

