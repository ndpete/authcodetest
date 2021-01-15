package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/urfave/cli"
)

// AuthCodeService is service to perform authcode functions
type AuthCodeService struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
	Redirect     string `json:"Redirect"`
	Port         string `json:"Port"`
	RootURL      string `json:"RootURL"`
	Scope        string
	authCode     string
	Tokens       AuthCodeResponse
}

// AuthCodeResponse is the basic response from WSO2
type AuthCodeResponse struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	Expires      int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
}

func main() {
	app := cli.NewApp()
	app.Name = "AuthCode Test"
	app.Usage = "Test authcode flow and get tokens from authcode and refresh token"
	app.Version = "0.1.1"
	app.Commands = []cli.Command{
		{
			Name:    "test",
			Aliases: []string{"t"},
			Usage:   "run authcode flow test",
			Action:  cliTest,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "config, c",
					Value: getDefaultFile(),
					Usage: "Load configuration from `FILE`",
				},
				cli.StringFlag{
					Name:  "scope, s",
					Value: "openid",
					Usage: "Overide default scope openid",
				},
			},
		},
		{
			Name:    "generate",
			Aliases: []string{"gen", "g"},
			Usage:   "Generate config file",
			Action:  cliGenerate,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "output, o",
					Value: getDefaultFile(),
					Usage: "Output to `FILE`",
				},
			},
		},
		{
			Name:    "timeout",
			Aliases: []string{"o"},
			Usage:   "run authcode timeout test",
			Action:  cliTimeout,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "config, c",
					Value: getDefaultFile(),
					Usage: "Load configuration from `FILE`",
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func cliTest(c *cli.Context) error {
	configFile := c.String("config")
	authService := newAuthCodeServiceFromFile(configFile)
	authService.Scope = c.String("scope")
	authService.runAuthTest()
	return nil
}

func cliTimeout(c *cli.Context) error {
	configFile := c.String("config")
	authService := newAuthCodeServiceFromFile(configFile)
	authService.timeoutTest()
	return nil
}

func newAuthCodeServiceFromFile(configFile string) *AuthCodeService {
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	authService := &AuthCodeService{}

	err = json.Unmarshal(file, authService)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}
	return authService
}

func getDefaultFile() string {
	user, err := user.Current()
	if err != nil {
		log.Fatalf("Coulding get current user: %v", err)
	}
	return fmt.Sprintf("%s/.authcodetest", user.HomeDir)
}

func cliGenerate(c *cli.Context) error {
	var clientID, clientSecret, redirect, apiURL string
	fmt.Print("Client ID: ")
	fmt.Scanln(&clientID)
	fmt.Print("Client Secret: ")
	fmt.Scanln(&clientSecret)
	fmt.Print("Redirect URL: ")
	fmt.Scanln(&redirect)
	fmt.Print("Root API URL: ")
	fmt.Scanln(&apiURL)

	u, _ := url.Parse(redirect)

	config := &AuthCodeService{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Redirect:     redirect,
		Port:         u.Port(),
		RootURL:      apiURL,
	}
	file, _ := json.MarshalIndent(config, "", "  ")
	file = append(file, '\n')
	fmt.Printf("%s", file)
	outfile := c.String("output")
	err := ioutil.WriteFile(outfile, file, 0644)
	if err != nil {
		log.Fatalf("Couldn't write config file: %v", err)
	}
	return nil
}

func (a *AuthCodeService) runAuthTest() {
	a.getAuthCode()
	err := a.getToken("authcode")
	if err != nil {
		log.Fatal("Error getting token from auth code")
	}
	log.Printf("Returned Auth - Refresh: %s - AccessToken: %s", a.Tokens.RefreshToken, a.Tokens.AccessToken)
	log.Print("Calling Echo...")
	echo := a.callEcho()
	log.Printf("Echo Status: %t", echo)
	log.Print("Refreshing Token...")
	err = a.getToken("refresh")
	if err != nil {
		log.Fatal("Error refreshing token")
	}
	log.Printf("Returned Refresh: %s", a.Tokens.RefreshToken)
	log.Print("Refreshing Token...")
	err = a.getToken("refresh")
	if err != nil {
		log.Fatal("Error refreshing token")
	}
	log.Printf("Returned Refresh: %s", a.Tokens.RefreshToken)
}

