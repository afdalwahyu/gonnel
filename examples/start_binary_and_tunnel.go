package main

import (
	"bufio"
	"fmt"
	"gonnel"
	"os"
)

func main() {
	client, err := gonnel.NewClient(gonnel.Options{
		BinaryPath: "../ngrok-bin/ngrok_linux",
	})
	if err != nil {
		fmt.Println(err)
	}
	defer client.Close()

	done := make(chan bool)
	go client.StartServer(done)
	<-done

	client.AddTunnel(&gonnel.Tunnel{
		Proto:        gonnel.HTTP,
		Name:         "awesome",
		LocalAddress: "127.0.0.1:4040",
		Auth:         "username:password",
	})

	client.ConnectAll()

	fmt.Print("Press any to disconnect")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadRune()

	client.DisconnectAll()
}
