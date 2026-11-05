package main

import (
	"fmt"
	"os"
	"strconv"

	"p2p-chat/cli"
	"p2p-chat/node"
)

func parseArgs(args []string) (string, string) {
	port := "5000"
	nick := "anon"

	switch len(args) {
	case 2:
		if _, err := strconv.Atoi(args[1]); err == nil {
			port = args[1]
		} else {
			nick = args[1]
		}
	case 3:
		if _, err := strconv.Atoi(args[1]); err == nil {
			port = args[1]
			nick = args[2]
		} else if _, err := strconv.Atoi(args[2]); err == nil {
			nick = args[1]
			port = args[2]
		} else {
			port = args[1]
			nick = args[2]
		}
	}

	return port, nick
}

func main() {
	port, nick := parseArgs(os.Args)

	identityPath := os.Getenv("P2P_CHAT_IDENTITY_FILE")
	if identityPath == "" {
		identityPath = "identity.json"
	}
	n, err := node.LoadOrCreateNode(identityPath, nick)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no se pudo cargar la identidad: %v\n", err)
		os.Exit(1)
	}
	n.StartListening(port)

	//fmt.Printf("Nodo [%s] (%s) escuchando en puerto %s\n", nick, nodeID, port)

	cli := cli.NewCLI(n)

	go cli.ListenEvents()
	cli.Start()
}
