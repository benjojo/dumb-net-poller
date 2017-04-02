package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
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

var (
	sshtarget = flag.String("target", "192.168.2.1:22",
		"The address of the SSH target to get data from")
	sshusername = flag.String("username", "ben", "The SSH username")
	sshpassword = flag.String("password", os.Getenv("SSH_PASSWORD"),
		"The SSH password, can also use ${SSH_PASSWORD}")
	configfile     = flag.String("cfg", "", "a json file that will override all settings")
	collectdserver = flag.String("collectdserver", "localhost",
		"put here the collectd server to send stats to")
	collectdport = flag.String("collectdport", "25826",
		"put here the collectd server port to send stats to")
	hostnameoveride = flag.String("hostnameoveride", "",
		"override what hostname is sent to collectd, Default is target")
	stdout = flag.Bool("stdout", false,
		"echo strings to be used in the exec mode of collectd")
)

func main() {
	flag.Parse()

	parseConfig()

	collectdhostname := strings.Split(*sshtarget, ":")[0]
	if *hostnameoveride != "" {
		collectdhostname = *hostnameoveride
	}

	conn, err := network.Dial(net.JoinHostPort(*collectdserver, *collectdport),
		network.ClientOptions{BufferSize: 100})
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

		// Once a Session is created, you can execute a single command on
		// the remote side using the Run method.
		var b bytes.Buffer
		session.Stdout = &b
		if err := session.Run("cat /proc/net/dev"); err != nil {
			log.Fatal("Failed to run: " + err.Error())
		}

		ifinfo := parseNetDev(b.String())
		if *stdout {
			echoToCollectdServer(ifinfo, collectdhostname)
		} else {
			sendToCollectdServer(ifinfo, collectdhostname, conn)
		}
		session.Close()
		time.Sleep(time.Second * 10)
	}

}

func echoToCollectdServer(input map[string]interfaceInfo, hostname string) {
	for interfacename, info := range input {
		interfacename = strings.Replace(interfacename, "-", "", 0)
		fmt.Printf("PUTVAL %s/interface-%s/if_octets interval=10 N:%d:%d\n", hostname, interfacename, api.Derive(info.RX["bytes"]), api.Derive(info.TX["bytes"]))
	}
}

func sendToCollectdServer(input map[string]interfaceInfo, hostname string, conn *network.Client) {
	ctx := context.Background() // Dunno

	for interfacename, info := range input {
		interfacename = strings.Replace(interfacename, "-", "", 0)
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
		if err := conn.Write(ctx, &vl); err != nil {
			log.Printf("collectd sending error %s", err.Error())
			continue
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

		fin.RX = RX
		fin.TX = TX
		ifname := strings.Split(parts[0], ":")[0]
		interfaces[ifname] = fin
	}
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

type jsonConfig struct {
	Collectdport    string `json:"collectdport"`
	Collectdserver  string `json:"collectdserver"`
	Hostnameoveride string `json:"hostnameoveride"`
	Sshpassword     string `json:"sshpassword"`
	Sshtarget       string `json:"sshtarget"`
	Sshusername     string `json:"sshusername"`
	Stdout          bool   `json:"stdout"`
}

func parseConfig() {
	if *configfile == "" {
		return
	}

	f, err := os.Open(*configfile)
	if err != nil {
		log.Fatalf("Unable to open config file, %s", err.Error())
	}

	jreader := json.NewDecoder(f)
	cfg := jsonConfig{}
	err = jreader.Decode(&cfg)
	if err != nil {
		log.Fatalf("Unable to parse config file, %s", err.Error())
	}

	if cfg.Sshtarget == "" || cfg.Sshusername == "" ||
		cfg.Sshpassword == "" {
		log.Fatalf("sshpassword, sshtarget, and sshusername must be filled out in the config file")
	}

	if cfg.Stdout != true &&
		(cfg.Collectdserver == "" || cfg.Collectdport == "") {
		log.Fatalf("You need to set a collectd server to submit to, or enable stdout mode.")
	}

	sshtarget = &cfg.Sshtarget
	sshusername = &cfg.Sshusername
	sshpassword = &cfg.Sshpassword
	collectdserver = &cfg.Collectdserver
	collectdport = &cfg.Collectdport
	hostnameoveride = &cfg.Hostnameoveride
	stdout = &cfg.Stdout
}
