// Package gonnel provides direct API tunnel using code. The implementation
// in this project inspired from ngrok wrapper node.js that using EventEmitter.
// In this package channel used to check if binary running successfully or not
//
// The Client package intended to handle all function that provided from binary
// like using auth token or create a tunnel
//
// Here is a simple example, initialize client binary and auth token automatically
// by running StartServer
//
//	client, err := gonnel.NewClient(gonnel.Options{
//		BinaryPath: "../ngrok-bin/ngrok_linux",
//	})
//	if err != nil {
//		fmt.Println(err)
//	}
//	defer client.Close()
//
//	done := make(chan bool)
//	go client.StartServer(done)
//	<-done
//
// This package also can directly create tunnel if you
// already started ngrok binary separately,
// WebUIAddress type need hostname and port
//
//	client := go_ngrok.Client{
//		WebUIAddress: "127.0.0.1:4040",
//		LogApi:       true,
//	}
//
//	// Create pointer tunnel
//	t := &go_ngrok.Tunnel{
//		Name:         "awesome",
//		Auth:         "username:password",
//		Inspect:      false,
//		LocalAddress: "4040",
//		Proto:        go_ngrok.HTTP,
//	}
//
//	if err := client.CreateTunnel(t); err != nil {
//		log.Fatalln(err)
//	}
//
package gonnel

import (
	"bytes"
	"errors"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sync"
	"syscall"
)

// Protocol type
type Protocol int

// Protocol that ngrok support
const (
	HTTP Protocol = iota
	TCP
	TLS
)

var protocols = [...]string{
	"http",
	"tcp",
	"tls",
}

func (p Protocol) String() string { return protocols[p] }

// Options that represents command that will be used to start binary
//
// Not all of this option necessary, if AuthToken provided then
// binary will run auth token first.
type Options struct {
	SubDomain     string // Sub domain config if you're using premium plan
	AuthToken     string // Auth token to authenticate client
	Region        string // Region that will tunneling from
	ConfigPath    string // Path config to store auth token or specific WebUI port
	BinaryPath    string // Binary file that will be running
	LogBinary     bool   // You can watch binary log or not
	IgnoreSignals bool   // Run child processes in a separate process group to ignore signals
}

// Client that provides all option and tunnel
//
// You don't need NewClient method if server client already started
type Client struct {
	Options      *Options  // Options that will be used for command
	Tunnel       []*Tunnel // List of all tunnel
	WebUIAddress string    // Client server for API communication
	LogApi       bool      // Log response from API or not
	commands     []string  // result of commands that will be used to run binary
	runningCmd   *exec.Cmd // Pointer of command that running
}

// Constant regex that will be used for handling stdout command
const (
	ngReady          = `starting web service.*addr=(\d+\.\d+\.\d+\.\d+:\d+)`   // check if ngrok ready
	ngInUse          = `address already in use`                                // check if port already in use
	ngSessionLimited = `is limited to (\d+) simultaneous ngrok client session` // Check limit ngrok
	webURI           = `\d+\.\d+\.\d+\.\d+:\d+`                                // Find client server
)

// NewClient that return Client pointer
//
// Client pointer can be used to close binary or start binary
func NewClient(opt Options) (*Client, error) {
	log.Println("New client")
	if opt.BinaryPath == "" {
		return nil, errors.New("binary path required")
	}

	if opt.Region == "" {
		opt.Region = "us"
	}

	if opt.AuthToken != "" {
		err := opt.AuthTokenCommand()
		if err != nil {
			return nil, err
		}
	}

	c := Client{Options: &opt}
	return &c, nil
}

// AuthTokenCommand that will be authenticate api token
func (o *Options) AuthTokenCommand() error {
	if o.AuthToken == "" {
		return errors.New("token missing")
	}

	if o.BinaryPath == "" {
		return errors.New("binary path file is missing")
	}

	commands := make([]string, 0)
	commands = append(commands, []string{"authtoken", o.AuthToken}...)

	if o.ConfigPath != "" {
		commands = append(commands, "--config"+o.ConfigPath)
	}

	cmd := exec.Command(o.BinaryPath, commands...)
	if o.IgnoreSignals {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
			Pgid:    0,
		}
	}
	var outBuffer, errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	if err := cmd.Start(); err != nil {
		return err
	}

	if errBuffer.String() != "" {
		return errors.New(errBuffer.String())
	}

	log.Println(outBuffer.String())
	return nil
}