func (a *AuthCodeService) getAuthCode() {
	codeChan := make(chan string)
	go startListener(a.Port, codeChan)
	url := fmt.Sprintf("%s/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=%s", a.RootURL, a.ClientID, a.Redirect, a.Scope)
	if runtime.GOOS == "darwin" {
		log.Printf("Launching Redirect URL in background: %s ", url)
		cmd := exec.Command("/usr/bin/open", "-g", url)
		cmd.Start()
	} else {
		fmt.Printf("Login URL: %s", url)
	}
	a.authCode = <-codeChan
	log.Print("Auth Code:", a.authCode)
}

func startListener(port string, c chan<- string) {
	port = fmt.Sprintf(":%s", port)
	stop := make(chan bool)
	srv := http.Server{Addr: port}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code, ok := r.URL.Query()["code"]
		if !ok {
			log.Print("Auth Code not found")
		}
		io.WriteString(w, "success")
		c <- code[len(code)-1]
		stop <- true
	})
	log.Print("Starting localhost listener")
	log.Print("If waiting check browser window for CAS")
	go srv.ListenAndServe()
	<-stop
	srv.Close()
}

func (a *AuthCodeService) getToken(method string) error {
	tokenURL := fmt.Sprintf("%s/token", a.RootURL)
	data := url.Values{}
	if method == "authcode" {
		data.Set("grant_type", "authorization_code")
		data.Set("code", a.authCode)
		data.Set("redirect_uri", a.Redirect)
		data.Set("scope", a.Scope)
	} else if method == "refresh" {
		data.Set("grant_type", "refresh_token")
		data.Set("refresh_token", a.Tokens.RefreshToken)
		data.Set("scope", a.Scope)
	} else {
		return errors.New("Invalid Method")
	}
	client := &http.Client{}
	req, _ := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	req.SetBasicAuth(a.ClientID, a.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode == 200 {
		json.Unmarshal(body, &a.Tokens)
		// fmt.Println(string(body))
		// fmt.Println(a)
		return nil
	}
	log.Printf("Token Response Code: %d", resp.StatusCode)
	log.Printf("ERROR token body: %s", body)
	return errors.New("Invalid response")
}

func (a *AuthCodeService) callEcho() bool {
	echoURL := fmt.Sprintf("%s/echo/v2/ping", a.RootURL)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", echoURL, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", a.Tokens.AccessToken))
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode == 200 {
		return true
	}
	log.Printf("ERROR: Echo status Code: %d", resp.StatusCode)
	log.Printf("ERROR: Echo body %s", body)
	return false
}

func (a *AuthCodeService) timeoutTest() {
	a.getAuthCode()
	err := a.getToken("authcode")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Refreshing token for fresh expiration")
	a.getToken("refresh")
	start := time.Now()
	expires := time.Duration(a.Tokens.Expires) * time.Second
	calcExpires := start.Add(expires)
	log.Printf("Token: %s - Created at: %s", a.Tokens.AccessToken, start.Format("2006-01-02T15:04:05"))
	log.Printf("Calculated Expire time: %s", calcExpires.Format("2006-01-02T15:04:05"))
	log.Printf("Expires in: %v", a.Tokens.Expires)
	duration := 0
	// Add 5 minutes to the expire time to ensure a failed case
	for duration < (a.Tokens.Expires + 300) {
		echo := a.callEcho()
		if echo != true {
			break
		}
		log.Printf("Echo Response: %t - Elapsed: %v - Sleeping 1 second....", echo, duration)
		time.Sleep(time.Second)
		duration++
	}
	end := time.Since(start)
	log.Printf("Total Elapsed Time: %v", end.Seconds())
	log.Printf("Refreshing Token")
	a.getToken("refresh")
	log.Printf("Echo Response: %t", a.callEcho())
}
