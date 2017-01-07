Monitoring SNMP less devices with ease
===

Blog post: https://blog.benjojo.co.uk/post/monitoring-wifi-devices-without-snmp

In the consumer world you will likely encounter networking devices that don't have a easy way to poll for their network stats,
or in some cases you hate <abbr style="border-bottom: 1px dotted green;" title="Simple Network Management Protocol
">SNMP</abbr> so much that you would rather go to rather worrying lengths to avoid it.

Either way, In my flat, I have two AP's, one is a [Ubiquiti Unifi AP-AC](https://www.ubnt.com/unifi/unifi-ap-ac-lite/) [(A)](https://archive.is/2ygni)
and another is a [cheap USB powered AP device](https://blog.benjojo.co.uk/asset/7A1zUBCSBE) that I use to patch up the last dead spot in my flat, Running [OpenWRT](https://openwrt.org/)

The issue is, both of these devices do not support SNMP, and that is _very_ annoying, since I want to add the thoughput infomation from them to my own metrics system.

However what I do have is the ability to log into both devices over SSH:

```
ben@metropolis:~$ ssh root@192.168.2.2


BusyBox v1.23.2 (2015-07-25 03:03:02 CEST) built-in shell (ash)

  _______                     ________        __
 |       |.-----.-----.-----.|  |  |  |.----.|  |_
 |   -   ||  _  |  -__|     ||  |  |  ||   _||   _|
 |_______||   __|_____|__|__||________||__|  |____|
          |__| W I R E L E S S   F R E E D O M
 -----------------------------------------------------
 CHAOS CALMER (15.05, r46767)
 -----------------------------------------------------
  * 1 1/2 oz Gin            Shake with a glassful
  * 1/4 oz Triple Sec       of broken ice and pour
  * 3/4 oz Lime Juice       unstrained into a goblet.
  * 1 1/2 oz Orange Juice
  * 1 tsp. Grenadine Syrup
 -----------------------------------------------------
root@OpenWrt:~# ifconfig eth0
eth0      Link encap:Ethernet  HWaddr AA:AA:AA:AA:AA:AA  
          inet6 addr: fe80::aaaa:aaff:feaa:aaaa/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:1291638 errors:0 dropped:0 overruns:0 frame:0
          TX packets:791097 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000 
          RX bytes:775606642 (739.6 MiB)  TX bytes:221519609 (211.2 MiB)
          Interrupt:5 
```

and on the UniFi (login is what you setup on the management portal):

```
ben@metropolis:~$ ssh 192.168.2.148


BusyBox v1.19.4 (2015-11-24 19:25:31 PST) built-in shell (ash)
Enter 'help' for a list of built-in commands.

BZ.v3.4.10# info

Model:       UAP-AC-Lite
Version:     3.4.10.3347
MAC Address: aa:aa:aa:aa:aa:aa
IP Address:  192.168.2.148
Hostname:    UBNT
Uptime:      424603 seconds

Status:      Unable to resolve (http://unifi:8080/inform)
BZ.v3.4.10# ifconfig eth0
eth0      Link encap:Ethernet  HWaddr AA:AA:AA:AA:AA:AA  
          inet6 addr: fe80::aaaa:aaff:feaa:aaaa/64 Scope:Link
          UP BROADCAST RUNNING PROMISC ALLMULTI MULTICAST  MTU:1500  Metric:1
          RX packets:1068057 errors:0 dropped:0 overruns:0 frame:0
          TX packets:774586 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000 
          RX bytes:524318849 (500.0 MiB)  TX bytes:272302817 (259.6 MiB)
          Interrupt:4 
```

So we can clearly get the numbers from the interface on the devices, but how can we turn these
into metrics itself? Can we avoid scraping ifconfig?

As always, the [/proc](https://blog.benjojo.co.uk/asset/F7w2dDYftP) filesystem comes to save us again!

```
ben@metropolis:~$ man proc | grep 'network device status' -B 1 -A 12
       /proc/net/dev
              The  dev  pseudo-file  contains network device status information.  This gives the number of received and sent
              packets, the number of errors and collisions and other basic statistics.  These are used  by  the  ifconfig(8)
              program to report device status.  The format is:

 Inter-|   Receive                                                |  Transmit
  face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
     lo: 2776770   11307    0    0    0     0          0         0  2776770   11307    0    0    0     0       0          0
   eth0: 1215645    2751    0    0    0     0          0         0  1782404    4324    0    0    0   427       0          0
   ppp0: 1622270    5552    1    0    0     0          0         0   354130    5669    0    0    0     0       0          0
   tap0:    7714      81    0    0    0     0          0         0     7714      81    0    0    0     0       0          0

```

and indeed it works on both of the AP's!

```
root@OpenWrt:~# cat /proc/net/dev
Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
eth0.1: 752363244 1291715    0    0    0     0          0    100592 169064832  646006    0    0    0     0       0          0
    lo: 41945520  616848    0    0    0     0          0         0 41945520  616848    0    0    0     0       0          0
 wlan0: 97870384  344592    0    0    0     0          0         0 745330677  940410    0    0    0     0       0          0
  eth0: 775616370 1291735    0    0    0     0          0         0 221527733  791149    0    0    0     0       0          0
br-lan: 51200876  611111    0    0    0     0          0         0 72040382  309854    0    0    0     0       0          0
eth0.2:       0       0    0    0    0     0          0         0 48944636  144942    0    0    0     0       0          0
```

```
BZ.v3.4.10# cat /proc/net/dev
Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
 wifi0:       0 68808530 3838220    0    0 3838220          0         0        0 9639362 195235    0    0     0       0          0
    lo: 3398312   38618    0    0    0     0          0         0  3398312   38618    0    0    0     0       0          0
   br0: 55225626  618615    0    0    0     0          0         0 95074612  456359    0    0    0     0       0          0
  ath2: 405992112  275307 2091 2091    0     0          0         0 468311173  428687 27905    0    0     0       0          0
  ath1:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  eth0: 524402812 1068845    0    0    0     0          0         0 272415027  775153    0    0    0     0       0          0
  ath0: 58625631   44108    0    0    0     0          0         0 43726042  410937    0   22    0     0       0          0
 wifi1:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
```

So, We can now just build a program to log into into the device and poll this information!

Using the fantastic [golang.org/x/crypto/ssh](https://godoc.org/golang.org/x/crypto/ssh) libary, it's super simple to set up a SSH connection and log into
the devices, Even better, since we have control over the low level connection, we can ensure
that we only need to login once while the application is running, saving the CPU on the APs from
a lot of work (since SSH crypto excahnges are very noticeably slow on these low end devices)

Overall, I present: "[dumb-net-poller](https://github.com/benjojo/dumb-net-poller)" a program you can run to get devices interface stats 
and then export them into collectd (and others systems, should anyone want to send a PR in).

Usage is pretty simple:

```
root@metrics:/etc/pollers# dumb-net-poller -h
Usage of dumb-net-poller:
  -cfg string
    	a json file that will override all settings
  -collectdport string
    	put here the collectd server port to send stats to (default "25826")
  -collectdserver string
    	put here the collectd server to send stats to (default "localhost")
  -hostnameoveride string
    	override what hostname is sent to collectd, Default is target
  -password string
    	The SSH password, can also use ${SSH_PASSWORD}
  -stdout
    	echo strings to be used in the exec mode of collectd
  -target string
    	The address of the SSH target to get data from (default "192.168.2.1:22")
  -username string
    	The SSH username (default "ben")
```

You will most likely want to not pass your SSH credentials as command line options,
so instead you can point it to a config file that looks the same, for example:

```
root@metrics:/etc/pollers# cat /etc/pollers/loadingap.json
{
  "sshtarget": "192.168.2.2:22",
  "sshusername": "root",
  "sshpassword": "xxxxxxxxxxxxx",
  "stdout": true,
  "hostnameoveride": "loadingap"
}
```

The stdout option is for use in the collectd exec module, You can configure it like so:

```
LoadPlugin exec

<Plugin exec>
        Exec nobody "/usr/bin/dumb-net-poller" "-cfg=/etc/pollers/unifiap.json"
</Plugin>
```

or, if you already have a network collectd setup, you can point it directly to the server.

I may be solving a problem no one else has, But either way, I got what I wanted in the end...

Grafana graphs :)

![grafanagraphs](https://blog.benjojo.co.uk/asset/2z8C0vMlQO)

As always, you can find the code here: https://github.com/benjojo/dumb-net-poller