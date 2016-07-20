Setting Up a Test Environment Using VMs
=======================================

This document describes scripts which can be used to setup a network configuration and start virtual machines for testing simple Cerana clusters on a Linux host. They support any number of virtual machines limited only by the capabilities of the host.

Each script maintains a configuration in `~/.testcerana`. This directory contains all of the options with with the scripts were last run making it unnecessary to remember the options when repeating the same test scenarios.

To reset an option to its default value simply use the value "default". e.g. To reset the disk image directory option (`start-vm`) use `--diskimagedir default`.

The following is an overview of the purpose and capabilities of each script. Each script also provides a comprehensive list of command line options which can be viewed using the `--help` option. Basically, the workflow is to use `vm-network` to configure the network for a test scenario and then use `start-vm` one or more times to start each of the VMs for the test.

cerana-functions.sh
-------------------

This script contains a number of helper functions used by the other scripts.

vm-network
----------

**NOTE:** Because this script reconfigures the network this script uses `sudo` to gain root access.

Testing interaction between the various nodes of a Cerana cluster requires a network configuration which allows communication between the nodes but avoids flooding the local network with test messages. This is accomplished using a bridge to connect a number of [TAP](https://en.wikipedia.org/wiki/TUN/TAP) devices. When using this script it helps to keep the following in mind. * A Cerana cluster can be comprised of 1 or more VMs. Each VM can have 1 or more TAP interfaces. The TAP interfaces to be associated with a specific VM is termed a "tapset". The number of *tapsets* is controlled using the option `--numsets`.

By default TAP interfaces are named using using the pattern `tap.<tapset>.<n>` where `<n>` is TAP number within a *tapset*. For example a configuration having three VMs with two interfaces each produces three *tapsets* each having two interfaces. The resulting TAP interfaces become:

```
        tap.1.1
        tap.1.2
        tap.2.1
        tap.2.2
        tap.3.1
        tap.3.2
```

Each TAP interface is assigned a MAC address beginning with the default pattern "DE:AD:BE:EF". The 5th byte of the MAC address is the number of the corresponding *tapset* and the 6th byte is the TAP number within the *tapset*.

These are then all linked to a single bridge having the default name `ceranabr0`.

**NOTE:** Currently only a single bridge is created. In the future multiple bridges will be used to better support testing VMs having multiple TAP interfaces. For example one bridge can be used for the node management interfaces while a second bridge can be used for a connection to a wider network.

The `vm-network` script also supports maintaining multiple network configurations making it easy to tear down one configuration and then setup another. See the `--config` option.

To help booting the first node a DHCP server is started which is configured to listen **only** on the test bridge. Once the first node is running this server can then be shut down to allow the first node to take over the DHCP function (`--shutdowndhcp`).

**NOTE:** [NAT](https://en.wikipedia.org/wiki/Network_address_translation) is not currently supported. NAT is needed if the nodes need to communicate outside the virtual test network. This may be supported in a future version.

start-vm
--------

The `start-vm` script uses [KVM](http://wiki.qemu.org/KVM) to run virtual machines. A big reason for KVM is it supports nested virtual machines provided your [kernel supports it](https://fedoraproject.org/wiki/How_to_enable_nested_virtualization_in_KVM). Installing QEMU-KVM is outside the scope of this document. Look for instructions relevant to the distro on which you will be running VMs.

**NOTE:** Each VM requires approximately 3GB of RAM.

After using `vm-network` to configure the network for a test scenario the VMs can be started using the `start-vm` script. One VM per *tapset* can be started. Each VM is assigned its own [UUID](https://en.wikipedia.org/wiki/Universally_unique_identifier) with the last byte being the same as the *tapset* number used for the VM.

Even though a given *tapset* may contain a large number of TAP interfaces a VM need only use a subset of those interfaces. This is controlled using the `--numvmif` option. Each of the interfaces used by the VM is given a unique MAC address again derived using the *tapset* number as part of the MAC address and using a scheme similar to the TAP interfaces but with the 5th byte having the pattern `8<n>` where `<n>` is the *tapset* number. This avoids conflicts with the MAC addresses assigned to the TAP interfaces while at the same time providing information making it easy to identify corresponding interfaces. **NOTE:** This scheme effectively limits the practical maximum number of VMs to 9 (1 thru 9).

Each VM can be started using images from a local build or downloaded from a build server which defaults to [S3](http://omniti-cerana-artifacts.s3.amazonaws.com/index.html?prefix=CeranaOS/jobs/build-cerana/).

Booting either kernel and initrd images or using an ISO is possible.

This script creates one or more disk images (`--numdisks`) for each VM which helps verification of [ZFS](https://en.wikipedia.org/wiki/ZFS) within Cerana. Each disk image by default is given a name which also uses the *tapset* to help identify it. This naming scheme by default uses the pattern `sas-<tapset>.<n>` where `<n>` is the disk number. For example a configuration having three VMs (*tapsets*) with two disk images each produces images having the following names:

```
        sas-1-1.img
        sas-1-2.img
        sas-2-1.img
        sas-2-2.img
        sas-3-1.img
        sas-3.2.img
```

Examples
========

Single Node
-----------

This example shows what happens when using only the default values. First `cd` to the directory where you want the various images (i.e. kernel, initrd, disk) to be saved.

```
vm-network --verbose
```

**NOTE:** If you've already been running `vm-network` you may want to use the `--resetdefaults` option to return to a known default state.

The interfaces `tap.1.1` and `ceranabr0` were created and `tap.1.1` added to the `ceranabr0` bridge. The `ceranabr0` bridge was assigned the IP address `10.0.2.2`. The `dhcpd` daemon was started and configured to listen only on the `10.0.2.0` subnet.

A configuration named `single` was created and saved in the `~/.testcerana` directory.

In this case the `artifacts` directory must exist and contain the kernel and initrd images.

```
start-vm --verbose
```

**NOTE:** If you've already been running `start-vm` you may want to use the `--resetdefaults` option to return to a known default state.

The interface `tap.1.1` is used as the management interface and by virtue of the `dhcpd` daemon is assigned the IP address `10.0.2.200`.

A disk image named `sas-1-1.img` was created and uses as a virtual disk for the VM.

The boot messages will have spewed to the console and you can log in as the root user (no password).

The root prompt contains the UUID assigned to the VM with the last byte equal to "01" to indicate the single VM in this scenario.

**NOTE:** Use [`^AX`](http://qemu.weilnetz.de/qemu-doc.html#mux_005fkeys) keystrokes to shutdown the VM.

Two Nodes
---------

This example requires reconfiguring the network to support two *tapsets* because of using two nodes in the test scenario. It also shows booting an ISO instead of the normal kernel images.

```
vm-network --verbose --numsets 2 --config two-node
```

Because this example is also booting the Cerana ISO it is necessary to shutdown the `dhcpd` daemon so that Cerana can take over that function.

```
vm-network --verbose --shutdowndchpd
```

This saves another configuration named `two-node` in the `~/.testcerana` directory. The existing network configuration was torn down and the new one created. The interfaces `tap.1.1` and `tap.2.1` have been created and linked to the `ceranabr0` bridge.

```
start-vm --verbose --boot iso
```

This boots the ISO image which in turn displays the GRUB menu. Cursor down and select the "CeranaOS Cluster Bootstrap" option. This starts the first node of the two node cluster.

Now open another console and `cd` to the same directory. This time use `start-vm` to start a second VM.

```
start-vm --verbose --boot iso --tapset 2
```

This too boots the ISO images which again displays the GRUB menu. This time cursor down and select the "CeranaOS Cluster Join" option.

This causes the second node to use the PXE protocol to boot using images provided by the `bootserver` running on the first node. Its management interface was assigned an IP address by the `dhcp-provider` also running on the first node.

Adding Interfaces to an Existing Configuration
----------------------------------------------

There are times when an additional interface is needed either to support an additional node or to add an interface for a node. This is a relatively simple thing to do using `vm-network`. Using the "Single Node" example above adding an interface is as simple as:

```
vm-network --verbose --numsets 2 --config single
```

This creates another TAP interface, `tap.2.1`, for the `single` configuration and adds it to the `ceranabr0` bridge. It was not necessary to shutdown the test network in this case. This also illustrates that `vm-network` is able to repair a network configuration (within limits) if an interface was deleted for any reason.

**NOTE:** At this time removing interfaces is possible but the script will not automatically delete TAP interfaces if the new number is smaller than before (e.g. the new `--numsets` is 1 but was 2). This a feature for the future. However, this does not cause a problem because the script will tear down all interfaces linked to a bridge when switching configurations or when the `--shutdown` option is used.

Using Downloaded Images
-----------------------

The `start-vm` script also supports downloading and using specific builds from a server. By default it downloads from the [CeranaOS instance on Amazon S3](http://omniti-cerana-artifacts.s3.amazonaws.com/index.html?prefix=CeranaOS/jobs/). This requires use of the `--job` and the `--build` options. The `--job` option defaults to `build-cerana` and is good for most cases. The `--build` option however has no default and must be set to a valid number ([look on S3](http://omniti-cerana-artifacts.s3.amazonaws.com/index.html?prefix=CeranaOS/jobs/build-cerana/)) before the download will work. For example the following downloads and boots build 121. All other options are whatever were set in a previous run.

**NOTE:** Symlinks are used to point to the build to boot. If files exist this script will not removed them. If you want to use the same directory you will need to manually remove the images.

```
start-vm --verbose --download --build 121
```

More
----

From time to time more examples will be added to this document to illustrate progressively complex scenarios.