# Gonnel

    > Golang wrapper for ngrok. Expose your localhost to the internet.
Tested on linux, hopefully supports Mac, Windows, and Linux


## Installation

 - Download ngrok binary [file](https://ngrok.com/download)
 - Install package
```
go get github.com/afdalwahyu/gonnel
```
## Update
```
go get -u github.com/afdalwahyu/gonnel
```

## [Examples:](https://github.com/afdalwahyu/gonnel/tree/master/examples)
### Create client &
```Go
package main

import (
	"fmt"
	"github.com/afdalwahyu/gonnel"
	"bufio"
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
```

## How it works
Inspired from [node.js wrapper](https://github.com/bubenshchykov/ngrok) that use ngrok binary, run it and use client api to create or close tunnel