// StartServer will be run command from previous options
//
// Channel needed to send information about WebUI started or not.
// stdout will be pipe and check using regex.
func (c *Client) StartServer(isReady chan bool) {
	log.Println("Start server")

	commands := c.Options.generateCommands()
	cmd := exec.Command(c.Options.BinaryPath, commands...)
	if c.Options.IgnoreSignals {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
			Pgid:    0,
		}
	}
	c.runningCmd = cmd
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	if !c.Options.IgnoreSignals {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(
			signalChan, syscall.SIGHUP,
			syscall.SIGINT, syscall.SIGTERM,
			syscall.SIGQUIT)
		go c.handleSignalInput(signalChan)
	}

	checkNGReady, err := regexp.Compile(ngReady)
	if err != nil {
		log.Fatalln(err)
	}

	checkNGInUse, err := regexp.Compile(ngInUse)
	if err != nil {
		log.Fatalln(err)
	}

	checkSessionLimit, err := regexp.Compile(ngSessionLimited)
	if err != nil {
		log.Fatalln(err)
	}

	checkWebURI, err := regexp.Compile(webURI)
	if err != nil {
		log.Fatalln(err)
	}

	chunk := make([]byte, 256)
	for {
		n, err := stdout.Read(chunk)
		if err != nil {
			log.Fatalln(err)
			os.Exit(1)
		}

		if n < 1 {
			continue
		}

		if c.Options.LogBinary {
			log.Print("Client-Bin-Log: ", string(chunk[:n]))
		}
		// handle regex (output) that search local ip and port for web ui
		if checkNGReady.Match(chunk[:n]) {
			host := checkWebURI.FindStringSubmatch(string(chunk[:n]))
			if len(host) >= 1 {
				log.Println("server client ready")
				c.WebUIAddress = host[0]
				isReady <- true
			}
		}
		if checkNGInUse.Match(chunk[:n]) {
			log.Fatalln("Address already in use")
		}
		if checkSessionLimit.Match(chunk[:n]) {
			log.Fatalln("Limit session reached for this account")
		}
	}
}

// generateCommands return array of commands
// that will be run on binary
func (o *Options) generateCommands() []string {
	commands := make([]string, 0)
	commands = append(commands, []string{"start", "--none", "--log=stdout"}...)
	commands = append(commands, "--region="+o.Region)

	if o.ConfigPath != "" {
		commands = append(commands, "--config="+o.ConfigPath)
	}

	if o.SubDomain != "" {
		commands = append(commands, "-subdomain="+o.SubDomain)
	}

	return commands
}

// handleSignalInput to handle signal form command,
// is program received signal or not
func (c *Client) handleSignalInput(signalChan chan os.Signal) {
	for {
		s := <-signalChan
		switch s {
		default:
			log.Println(s)
			c.Signal(s)
			os.Exit(1)
		}
	}
}

// AddTunnel create a new tunnel without connecting it
func (c *Client) AddTunnel(t *Tunnel) {
	log.Println("Add tunnel")
	c.Tunnel = append(c.Tunnel, t)
}

// ConnectAll connect all tunnels that created
func (c *Client) ConnectAll() error {
	wg := &sync.WaitGroup{}
	// api request post to /api/tunnels
	log.Println("Connecting")

	if len(c.Tunnel) < 1 {
		return errors.New("need at least 1 tunnel to connect")
	}

	for _, t := range c.Tunnel {
		if !t.IsCreated {
			wg.Add(1)
			go func(x *Tunnel) {
				c.CreateTunnel(x)
				wg.Done()
			}(t)
		}
	}

	wg.Wait()
	return nil
}

// DisconnectAll disconnect all tunnels that previously tunneled
func (c *Client) DisconnectAll() error {
	wg := &sync.WaitGroup{}
	//	api request delete to /api/tunnels/:Name
	log.Println("Disconnecting")
	if len(c.Tunnel) < 1 {
		return errors.New("need at least 1 tunnel to disconnect")
	}

	for _, t := range c.Tunnel {
		if t.IsCreated {
			wg.Add(1)
			go func() {
				c.CloseTunnel(t)
				wg.Done()
			}()
		}
	}

	wg.Wait()
	return nil
}

// Close running command and send kill signal to ngrok binary
func (c *Client) Close() error {
	return c.runningCmd.Process.Kill()
}

// Signal handle signal input and proceed to command
func (c *Client) Signal(signal os.Signal) error {
	return c.runningCmd.Process.Signal(signal)
}
