package main

import (
	"bytes"
	"context"
	"flag"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"collectd.org/api"
	"collectd.org/network"
	"golang.org/x/crypto/ssh"
)

func main() {
	sshtarget := flag.String("target", "192.168.2.1:22", "The address of the SSH target to get data from")
	sshusername := flag.String("username", "ben", "The SSH username")
	sshpassword := flag.String("password", os.Getenv("SSH_PASSWORD"), "The SSH password, can also use ${SSH_PASSWORD}")
	collectdserver := flag.String("collectdserver", "localhost", "put here the collectd server to send stats to")
	collectdport := flag.String("collectdport", "25826", "put here the collectd server port to send stats to")
	hostnameoveride := flag.String("hostnameoveride", "", "override what hostname is sent to collectd, Default is target")
	flag.Parse()

	collectdhostname := strings.Split(*sshtarget, ":")[0]
	if *hostnameoveride != "" {
		collectdhostname = *hostnameoveride
	}

	conn, err := network.Dial(net.JoinHostPort(*collectdserver, *collectdport), network.ClientOptions{BufferSize: 100})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("%s", *sshtarget)

	config := &ssh.ClientConfig{
		User: *sshusername,
		Auth: []ssh.AuthMethod{
			ssh.Password(*sshpassword),
		},
	}
	client, err := ssh.Dial("tcp", *sshtarget, config)
	if err != nil {
		log.Fatal("Failed to dial: ", err)
	}

	for {
		// Each ClientConn can support multiple interactive sessions,
		// represented by a Session.
		session, err := client.NewSession()
		if err != nil {
			log.Fatal("Failed to create session: ", err)
		}
		defer session.Close()

		// Once a Session is created, you can execute a single command on
		// the remote side using the Run method.
		var b bytes.Buffer
		session.Stdout = &b
		if err := session.Run("cat /proc/net/dev"); err != nil {
			log.Fatal("Failed to run: " + err.Error())
		}
		// fmt.Println(b.String())
		ifinfo := parseNetDev(b.String())
		sendToCollectdServer(ifinfo, collectdhostname, conn)
		time.Sleep(time.Second * 10)
	}

}

func sendToCollectdServer(input map[string]interfaceInfo, hostname string, conn *network.Client) {
	ctx := context.Background() // Dunno

	for interfacename, info := range input {
		vl := api.ValueList{
			Identifier: api.Identifier{
				Host:   hostname,
				Plugin: "interface-" + interfacename,
				Type:   "if_octets",
			},
			Time:     time.Now(),
			Interval: 10 * time.Second,
			Values:   []api.Value{api.Derive(info.RX["bytes"]), api.Derive(info.TX["bytes"])},
		}
		// log.Printf("Debug: %s -> %d|%d", interfacename, api.Derive(info.RX["bytes"]), api.Derive(info.TX["bytes"]))
		if err := conn.Write(ctx, &vl); err != nil {
			return
		}
	}

}

type interfaceInfo struct {
	RX map[string]int
	TX map[string]int
}

func parseNetDev(input string) map[string]interfaceInfo {
	lines := strings.Split(input, "\n")

	// First of all, We need to figure out the keys at the top.
	key := compressWhitespace(lines[1])
	keys := strings.Split(key, " ")
	RXKeys := make([]string, 0)
	TXKeys := make([]string, 0)
	SeenPipe := false

	RXKeys = append(RXKeys, "bytes")
	for keycount := 2; keycount < len(keys); keycount++ {
		if strings.Contains(keys[keycount], "|") {
			split := strings.Split(keys[keycount], "|")
			RXKeys = append(RXKeys, split[0])
			TXKeys = append(TXKeys, split[1])
			SeenPipe = true
		} else {
			if SeenPipe {
				TXKeys = append(TXKeys, keys[keycount])
			} else {
				RXKeys = append(RXKeys, keys[keycount])
			}
		}
	}
	// fmt.Printf("%v\n", RXKeys)
	// fmt.Printf("%v\n", TXKeys)
	// totalkeys := len(RXKeys) + len(TXKeys)

	interfaces := make(map[string]interfaceInfo)

	for ifline := 2; ifline < len(lines)-1; ifline++ {
		fin := interfaceInfo{}
		RX := make(map[string]int)
		TX := make(map[string]int)

		parts := strings.Split(compressWhitespace(lines[ifline]), " ")

		partcount := 1
		for _, v := range RXKeys {
			count, err := strconv.ParseInt(parts[partcount], 10, 64)
			if err != nil {
				log.Fatalf("Odd, non number value seen while parsing /proc/net/dev %s on line %d", parts[partcount], ifline)
			}

			RX[v] = int(count)
			partcount++
		}

		for _, v := range TXKeys {
			count, err := strconv.ParseInt(parts[partcount], 10, 64)
			if err != nil {
				log.Fatalf("Odd, non number value seen while parsing /proc/net/dev %s on line %d", parts[partcount], ifline)
			}

			TX[v] = int(count)
			partcount++
		}

		// log.Printf("I passed parse!")

		fin.RX = RX
		fin.TX = TX
		ifname := strings.Split(parts[0], ":")[0]
		interfaces[ifname] = fin

		// for k, v := range fin.TX {
		// 	log.Printf("Presend Debug TX: %s - %s - %d", ifname, k, v)
		// }
	}

	// fmt.Printf("%v", interfaces)
	return interfaces
}

var (
	releadclosewhtsp = regexp.MustCompile(`^[\s\p{Zs}]+|[\s\p{Zs}]+$`)
	reinsidewhtsp    = regexp.MustCompile(`[\s\p{Zs}]{2,}`)
)

func compressWhitespace(input string) string {
	final := releadclosewhtsp.ReplaceAllString(input, "")
	final = reinsidewhtsp.ReplaceAllString(final, " ")
	return final
}
