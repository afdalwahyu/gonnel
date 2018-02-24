package gonnel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// Tunnel will be used in creating or closing a tunnel.
// a tunnel also can connect to ngrok directly
// as long ngrok client server already running
type Tunnel struct {
	Proto         Protocol // Protocol that use in tunneling process
	Name          string   // A name that used for creating or closing
	LocalAddress  string   // Can be host with port or port only
	Auth          string   // Username & password that will authenticate to access tunnel
	Inspect       bool     // Inspect transaction data tunnel that will be logged in binary file
	RemoteAddress string   // Result ngrok connection address
	IsCreated     bool     // Information tunnel created or not
}

// Maximum retries until tunnel connected/closed
const maxRetries = 100

// CreateTunnel that create connection to ngrok server
//
// Error will be from api ngrok server client, retries is used because server client
// not always success when started. Need at least 1 or 2 second to start.
func (c *Client) CreateTunnel(t *Tunnel) (err error) {
	for attempt := uint(0); attempt <= maxRetries; attempt++ {
		err = func() error {
			log.Printf("Creating tunnel %d attempt \n", attempt)
			time.Sleep(1 * time.Second)
			var record responseCreateTunnel
			jsonData := map[string]interface{}{
				"addr":    t.LocalAddress,
				"proto":   t.Proto.String(),
				"name":    t.Name,
				"inspect": t.Inspect,
				"auth":    t.Auth,
			}

			if t.Proto.String() == "http" {
				jsonData["bind_tls"] = true
			}

			url := fmt.Sprintf("http://%s/api/tunnels", c.WebUIAddress)
			jsonValue, err := json.Marshal(jsonData)
			if err != nil {
				return err
			}
			res, err := http.Post(url, "application/json", bytes.NewBuffer(jsonValue))
			if err != nil {
				return err
			}
			defer res.Body.Close()

			if res.StatusCode < 200 || res.StatusCode > 299 {
				res, _ := ioutil.ReadAll(res.Body)
				return errors.New("error api: " + string(res))
			}

			if err := json.NewDecoder(res.Body).Decode(&record); err != nil {
				return err
			}

			t.RemoteAddress = record.PublicURL
			t.IsCreated = true
			log.Println("tunnel " + t.Name + " is created using: " + t.RemoteAddress + " address")
			return nil
		}()
		if c.LogApi && err != nil {
			log.Println(err)
		}
		if err == nil {
			break
		}
	}
	return
}

// CloseTunnel that close tunnel from ngrok server
//
// Close tunnel call API using DELETE method
func (c *Client) CloseTunnel(t *Tunnel) (err error) {
	for attempt := uint(0); attempt <= maxRetries; attempt++ {
		err = func() error {
			log.Println("Closing tunnel in " + t.RemoteAddress)
			url := fmt.Sprintf("http://%s/api/tunnels/%s", c.WebUIAddress, t.Name)
			req, err := http.NewRequest("DELETE", url, nil)
			if err != nil {
				log.Println(err)
				return err
			}
			client := &http.Client{}
			res, err := client.Do(req)
			if err != nil {
				log.Println(err)
				return err
			}
			defer res.Body.Close()

			if res.StatusCode < 200 || res.StatusCode > 299 {
				res, _ := ioutil.ReadAll(res.Body)
				return errors.New("error api: " + string(res))
			}

			t.RemoteAddress = ""
			t.IsCreated = false
			log.Println("Tunnel " + t.Name + " successfully closed")
			return nil
		}()
		if c.LogApi && err != nil {
			log.Println(err)
		}
		if err == nil {
			break
		}
	}
	return
}
