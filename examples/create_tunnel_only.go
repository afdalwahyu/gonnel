package main

import (
	"fmt"
	"gonnel"
	"log"
)

// assumption server ngrok client already started
func main() {
	// Example using pre running binary
	client := gonnel.Client{
		WebUIAddress: "127.0.0.1:4040",
		LogApi:       true,
	}

	// Create pointer tunnel
	t := &gonnel.Tunnel{
		Name:         "awesome",
		Auth:         "u:p",
		Inspect:      false,
		LocalAddress: "127.0.0.1:4040",
		Proto:        gonnel.HTTP,
	}

	if err := client.CreateTunnel(t); err != nil {
		log.Fatalln(err)
	}

	fmt.Println(t.RemoteAddress)

	if err := client.CloseTunnel(t); err != nil {
		log.Fatalln(err)
	}
}
